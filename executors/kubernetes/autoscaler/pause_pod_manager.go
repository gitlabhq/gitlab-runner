package autoscaler

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	api "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/dns"
)

const (
	// defaultPausePodImage is the standard Kubernetes pause container image.
	defaultPausePodImage = "registry.k8s.io/pause:3.10"

	// defaultPriorityClassName is the default PriorityClass for pause pods.
	defaultPriorityClassName = "gitlab-runner-idle-capacity"
	// defaultPriorityClassValue is the priority value for the default PriorityClass.
	// Set to -1 so pause pods are preempted by any normal pods (priority 0).
	defaultPriorityClassValue = -1

	// pausePodLabel is the label used to identify pause pods managed by this controller.
	pausePodLabel = "runner.gitlab.com/pause-pod"
	// pausePodLabelValue is the value for the pause pod label.
	pausePodLabelValue = "true"
	// runnerIDLabel identifies which runner token owns the pause pod.
	runnerIDLabel = "manager.runner.gitlab.com/id-short"
	// systemIDLabel identifies the runner instance (for multiple processes sharing a token).
	systemIDLabel = "manager.runner.gitlab.com/system-id"

	// heartbeatAnnotation stores the last heartbeat timestamp for orphan detection.
	heartbeatAnnotation = "runner.gitlab.com/pause-heartbeat"

	// defaultReconcileInterval is how often the manager reconciles pause pod count.
	defaultReconcileInterval = 10 * time.Second

	// heartbeatInterval is how often we update the heartbeat annotation.
	heartbeatInterval = 1 * time.Minute

	// orphanThreshold is how long a deployment can go without a heartbeat before
	// it's considered orphaned and eligible for cleanup.
	orphanThreshold = 1 * time.Hour
)

// PausePodManagerConfig holds configuration for the pause pod manager.
type PausePodManagerConfig struct {
	// Namespace where pause pods are created.
	Namespace string
	// RunnerShortToken is the short runner token identifier.
	RunnerShortToken string
	// RunnerName is the human-readable name of the runner worker.
	RunnerName string
	// SystemID uniquely identifies this runner instance.
	SystemID string
	// MaxPausePods limits the number of pause pods. 0 means unlimited.
	MaxPausePods int
	// Image for pause pods. Defaults to defaultPausePodImage.
	Image string
	// PriorityClassName for pause pods (should be lower than job pods).
	PriorityClassName string
	// Policies define when and how many pause pods to maintain.
	Policies PolicyList
	// ResourceRequests for pause pods (should match job pod requests).
	ResourceRequests api.ResourceList
	// NodeSelector for pause pods.
	NodeSelector map[string]string
	// Tolerations for pause pods.
	Tolerations []api.Toleration
	// ServiceAccountName for pause pods.
	ServiceAccountName string
	// RuntimeClassName for pause pods.
	RuntimeClassName *string
}

// PausePodManager manages a Deployment of pause pods for pre-warming cluster capacity.
type PausePodManager struct {
	config  PausePodManagerConfig
	client  kubernetes.Interface
	log     logrus.FieldLogger
	metrics *Metrics

	mu                  sync.RWMutex
	activeJobs          int
	lastDesiredReplicas int
	scaleDownAllowedAt  time.Time
	lastHeartbeat       time.Time
	stopCh              chan struct{}
	stopped             bool
}

// NewPausePodManager creates a new pause pod manager.
func NewPausePodManager(client kubernetes.Interface, config PausePodManagerConfig, log logrus.FieldLogger, metrics *Metrics) (*PausePodManager, error) {
	if err := config.Policies.ParseAll(); err != nil {
		return nil, fmt.Errorf("parsing policies: %w", err)
	}

	return &PausePodManager{
		config:  config,
		client:  client,
		log:     log.WithField("component", "pause-pod-manager"),
		metrics: metrics,
		stopCh:  make(chan struct{}),
	}, nil
}

// Start begins the pause pod reconciliation loop.
func (m *PausePodManager) Start(ctx context.Context) {
	m.log.Info("Starting pause pod manager")

	// Ensure PriorityClass exists if using default
	if err := m.ensurePriorityClass(ctx); err != nil {
		m.log.WithError(err).Warn("Failed to ensure PriorityClass exists")
	}

	ticker := time.NewTicker(defaultReconcileInterval)
	defer ticker.Stop()

	// Initial reconciliation
	m.reconcile(ctx)

	for {
		select {
		case <-ctx.Done():
			m.log.Info("Pause pod manager stopping due to context cancellation")
			return
		case <-m.stopCh:
			m.log.Info("Pause pod manager stopped")
			return
		case <-ticker.C:
			m.reconcile(ctx)
		}
	}
}

// Stop stops the pause pod manager and cleans up the deployment.
func (m *PausePodManager) Stop(ctx context.Context) {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	close(m.stopCh)
	m.mu.Unlock()

	// Clean up deployment on shutdown
	m.log.Info("Cleaning up pause pod deployment on shutdown")
	if err := m.deleteDeployment(ctx); err != nil {
		m.log.WithError(err).Warn("Failed to clean up pause pod deployment")
	}
}

// SetActiveJobs sets the number of currently running jobs.
func (m *PausePodManager) SetActiveJobs(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeJobs = count
}

// IncrementActiveJobs increments the active job count.
func (m *PausePodManager) IncrementActiveJobs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeJobs++
}

// DecrementActiveJobs decrements the active job count.
func (m *PausePodManager) DecrementActiveJobs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeJobs > 0 {
		m.activeJobs--
	}
}

func (m *PausePodManager) getActiveJobs() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeJobs
}

func (m *PausePodManager) deploymentName() string {
	return fmt.Sprintf("runner-pause-%s", dns.MakeRFC1123Compatible(m.config.RunnerShortToken+"-"+m.config.SystemID))
}

func (m *PausePodManager) reconcile(ctx context.Context) {
	log := m.log.WithField("operation", "reconcile")

	currentReplicas, deploymentExists, err := m.getCurrentReplicas(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to get pause pod deployment")
		if m.metrics != nil {
			m.metrics.IncReconcileErrors(m.config.RunnerShortToken, m.config.RunnerName, m.config.SystemID, "get_deployment")
		}
		return
	}

	now := time.Now()
	policy := m.config.Policies.Active(now)
	desiredReplicas := m.calculateDesiredReplicas(policy)
	targetReplicas := m.applyScaleDownCooldown(desiredReplicas, policy, now)
	activeJobs := m.getActiveJobs()

	log.WithFields(logrus.Fields{
		"current_replicas": currentReplicas,
		"desired_replicas": desiredReplicas,
		"target_replicas":  targetReplicas,
		"active_jobs":      activeJobs,
	}).Debug("Reconciling pause pod deployment")

	if m.metrics != nil {
		m.metrics.SetCurrentPods(m.config.RunnerShortToken, m.config.RunnerName, m.config.SystemID, currentReplicas)
		m.metrics.SetDesiredPods(m.config.RunnerShortToken, m.config.RunnerName, m.config.SystemID, desiredReplicas)
	}

	if err := m.applyDesiredState(ctx, deploymentExists, currentReplicas, targetReplicas); err != nil {
		log.WithError(err).Error("Failed to apply desired deployment state")
		if m.metrics != nil {
			m.metrics.IncReconcileErrors(m.config.RunnerShortToken, m.config.RunnerName, m.config.SystemID, "apply_state")
		}
		return
	}

	if deploymentExists && time.Since(m.lastHeartbeat) >= heartbeatInterval {
		if err := m.updateHeartbeat(ctx); err != nil {
			log.WithError(err).Warn("Failed to update heartbeat")
		}
	}
}

// getCurrentReplicas returns the number of current replicas of the pause deployment.
// Second value is set to false if the deployment doesn't exist.
func (m *PausePodManager) getCurrentReplicas(ctx context.Context) (int, bool, error) {
	deployment, err := m.getDeployment(ctx)
	if errors.IsNotFound(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if deployment.Spec.Replicas == nil {
		return 0, true, nil
	}
	return int(*deployment.Spec.Replicas), true, nil
}

func (m *PausePodManager) calculateDesiredReplicas(policy Policy) int {
	desired := policy.IdleCount

	// Apply scale factor if configured
	if policy.ScaleFactor > 0 {
		scaled := int(math.Ceil(policy.ScaleFactor * float64(m.getActiveJobs())))
		if policy.ScaleFactorLimit > 0 {
			scaled = min(scaled, policy.ScaleFactorLimit)
		}
		desired = max(desired, scaled)
	}

	// Don't exceed max pods
	if m.config.MaxPausePods > 0 {
		desired = min(desired, m.config.MaxPausePods)
	}

	return desired
}

// applyScaleDownCooldown applies the idle_time cooldown for scale-down operations.
// Scale-up is immediate, but scale-down waits for idle_time to prevent thrashing.
func (m *PausePodManager) applyScaleDownCooldown(desired int, policy Policy, now time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Scale up is always immediate
	if desired >= m.lastDesiredReplicas {
		m.lastDesiredReplicas = desired
		m.scaleDownAllowedAt = now.Add(policy.IdleTime)
		return desired
	}

	// Scale down: check cooldown
	if now.Before(m.scaleDownAllowedAt) {
		// Still in cooldown, keep previous count
		return m.lastDesiredReplicas
	}

	// Cooldown expired, allow scale down
	m.lastDesiredReplicas = desired
	m.scaleDownAllowedAt = now.Add(policy.IdleTime)
	return desired
}

func (m *PausePodManager) applyDesiredState(ctx context.Context, exists bool, current, desired int) error {
	if !exists {
		if desired > 0 {
			if m.metrics != nil {
				m.metrics.IncScaleUp(m.config.RunnerShortToken, m.config.RunnerName, m.config.SystemID)
			}
			return m.createDeployment(ctx, desired)
		}
		return nil
	}

	if current < desired {
		if m.metrics != nil {
			m.metrics.IncScaleUp(m.config.RunnerShortToken, m.config.RunnerName, m.config.SystemID)
		}
		return m.updateDeploymentReplicas(ctx, desired)
	}

	if current > desired {
		if m.metrics != nil {
			m.metrics.IncScaleDown(m.config.RunnerShortToken, m.config.RunnerName, m.config.SystemID)
		}
		return m.updateDeploymentReplicas(ctx, desired)
	}

	return nil
}

func (m *PausePodManager) ensurePriorityClass(ctx context.Context) error {
	// Only create if using default priority class name (or none specified)
	if m.config.PriorityClassName != "" && m.config.PriorityClassName != defaultPriorityClassName {
		return nil
	}

	// Check if it already exists
	// kubeAPI: scheduling.k8s.io/priorityclasses, get, kubernetes.autoscaler
	_, err := m.client.SchedulingV1().PriorityClasses().Get(ctx, defaultPriorityClassName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// Create the PriorityClass
	pc := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultPriorityClassName,
		},
		Value:         defaultPriorityClassValue,
		GlobalDefault: false,
		Description:   "Low priority class for GitLab Runner pause pods. These pods reserve capacity and are preempted when job pods need resources.",
	}

	// kubeAPI: scheduling.k8s.io/priorityclasses, create, kubernetes.autoscaler
	_, err = m.client.SchedulingV1().PriorityClasses().Create(ctx, pc, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		// Race condition - another runner created it
		return nil
	}
	if err != nil {
		return err
	}

	m.log.WithField("priority_class", defaultPriorityClassName).Info("Created PriorityClass for pause pods")
	return nil
}

func (m *PausePodManager) getDeployment(ctx context.Context) (*appsv1.Deployment, error) {
	// kubeAPI: apps/deployments, get, kubernetes.autoscaler
	return m.client.AppsV1().Deployments(m.config.Namespace).Get(ctx, m.deploymentName(), metav1.GetOptions{})
}

func (m *PausePodManager) createDeployment(ctx context.Context, replicas int) error {
	deployment := m.buildDeployment(replicas)

	// kubeAPI: apps/deployments, create, kubernetes.autoscaler
	_, err := m.client.AppsV1().Deployments(m.config.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	m.lastHeartbeat = time.Now()

	m.log.WithFields(logrus.Fields{
		"deployment": deployment.Name,
		"replicas":   replicas,
	}).Info("Created pause pod deployment")

	return nil
}

func (m *PausePodManager) updateDeploymentReplicas(ctx context.Context, replicas int) error {
	deployment, err := m.getDeployment(ctx)
	if err != nil {
		return err
	}

	r := int32(replicas)
	deployment.Spec.Replicas = &r

	// kubeAPI: apps/deployments, update, kubernetes.autoscaler
	_, err = m.client.AppsV1().Deployments(m.config.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	m.log.WithFields(logrus.Fields{
		"deployment": deployment.Name,
		"replicas":   replicas,
	}).Info("Updated pause pod deployment replicas")

	return nil
}

func (m *PausePodManager) updateHeartbeat(ctx context.Context) error {
	deployment, err := m.getDeployment(ctx)
	if err != nil {
		return err
	}

	if deployment.Annotations == nil {
		deployment.Annotations = make(map[string]string)
	}
	deployment.Annotations[heartbeatAnnotation] = time.Now().UTC().Format(time.RFC3339)

	// kubeAPI: apps/deployments, update, kubernetes.autoscaler
	_, err = m.client.AppsV1().Deployments(m.config.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	m.lastHeartbeat = time.Now()
	return nil
}

func (m *PausePodManager) deleteDeployment(ctx context.Context) error {
	// kubeAPI: apps/deployments, delete, kubernetes.autoscaler
	err := m.client.AppsV1().Deployments(m.config.Namespace).Delete(ctx, m.deploymentName(), metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

func (m *PausePodManager) buildDeployment(replicas int) *appsv1.Deployment {
	image := m.config.Image
	if image == "" {
		image = defaultPausePodImage
	}

	labels := map[string]string{
		pausePodLabel: pausePodLabelValue,
		runnerIDLabel: m.config.RunnerShortToken,
		systemIDLabel: m.config.SystemID,
	}

	r := int32(replicas)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.deploymentName(),
			Namespace: m.config.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				heartbeatAnnotation: time.Now().UTC().Format(time.RFC3339),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:  "pause",
							Image: image,
							Resources: api.ResourceRequirements{
								Requests: m.config.ResourceRequests,
							},
						},
					},
					TerminationGracePeriodSeconds: int64Ptr(0),
					NodeSelector:                  m.config.NodeSelector,
					Tolerations:                   m.config.Tolerations,
					ServiceAccountName:            m.config.ServiceAccountName,
					RuntimeClassName:              m.config.RuntimeClassName,
				},
			},
		},
	}

	if m.config.PriorityClassName != "" {
		deployment.Spec.Template.Spec.PriorityClassName = m.config.PriorityClassName
	}

	return deployment
}

// BuildResourceRequests creates resource requests matching typical job pod requirements.
func BuildResourceRequests(cpuRequest, memoryRequest string) api.ResourceList {
	resources := api.ResourceList{}

	if cpuRequest != "" {
		if q, err := resource.ParseQuantity(cpuRequest); err == nil {
			resources[api.ResourceCPU] = q
		}
	}

	if memoryRequest != "" {
		if q, err := resource.ParseQuantity(memoryRequest); err == nil {
			resources[api.ResourceMemory] = q
		}
	}

	return resources
}

func int64Ptr(i int64) *int64 {
	return &i
}

// CleanupOrphanedDeployments removes pause pod deployments that haven't received
// a heartbeat within the orphanThreshold. This handles the case where a runner
// process dies without cleaning up its deployment.
//
// This function is called once at startup. Deployments orphaned less than
// orphanThreshold ago will not be cleaned up until the next runner restart.
func CleanupOrphanedDeployments(ctx context.Context, client kubernetes.Interface, namespace string, log logrus.FieldLogger) error {
	// kubeAPI: apps/deployments, list, kubernetes.autoscaler
	deployments, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: pausePodLabel + "=" + pausePodLabelValue,
	})
	if err != nil {
		return err
	}

	for _, deployment := range deployments.Items {
		heartbeat, ok := deployment.Annotations[heartbeatAnnotation]
		if !ok {
			continue
		}

		heartbeatTime, err := time.Parse(time.RFC3339, heartbeat)
		if err != nil {
			log.WithError(err).WithField("deployment", deployment.Name).Warn("Failed to parse heartbeat annotation")
			continue
		}

		if time.Since(heartbeatTime) > orphanThreshold {
			log.WithFields(logrus.Fields{
				"deployment":     deployment.Name,
				"last_heartbeat": heartbeat,
			}).Info("Cleaning up orphaned pause pod deployment")

			// kubeAPI: apps/deployments, delete, kubernetes.autoscaler
			if err := client.AppsV1().Deployments(namespace).Delete(ctx, deployment.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				log.WithError(err).WithField("deployment", deployment.Name).Warn("Failed to delete orphaned deployment")
			}
		}
	}

	return nil
}

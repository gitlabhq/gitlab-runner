package autoscaler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	api "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Compile-time interface assertions.
var (
	_ common.ShutdownAwareExecutorProvider = (*Provider)(nil)
	_ common.RuntimeEnvironmentSweeper     = (*Provider)(nil)
)

// Provider wraps an ExecutorProvider to add pause pod management.
type Provider struct {
	common.ExecutorProvider

	mu       sync.Mutex                     // managers and draining
	managers map[string]*autoscalingManager // keyed by runner token
	draining bool
	metrics  *Metrics

	// For testing
	newKubeClient func(*restclient.Config) (kubernetes.Interface, error)
	getKubeConfig func(*common.KubernetesConfig) (*restclient.Config, error)
}

// autoscalingManager is in the map before its manager exists, so p.mu is never held across the API
// calls that create one. ready is closed once manager and err are set.
type autoscalingManager struct {
	configLoadedAt string
	ready          chan struct{}

	manager *PausePodManager
	cancel  context.CancelFunc
	err     error
}

func (rm *autoscalingManager) stop(ctx context.Context) {
	<-rm.ready
	if rm.manager != nil {
		rm.manager.Stop(ctx)
		rm.cancel()
	}
}

// NewProvider creates a new provider that wraps the given executor provider
// and adds pause pod management capabilities.
func NewProvider(ep common.ExecutorProvider) *Provider {
	return &Provider{
		ExecutorProvider: ep,
		managers:         make(map[string]*autoscalingManager),
		metrics:          NewMetrics(),
		newKubeClient: func(c *restclient.Config) (kubernetes.Interface, error) {
			return kubernetes.NewForConfig(c)
		},
		getKubeConfig: getKubeConfig,
	}
}

// Describe implements prometheus.Collector.
func (p *Provider) Describe(ch chan<- *prometheus.Desc) {
	p.metrics.Describe(ch)
}

// Collect implements prometheus.Collector.
func (p *Provider) Collect(ch chan<- prometheus.Metric) {
	p.metrics.Collect(ch)
}

// Init implements ManagedExecutorProvider.
func (p *Provider) Init(_ *common.Config) {}

// BeginShutdown implements ShutdownAwareExecutorProvider.
func (p *Provider) BeginShutdown(_ *common.Config) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.draining = true
	for _, rm := range p.managers {
		if rm.manager != nil {
			rm.manager.BeginShutdown()
		}
	}
}

// Shutdown implements ManagedExecutorProvider.
func (p *Provider) Shutdown(ctx context.Context, _ *common.Config) {
	p.mu.Lock()
	managers := p.managers
	p.managers = make(map[string]*autoscalingManager)
	p.mu.Unlock()

	var wg sync.WaitGroup
	for _, rm := range managers {
		wg.Go(func() { rm.stop(ctx) })
	}
	wg.Wait()
}

// SweepRuntimeEnvironments forwards to the wrapped provider. The embedded
// interface does not promote methods outside common.ExecutorProvider, so
// without this the sweep loop would never see the Kubernetes implementation.
func (p *Provider) SweepRuntimeEnvironments(ctx context.Context, config *common.RunnerConfig, now time.Time) error {
	sweeper, ok := p.ExecutorProvider.(common.RuntimeEnvironmentSweeper)
	if !ok {
		return nil
	}
	return sweeper.SweepRuntimeEnvironments(ctx, config, now)
}

// Acquire acquires resources and ensures pause pod manager is running.
func (p *Provider) Acquire(config *common.RunnerConfig) (common.ExecutorData, error) {
	return p.acquire(config, true)
}

// AcquireCanary implements CanaryAcquirer. The canary's pod is what makes room: it preempts a pause pod
// like any job pod, so it must not wait for one. On a rollout this is what lets a new replica become
// Ready while the old replica's pause pods hold every slot.
func (p *Provider) AcquireCanary(config *common.RunnerConfig) (common.ExecutorData, error) {
	return p.acquire(config, false)
}

func (p *Provider) acquire(config *common.RunnerConfig, check bool) (common.ExecutorData, error) {
	manager, err := p.manager(config)
	if err != nil {
		return nil, err
	}

	reserved := false
	if check && manager != nil && manager.preemptive(time.Now()) {
		if !manager.Reserve() {
			logrus.WithField("runner", config.ShortDescription()).Debug("Declining job request: no scheduled pause pod to take the job")
			return nil, &common.NoFreeExecutorError{Message: "no scheduled pause pod to take the job"}
		}
		reserved = true
	}

	data, err := p.ExecutorProvider.Acquire(config)
	if err != nil {
		if reserved {
			manager.Release()
		}
		return data, err
	}

	if manager == nil {
		return data, nil
	}

	manager.IncrementActiveJobs()

	acq := &acquisition{ExecutorData: data}
	if reserved {
		acq.release = manager.Release
	}

	return acq, nil
}

// manager returns the runner's pause pod manager, starting it if needed. A manager that cannot start
// is an error while the active policy has preemptive mode, since the check cannot run without it, and
// a warning otherwise, as before.
func (p *Provider) manager(config *common.RunnerConfig) (*PausePodManager, error) {
	manager, err := p.ensureManager(config)
	if err == nil {
		return manager, nil
	}

	policies := policiesFromConfig(config.Kubernetes.Autoscaler)
	if policies.ParseAll() == nil && policies.Active(time.Now()).Preemptive() {
		return nil, fmt.Errorf("pause pod manager: %w", err)
	}

	logrus.WithError(err).Warn("Failed to start pause pod manager")
	return nil, nil
}

// Release releases resources and decrements the active job count.
func (p *Provider) Release(config *common.RunnerConfig, data common.ExecutorData) {
	if a, ok := data.(*acquisition); ok {
		a.end()
		data = a.ExecutorData
	}

	if manager := p.GetManager(config); manager != nil {
		manager.DecrementActiveJobs()
	}

	p.ExecutorProvider.Release(config, data)
}

func policiesFromConfig(autoscalerConfig *common.KubernetesAutoscalerConfig) PolicyList {
	policies := make(PolicyList, len(autoscalerConfig.Policy))
	for i, pc := range autoscalerConfig.Policy {
		policies[i] = Policy{
			Periods:          pc.Periods,
			Timezone:         pc.Timezone,
			IdleCount:        pc.IdleCount,
			IdleTime:         pc.IdleTime,
			ScaleFactor:      pc.ScaleFactor,
			ScaleFactorLimit: pc.ScaleFactorLimit,
			PreemptiveMode:   pc.PreemptiveMode != nil && *pc.PreemptiveMode,
		}
	}
	return policies
}

// acquisition wraps the executor's data with the reservation. Only LogFields is forwarded.
type acquisition struct {
	common.ExecutorData

	release func()
	once    sync.Once
}

func (a *acquisition) Unwrap() common.ExecutorData {
	return a.ExecutorData
}

func (a *acquisition) PodScheduled() {
	a.end()
}

func (a *acquisition) end() {
	a.once.Do(func() {
		if a.release != nil {
			a.release()
		}
	})
}

func (a *acquisition) LogFields() map[string]string {
	if l, ok := a.ExecutorData.(common.ExecutorDataLogger); ok {
		return l.LogFields()
	}
	return nil
}

func (p *Provider) ensureManager(config *common.RunnerConfig) (*PausePodManager, error) {
	if config.Kubernetes == nil || config.Kubernetes.Autoscaler == nil {
		return nil, nil
	}

	if len(config.Kubernetes.Autoscaler.Policy) == 0 {
		return nil, nil
	}

	token := config.GetToken()
	configKey := configLoadedKey(config)

	p.mu.Lock()
	previous, exists := p.managers[token]
	if exists && previous.configLoadedAt == configKey {
		p.mu.Unlock()
		<-previous.ready
		return previous.manager, previous.err
	}
	rm := &autoscalingManager{configLoadedAt: configKey, ready: make(chan struct{})}
	p.managers[token] = rm
	p.mu.Unlock()

	defer close(rm.ready)

	if exists {
		previous.stop(context.Background())
	}

	manager, cancel, err := p.createManager(config)

	p.mu.Lock()
	if err != nil {
		rm.err = err
		if p.managers[token] == rm {
			delete(p.managers, token)
		}
		p.mu.Unlock()
		return nil, err
	}
	rm.manager, rm.cancel = manager, cancel
	draining := p.draining
	p.mu.Unlock()

	if draining {
		manager.BeginShutdown()
	}

	return manager, nil
}

func (p *Provider) createManager(config *common.RunnerConfig) (*PausePodManager, context.CancelFunc, error) {
	k8sConfig := config.Kubernetes
	autoscalerConfig := k8sConfig.Autoscaler

	// ensureManager retries on every Acquire, so reject an invalid config before
	// creating a client or making any API call.
	if err := validatePodMetadata(autoscalerConfig.PausePodLabels, autoscalerConfig.PausePodAnnotations); err != nil {
		return nil, nil, err
	}

	kubeConfig, err := p.getKubeConfig(k8sConfig)
	if err != nil {
		return nil, nil, err
	}

	client, err := p.newKubeClient(kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	log := logrus.WithFields(logrus.Fields{
		"runner":    config.ShortDescription(),
		"namespace": k8sConfig.Namespace,
	})

	// Clean up any orphaned deployments from previous runner instances
	if err := CleanupOrphanedDeployments(context.Background(), client, k8sConfig.Namespace, log); err != nil {
		log.WithError(err).Warn("Failed to cleanup orphaned deployments")
	}

	policies := policiesFromConfig(autoscalerConfig)

	priorityClassName := autoscalerConfig.PausePodPriorityClassName
	if priorityClassName == "" {
		priorityClassName = defaultPriorityClassName
	}

	managerConfig := PausePodManagerConfig{
		Namespace:          k8sConfig.Namespace,
		RunnerShortToken:   config.ShortDescription(),
		RunnerName:         config.Name,
		SystemID:           config.GetSystemID(),
		MaxPausePods:       autoscalerConfig.MaxPausePods,
		Image:              autoscalerConfig.PausePodImage,
		PriorityClassName:  priorityClassName,
		Policies:           policies,
		ResourceRequests:   BuildResourceRequests(k8sConfig),
		NodeSelector:       k8sConfig.NodeSelector,
		Tolerations:        k8sConfig.GetNodeTolerations(),
		Affinity:           pausePodAffinity(k8sConfig),
		SchedulerName:      k8sConfig.SchedulerName,
		ServiceAccountName: k8sConfig.ServiceAccount,
		RuntimeClassName:   k8sConfig.RuntimeClassName,
		ImagePullSecrets:   localObjectReferences(autoscalerConfig.PausePodImagePullSecrets),
		Labels:             autoscalerConfig.PausePodLabels,
		Annotations:        autoscalerConfig.PausePodAnnotations,
	}

	manager, err := NewPausePodManager(client, managerConfig, log, p.metrics)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	go manager.Start(ctx)

	return manager, cancel, nil
}

// GetManager returns the pause pod manager for a runner, if one exists.
// This is used by the executor to update active job counts.
func (p *Provider) GetManager(config *common.RunnerConfig) *PausePodManager {
	p.mu.Lock()
	defer p.mu.Unlock()

	rm, exists := p.managers[config.GetToken()]
	if !exists {
		return nil
	}
	return rm.manager
}

// Pod affinity terms select on job labels pause pods lack, and a required anti-affinity term on a pause
// pod blocks the job pod from its node.
func pausePodAffinity(k8sConfig *common.KubernetesConfig) *api.Affinity {
	if k8sConfig.Affinity.NodeAffinity == nil {
		return nil
	}
	return &api.Affinity{NodeAffinity: k8sConfig.GetNodeAffinity()}
}

func configLoadedKey(config *common.RunnerConfig) string {
	return config.ConfigLoadedAt.String()
}

func getKubeConfig(k8sConfig *common.KubernetesConfig) (*restclient.Config, error) {
	if k8sConfig.Host != "" {
		return &restclient.Config{
			Host:        k8sConfig.Host,
			BearerToken: k8sConfig.BearerToken,
			TLSClientConfig: restclient.TLSClientConfig{
				CAFile:   k8sConfig.CAFile,
				CertFile: k8sConfig.CertFile,
				KeyFile:  k8sConfig.KeyFile,
			},
		}, nil
	}

	// Use in-cluster config or kubeconfig
	config, err := restclient.InClusterConfig()
	if err != nil {
		// Fall back to default kubeconfig
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{CurrentContext: k8sConfig.Context},
		).ClientConfig()
	}
	return config, nil
}

func localObjectReferences(names []string) []api.LocalObjectReference {
	refs := make([]api.LocalObjectReference, 0, len(names))

	for _, name := range names {
		if name == "" {
			continue
		}
		refs = append(refs, api.LocalObjectReference{Name: name})
	}

	return refs
}

package autoscaler

import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// Compile-time interface assertion.
var _ common.ManagedExecutorProvider = (*Provider)(nil)

// Provider wraps an ExecutorProvider to add pause pod management.
type Provider struct {
	common.ExecutorProvider

	mu       sync.Mutex
	managers map[string]*autoscalingManager // keyed by runner token
	metrics  *Metrics

	// For testing
	newKubeClient func(*restclient.Config) (kubernetes.Interface, error)
	getKubeConfig func(*common.KubernetesConfig) (*restclient.Config, error)
}

type autoscalingManager struct {
	manager        *PausePodManager
	cancel         context.CancelFunc
	configLoadedAt string
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
func (p *Provider) Init() {}

// Shutdown implements ManagedExecutorProvider.
func (p *Provider) Shutdown(ctx context.Context, _ *common.Config) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var wg sync.WaitGroup
	for token, rm := range p.managers {
		wg.Go(func() {
			rm.manager.Stop(ctx)
			rm.cancel()
		})
		delete(p.managers, token)
	}
	wg.Wait()
}

// Acquire acquires resources and ensures pause pod manager is running.
func (p *Provider) Acquire(config *common.RunnerConfig) (common.ExecutorData, error) {
	if err := p.ensureManager(config); err != nil {
		logrus.WithError(err).Warn("Failed to start pause pod manager")
	}

	data, err := p.ExecutorProvider.Acquire(config)
	if err != nil {
		return data, err
	}

	if manager := p.GetManager(config); manager != nil {
		manager.IncrementActiveJobs()
	}

	return data, nil
}

// Release releases resources and decrements the active job count.
func (p *Provider) Release(config *common.RunnerConfig, data common.ExecutorData) {
	if manager := p.GetManager(config); manager != nil {
		manager.DecrementActiveJobs()
	}

	p.ExecutorProvider.Release(config, data)
}

func (p *Provider) ensureManager(config *common.RunnerConfig) error {
	if config.Kubernetes == nil || config.Kubernetes.Autoscaler == nil {
		return nil
	}

	if len(config.Kubernetes.Autoscaler.Policy) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	token := config.GetToken()
	rm, exists := p.managers[token]

	// Check if config changed
	configKey := configLoadedKey(config)
	if exists && rm.configLoadedAt == configKey {
		return nil
	}

	// Stop existing manager if config changed
	if exists {
		rm.manager.Stop(context.Background())
		rm.cancel()
		delete(p.managers, token)
	}

	// Create new manager
	manager, cancel, err := p.createManager(config)
	if err != nil {
		return err
	}

	p.managers[token] = &autoscalingManager{
		manager:        manager,
		cancel:         cancel,
		configLoadedAt: configKey,
	}

	return nil
}

func (p *Provider) createManager(config *common.RunnerConfig) (*PausePodManager, context.CancelFunc, error) {
	k8sConfig := config.Kubernetes
	autoscalerConfig := k8sConfig.Autoscaler

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

	// Build policy list from config
	policies := make(PolicyList, len(autoscalerConfig.Policy))
	for i, pc := range autoscalerConfig.Policy {
		policies[i] = Policy{
			Periods:          pc.Periods,
			Timezone:         pc.Timezone,
			IdleCount:        pc.IdleCount,
			IdleTime:         pc.IdleTime,
			ScaleFactor:      pc.ScaleFactor,
			ScaleFactorLimit: pc.ScaleFactorLimit,
		}
	}

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
		ResourceRequests:   BuildResourceRequests(k8sConfig.CPURequest, k8sConfig.MemoryRequest),
		NodeSelector:       k8sConfig.NodeSelector,
		Tolerations:        k8sConfig.GetNodeTolerations(),
		ServiceAccountName: k8sConfig.ServiceAccount,
		RuntimeClassName:   k8sConfig.RuntimeClassName,
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

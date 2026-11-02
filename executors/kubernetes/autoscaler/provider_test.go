//go:build !integration

package autoscaler

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestProvider_AcquireRelease_TracksActiveJobs(t *testing.T) {
	mockProvider := common.NewMockExecutorProvider(t)
	mockProvider.EXPECT().Acquire(mock.Anything).Return(nil, nil)
	mockProvider.EXPECT().Release(mock.Anything, mock.Anything).Return()

	provider := NewProvider(mockProvider)

	// Stub out kube client creation
	provider.newKubeClient = func(*restclient.Config) (kubernetes.Interface, error) {
		return fake.NewClientset(), nil
	}
	provider.getKubeConfig = func(*common.KubernetesConfig) (*restclient.Config, error) {
		return &restclient.Config{}, nil
	}

	config := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-token",
		},
		RunnerSettings: common.RunnerSettings{
			Kubernetes: &common.KubernetesConfig{
				Namespace: "default",
				Autoscaler: &common.KubernetesAutoscalerConfig{
					Policy: []common.AutoscalerPolicyConfig{
						{
							IdleCount: 2,
							Periods:   []string{"* * * * *"},
						},
					},
				},
			},
		},
		ConfigLoadedAt: time.Now(),
	}

	// First acquire - should create manager and increment
	_, err := provider.Acquire(config)
	require.NoError(t, err)

	manager := provider.GetManager(config)
	require.NotNil(t, manager)
	assert.Equal(t, 1, manager.getActiveJobs())

	// Second acquire - should increment again
	_, err = provider.Acquire(config)
	require.NoError(t, err)
	assert.Equal(t, 2, manager.getActiveJobs())

	// Release - should decrement
	provider.Release(config, nil)
	assert.Equal(t, 1, manager.getActiveJobs())

	// Release again
	provider.Release(config, nil)
	assert.Equal(t, 0, manager.getActiveJobs())
}

func TestProvider_Acquire_NoAutoscalerConfig(t *testing.T) {
	mockProvider := common.NewMockExecutorProvider(t)
	mockProvider.EXPECT().Acquire(mock.Anything).Return(nil, nil)

	provider := NewProvider(mockProvider)

	config := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-token",
		},
		RunnerSettings: common.RunnerSettings{
			Kubernetes: &common.KubernetesConfig{
				Namespace: "default",
			},
		},
	}

	_, err := provider.Acquire(config)
	require.NoError(t, err)

	// No manager should be created
	assert.Nil(t, provider.GetManager(config))
}

func TestProvider_Acquire_PassesPausePodLabelsAndAnnotations(t *testing.T) {
	mockProvider := common.NewMockExecutorProvider(t)
	mockProvider.EXPECT().Acquire(mock.Anything).Return(nil, nil)

	provider := NewProvider(mockProvider)

	provider.newKubeClient = func(*restclient.Config) (kubernetes.Interface, error) {
		return fake.NewClientset(), nil
	}
	provider.getKubeConfig = func(*common.KubernetesConfig) (*restclient.Config, error) {
		return &restclient.Config{}, nil
	}

	labels := map[string]string{"team": "platform-ci"}
	annotations := map[string]string{"arm/include": "true"}

	config := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-token",
		},
		RunnerSettings: common.RunnerSettings{
			Kubernetes: &common.KubernetesConfig{
				Namespace: "default",
				Autoscaler: &common.KubernetesAutoscalerConfig{
					Policy: []common.AutoscalerPolicyConfig{
						{
							IdleCount: 2,
							Periods:   []string{"* * * * *"},
						},
					},
					PausePodLabels:      labels,
					PausePodAnnotations: annotations,
				},
			},
		},
		ConfigLoadedAt: time.Now(),
	}

	_, err := provider.Acquire(config)
	require.NoError(t, err)

	manager := provider.GetManager(config)
	require.NotNil(t, manager)
	assert.Equal(t, labels, manager.config.Labels)
	assert.Equal(t, annotations, manager.config.Annotations)
}

func TestProvider_Acquire_InvalidPausePodLabelsDoesNotCreateManager(t *testing.T) {
	mockProvider := common.NewMockExecutorProvider(t)
	mockProvider.EXPECT().Acquire(mock.Anything).Return(nil, nil)

	provider := NewProvider(mockProvider)

	kubeClientCreated := false
	provider.newKubeClient = func(*restclient.Config) (kubernetes.Interface, error) {
		kubeClientCreated = true
		return fake.NewClientset(), nil
	}
	provider.getKubeConfig = func(*common.KubernetesConfig) (*restclient.Config, error) {
		return &restclient.Config{}, nil
	}

	config := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-token",
		},
		RunnerSettings: common.RunnerSettings{
			Kubernetes: &common.KubernetesConfig{
				Namespace: "default",
				Autoscaler: &common.KubernetesAutoscalerConfig{
					Policy: []common.AutoscalerPolicyConfig{
						{
							IdleCount: 2,
							Periods:   []string{"* * * * *"},
						},
					},
					PausePodLabels: map[string]string{"team name": "platform-ci"},
				},
			},
		},
		ConfigLoadedAt: time.Now(),
	}

	// Acquire must still succeed so the job runs; only the pause pod manager is skipped.
	_, err := provider.Acquire(config)
	require.NoError(t, err)

	assert.Nil(t, provider.GetManager(config))
	// ensureManager runs on every Acquire, so an invalid config must be rejected before
	// any Kubernetes client or API call is made, or each poll would waste a request.
	assert.False(t, kubeClientCreated, "kube client must not be created for an invalid config")
}

func TestProvider_ConfigReload_ReplacesManager(t *testing.T) {
	mockProvider := common.NewMockExecutorProvider(t)
	mockProvider.EXPECT().Acquire(mock.Anything).Return(nil, nil)

	provider := NewProvider(mockProvider)

	provider.newKubeClient = func(*restclient.Config) (kubernetes.Interface, error) {
		return fake.NewClientset(), nil
	}
	provider.getKubeConfig = func(*common.KubernetesConfig) (*restclient.Config, error) {
		return &restclient.Config{}, nil
	}

	configTime1 := time.Now()
	config := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-token",
		},
		RunnerSettings: common.RunnerSettings{
			Kubernetes: &common.KubernetesConfig{
				Namespace: "default",
				Autoscaler: &common.KubernetesAutoscalerConfig{
					Policy: []common.AutoscalerPolicyConfig{
						{
							IdleCount: 2,
							Periods:   []string{"* * * * *"},
						},
					},
				},
			},
		},
		ConfigLoadedAt: configTime1,
	}

	// First acquire creates a manager
	_, err := provider.Acquire(config)
	require.NoError(t, err)

	manager1 := provider.GetManager(config)
	require.NotNil(t, manager1)

	// Same config timestamp - should reuse same manager
	_, err = provider.Acquire(config)
	require.NoError(t, err)

	manager1Again := provider.GetManager(config)
	assert.Same(t, manager1, manager1Again, "same config should reuse manager")

	// Simulate config reload with new timestamp
	config2 := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-token",
		},
		RunnerSettings: common.RunnerSettings{
			Kubernetes: &common.KubernetesConfig{
				Namespace: "default",
				Autoscaler: &common.KubernetesAutoscalerConfig{
					Policy: []common.AutoscalerPolicyConfig{
						{
							IdleCount: 5,
							Periods:   []string{"* * * * *"},
						},
					},
				},
			},
		},
		ConfigLoadedAt: configTime1.Add(1 * time.Second),
	}

	_, err = provider.Acquire(config2)
	require.NoError(t, err)

	manager2 := provider.GetManager(config2)
	require.NotNil(t, manager2)
	assert.NotSame(t, manager1, manager2, "config reload should create new manager")
}

type sweepingExecutorProvider struct {
	*common.MockExecutorProvider
	*common.MockRuntimeEnvironmentSweeper
}

func TestProvider_SweepRuntimeEnvironments(t *testing.T) {
	cfg := &common.RunnerConfig{Name: "r"}
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)

	t.Run("forwards to a wrapped sweeper", func(t *testing.T) {
		inner := &sweepingExecutorProvider{
			MockExecutorProvider:          common.NewMockExecutorProvider(t),
			MockRuntimeEnvironmentSweeper: common.NewMockRuntimeEnvironmentSweeper(t),
		}
		inner.MockRuntimeEnvironmentSweeper.EXPECT().
			SweepRuntimeEnvironments(mock.Anything, cfg, now).
			Return(assert.AnError).
			Once()

		err := NewProvider(inner).SweepRuntimeEnvironments(t.Context(), cfg, now)
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("no-op when the wrapped provider cannot sweep", func(t *testing.T) {
		err := NewProvider(common.NewMockExecutorProvider(t)).SweepRuntimeEnvironments(t.Context(), cfg, now)
		assert.NoError(t, err)
	})
}

func TestProvider_BeginShutdown_DrainsManagers(t *testing.T) {
	mockProvider := common.NewMockExecutorProvider(t)
	mockProvider.EXPECT().Acquire(mock.Anything).Return(nil, nil).Once()

	provider := NewProvider(mockProvider)
	provider.newKubeClient = func(*restclient.Config) (kubernetes.Interface, error) {
		return fake.NewClientset(), nil
	}
	provider.getKubeConfig = func(*common.KubernetesConfig) (*restclient.Config, error) {
		return &restclient.Config{}, nil
	}

	config := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{Token: "test-token"},
		RunnerSettings: common.RunnerSettings{
			Kubernetes: &common.KubernetesConfig{
				Namespace: "default",
				Autoscaler: &common.KubernetesAutoscalerConfig{
					Policy: []common.AutoscalerPolicyConfig{{IdleCount: 2, Periods: []string{"* * * * *"}}},
				},
			},
		},
		ConfigLoadedAt: time.Now(),
	}

	_, err := provider.Acquire(config)
	require.NoError(t, err)
	manager := provider.GetManager(config)
	require.NotNil(t, manager)
	assert.False(t, manager.isDraining())

	provider.BeginShutdown(nil)

	assert.True(t, manager.isDraining())
}

func TestProvider_Acquire_PausePodsScheduleLikeJobPods(t *testing.T) {
	mockProvider := common.NewMockExecutorProvider(t)
	mockProvider.EXPECT().Acquire(mock.Anything).Return(nil, nil)

	provider := NewProvider(mockProvider)
	provider.newKubeClient = func(*restclient.Config) (kubernetes.Interface, error) {
		return fake.NewClientset(), nil
	}
	provider.getKubeConfig = func(*common.KubernetesConfig) (*restclient.Config, error) {
		return &restclient.Config{}, nil
	}

	config := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{Token: "test-token"},
		RunnerSettings: common.RunnerSettings{
			Kubernetes: &common.KubernetesConfig{
				Namespace:           "default",
				SchedulerName:       "custom-scheduler",
				PodCPURequest:       "2",
				PodMemoryRequest:    "4Gi",
				HelperMemoryRequest: "128Mi",
				Affinity: common.KubernetesAffinity{
					NodeAffinity: &common.KubernetesNodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &common.NodeSelector{
							NodeSelectorTerms: []common.NodeSelectorTerm{{
								MatchExpressions: []common.NodeSelectorRequirement{{Key: "zone", Operator: "In", Values: []string{"a"}}},
							}},
						},
					},
					PodAntiAffinity: &common.KubernetesPodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []common.PodAffinityTerm{{
							TopologyKey:   "kubernetes.io/hostname",
							LabelSelector: &common.LabelSelector{MatchLabels: map[string]string{"ci-job": "true"}},
						}},
					},
				},
				Autoscaler: &common.KubernetesAutoscalerConfig{
					Policy: []common.AutoscalerPolicyConfig{{IdleCount: 1, Periods: []string{"* * * * *"}}},
				},
			},
		},
		ConfigLoadedAt: time.Now(),
	}

	_, err := provider.Acquire(config)
	require.NoError(t, err)

	manager := provider.GetManager(config)
	require.NotNil(t, manager)
	podSpec := manager.buildDeployment(1).Spec.Template.Spec
	assert.Equal(t, "custom-scheduler", podSpec.SchedulerName)
	require.NotNil(t, podSpec.Affinity)
	assert.Equal(t, "zone", podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key)
	assert.Nil(t, podSpec.Affinity.PodAntiAffinity, "a required anti-affinity term on a pause pod blocks the job pod from its node")
	assert.Equal(t, "4Gi", podSpec.Containers[0].Resources.Requests.Memory().String())
	assert.Equal(t, "2", podSpec.Containers[0].Resources.Requests.Cpu().String())
}

func TestProvider_BeginShutdown_DoesNotWaitForManagerCreation(t *testing.T) {
	client := fake.NewClientset()
	listStarted := make(chan struct{})
	releaseList := make(chan struct{})
	client.PrependReactor("list", "deployments", func(k8stesting.Action) (bool, runtime.Object, error) {
		close(listStarted)
		<-releaseList
		return false, nil, nil
	})

	provider := NewProvider(common.NewMockExecutorProvider(t))
	provider.newKubeClient = func(*restclient.Config) (kubernetes.Interface, error) { return client, nil }
	provider.getKubeConfig = func(*common.KubernetesConfig) (*restclient.Config, error) { return &restclient.Config{}, nil }

	config := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{Token: "test-token"},
		RunnerSettings: common.RunnerSettings{
			Kubernetes: &common.KubernetesConfig{
				Namespace:  "default",
				Autoscaler: &common.KubernetesAutoscalerConfig{Policy: []common.AutoscalerPolicyConfig{{IdleCount: 2, Periods: []string{"* * * * *"}}}},
			},
		},
		ConfigLoadedAt: time.Now(),
	}

	created := make(chan error, 1)
	go func() { _, err := provider.ensureManager(config); created <- err }()
	<-listStarted

	done := make(chan struct{})
	go func() { provider.BeginShutdown(nil); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("BeginShutdown waited for the deployment list")
	}

	close(releaseList)
	require.NoError(t, <-created)
	manager := provider.GetManager(config)
	require.NotNil(t, manager)
	assert.True(t, manager.isDraining(), "a manager created during shutdown starts draining")
}

func TestProvider_EnsureManager_ConcurrentCallersShareOneManager(t *testing.T) {
	client := fake.NewClientset()
	listStarted := make(chan struct{})
	releaseList := make(chan struct{})
	var lists atomic.Int32
	client.PrependReactor("list", "deployments", func(k8stesting.Action) (bool, runtime.Object, error) {
		if lists.Add(1) == 1 {
			close(listStarted)
		}
		<-releaseList
		return false, nil, nil
	})

	provider := NewProvider(common.NewMockExecutorProvider(t))
	provider.newKubeClient = func(*restclient.Config) (kubernetes.Interface, error) { return client, nil }
	provider.getKubeConfig = func(*common.KubernetesConfig) (*restclient.Config, error) { return &restclient.Config{}, nil }
	config := signalProviderConfig(1)

	results := make(chan error, 2)
	go func() { _, err := provider.ensureManager(config); results <- err }()
	<-listStarted
	go func() { _, err := provider.ensureManager(config); results <- err }()

	select {
	case err := <-results:
		t.Fatalf("a caller returned before the manager existed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseList)
	require.NoError(t, <-results)
	require.NoError(t, <-results)
	assert.Equal(t, int32(1), lists.Load(), "one creation")
	require.NotNil(t, provider.GetManager(config))
}

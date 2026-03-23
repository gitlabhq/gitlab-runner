//go:build !integration

package autoscaler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"

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

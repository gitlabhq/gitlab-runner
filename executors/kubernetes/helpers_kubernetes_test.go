package kubernetes

import (
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

// GetKubeClientConfig is used to export the getKubeClientConfig function for integration tests
func GetKubeClientConfig(config *common.KubernetesConfig) (kubeConfig *restclient.Config, err error) {
	return getKubeClientConfig(config, new(overwrites))
}

// NewDefaultExecutorForTest is used to expose the executor to integration tests
func NewDefaultExecutorForTest() common.Executor {
	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executorOptions,
		},
	}
	e.getKubeConfig = getKubeClientConfig
	e.newKubeClient = func(config *restclient.Config) (kubernetes.Interface, error) {
		return kubernetes.NewForConfig(config)
	}
	return e
}

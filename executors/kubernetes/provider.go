package kubernetes

import (
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

type executorData struct {
	PodName string
}

func (d *executorData) LogFields() map[string]string {
	if d.PodName == "" {
		return nil
	}
	return map[string]string{"pod_name": d.PodName}
}

type executorProvider struct {
	executors.DefaultExecutorProvider

	// Used by SweepRuntimeEnvironments, which runs without a job.
	getKubeConfig func(*common.KubernetesConfig, *overwrites) (*restclient.Config, error)
	newKubeClient func(*restclient.Config) (kubernetes.Interface, error)
}

func (p executorProvider) Acquire(config *common.RunnerConfig) (common.ExecutorData, error) {
	return &executorData{}, nil
}

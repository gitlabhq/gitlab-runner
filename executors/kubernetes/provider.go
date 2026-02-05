package kubernetes

import (
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
}

func (p executorProvider) Acquire(config *common.RunnerConfig) (common.ExecutorData, error) {
	return &executorData{}, nil
}

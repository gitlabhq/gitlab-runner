package docker

import (
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

type executorData struct {
	ContainerName string
}

func (d *executorData) LogFields() map[string]string {
	if d.ContainerName == "" {
		return nil
	}
	return map[string]string{"container_name": strings.TrimPrefix(d.ContainerName, "/")}
}

type executorProvider struct {
	executors.DefaultExecutorProvider
}

func (p executorProvider) Acquire(config *common.RunnerConfig) (common.ExecutorData, error) {
	return &executorData{}, nil
}

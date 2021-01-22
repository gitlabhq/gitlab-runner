package docker

import (
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/pull"
)

var createPullManager = func(e *executor) (pull.Manager, error) {
	config := pull.ManagerConfig{
		DockerConfig: e.Config.Docker,
		AuthConfig:   e.Build.GetDockerAuthConfig(),
		ShellUser:    e.Shell().User,
		Credentials:  e.Build.Credentials,
	}

	pullManager := pull.NewManager(e.Context, &e.BuildLogger, config, e.client, func() {
		e.SetCurrentStage(ExecutorStagePullingImage)
	})

	return pullManager, nil
}

func (e *executor) createPullManager() error {
	pm, err := createPullManager(e)
	if err != nil {
		return err
	}

	e.pullManager = pm

	return nil
}

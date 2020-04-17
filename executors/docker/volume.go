package docker

import (
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes"
)

var createVolumesManager = func(e *executor) (volumes.Manager, error) {
	config := volumes.ManagerConfig{
		CacheDir:     e.Config.Docker.CacheDir,
		BasePath:     e.Build.FullProjectDir(),
		UniqueName:   e.Build.ProjectUniqueName(),
		DisableCache: e.Config.Docker.DisableCache,
	}

	volumesManager := volumes.NewManager(&e.BuildLogger, e.volumeParser, e.client, config)

	return volumesManager, nil
}

func (e *executor) createVolumesManager() error {
	vm, err := createVolumesManager(e)
	if err != nil {
		return err
	}

	e.volumesManager = vm

	return nil
}

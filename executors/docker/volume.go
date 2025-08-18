package docker

import (
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes"
)

var createVolumesManager = func(e *executor) (volumes.Manager, error) {
	config := volumes.ManagerConfig{
		CacheDir:      e.Config.Docker.CacheDir,
		BasePath:      e.Build.FullProjectDir(),
		UniqueName:    e.Build.ProjectRealUniqueName(),
		TemporaryName: e.getProjectUniqRandomizedName(),
		DisableCache:  e.Config.Docker.DisableCache,
		Driver:        e.Config.Docker.VolumeDriver,
		DriverOpts:    e.Config.Docker.VolumeDriverOps,
		Protected:     e.Build.IsProtected(),
	}

	if e.newVolumePermissionSetter != nil {
		setter, err := e.newVolumePermissionSetter()
		if err != nil {
			return nil, err
		}
		config.PermissionSetter = setter
	}

	volumesManager := volumes.NewManager(&e.BuildLogger, e.volumeParser, e.dockerConn, config, e.labeler)

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

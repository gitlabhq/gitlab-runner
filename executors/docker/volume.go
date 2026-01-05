package docker

import (
	"slices"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes"
)

var createVolumesManager = func(e *executor) (volumes.Manager, error) {
	// Note if any of the cache keys includes the `-protected` suffix (but not the `-non_protected` suffix).
	// See https://gitlab.com/gitlab-org/gitlab/-/work_items/494478.
	protectedKeyIdx := slices.IndexFunc(e.Build.Cache, func(c common.Cache) bool {
		return strings.HasSuffix(c.Key, "-protected") && !strings.HasSuffix(c.Key, "-non_protected")
	})

	config := volumes.ManagerConfig{
		CacheDir:      e.Config.Docker.CacheDir,
		BasePath:      e.Build.FullProjectDir(),
		UniqueName:    e.Build.ProjectRealUniqueName(),
		TemporaryName: e.getProjectUniqRandomizedName(),
		DisableCache:  e.Config.Docker.DisableCache,
		Driver:        e.Config.Docker.VolumeDriver,
		DriverOpts:    e.Config.Docker.VolumeDriverOps,
		// the volume should be protected if the ref is protected OR if any of the cache volumes have the protected
		// suffix. See https://gitlab.com/gitlab-org/gitlab/-/work_items/494478.
		Protected: e.Build.IsProtected() || protectedKeyIdx >= 0,
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

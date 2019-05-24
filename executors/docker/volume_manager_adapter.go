package docker

import (
	"context"

	"github.com/docker/docker/api/types/container"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	docker_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

type volumesManagerAdapter struct {
	docker_helpers.Client

	e *executor
}

func (a *volumesManagerAdapter) LabelContainer(container *container.Config, containerType string, otherLabels ...string) {
	container.Labels = a.e.getLabels(containerType, otherLabels...)
}

func (a *volumesManagerAdapter) WaitForContainer(id string) error {
	return a.e.waitForContainer(a.e.Context, id)
}

func (a *volumesManagerAdapter) RemoveContainer(ctx context.Context, id string) error {
	return a.e.removeContainer(ctx, id)
}

var createVolumesManager = func(e *executor) (volumes.Manager, error) {
	adapter := &volumesManagerAdapter{
		Client: e.client,
		e:      e,
	}

	helperImage, err := e.getPrebuiltImage()
	if err != nil {
		return nil, err
	}

	volumeParser, err := parser.New(e.info)
	if err != nil {
		return nil, err
	}

	ccManager := volumes.NewCacheContainerManager(
		e.Context,
		&e.BuildLogger,
		adapter,
		helperImage,
	)

	config := volumes.ManagerConfig{
		CacheDir:          e.Config.Docker.CacheDir,
		BaseContainerPath: e.Build.FullProjectDir(),
		UniqueName:        e.Build.ProjectUniqueName(),
		DisableCache:      e.Config.Docker.DisableCache,
	}

	volumesManager := volumes.NewManager(&e.BuildLogger, volumeParser, ccManager, config)

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

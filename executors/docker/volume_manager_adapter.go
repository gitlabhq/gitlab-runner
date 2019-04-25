package docker

import (
	"context"

	"github.com/docker/docker/api/types/container"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes"
	docker_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
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

func (e *executor) checkOutdatedHelperImage() bool {
	return !e.Build.IsFeatureFlagOn(featureflags.DockerHelperImageV2) && e.Config.Docker.HelperImage != ""
}

func (e *executor) getVolumesManager() (volumes.Manager, error) {
	if e.volumesManager != nil {
		return e.volumesManager, nil
	}

	adapter := &volumesManagerAdapter{
		Client: e.client,
		e:      e,
	}

	helperImage, err := e.getPrebuiltImage()
	if err != nil {
		return nil, err
	}

	ccManager := volumes.NewCacheContainerManager(
		e.Context,
		e.BuildLogger,
		adapter,
		helperImage,
		e.checkOutdatedHelperImage(),
	)

	config := volumes.ManagerConfig{
		CacheDir:          e.Config.Docker.CacheDir,
		BaseContainerPath: e.Build.FullProjectDir(),
		UniqName:          e.Build.ProjectUniqueName(),
		DisableCache:      e.Config.Docker.DisableCache,
	}

	e.volumesManager = volumes.NewManager(e.BuildLogger, ccManager, config)

	return e.volumesManager, nil
}

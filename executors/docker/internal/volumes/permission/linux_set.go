package permission

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/wait"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

const dstMount = "/gitlab-runner-cache-init"

type dockerLinuxSetter struct {
	client      docker.Client
	waiter      wait.Waiter
	logger      logrus.FieldLogger
	helperImage *types.ImageInspect
}

func NewDockerLinuxSetter(c docker.Client, logger logrus.FieldLogger, helperImage *types.ImageInspect) Setter {
	return &dockerLinuxSetter{
		client:      c,
		waiter:      wait.NewDockerKillWaiter(c),
		logger:      logger,
		helperImage: helperImage,
	}
}

// Set will take the specified volume, and change the OS
// permissions so that any user can read/write to it.
//
// By default when a volume is mounted to a container it has Unix permissions
// 755, so everyone can read from it but only root can write to it. This
// prevents images that don't have root user to fail to write to mounted
// volumes.
func (d *dockerLinuxSetter) Set(ctx context.Context, volumeName string, labels map[string]string) error {
	d.logger = d.logger.WithFields(logrus.Fields{
		"volume_name": volumeName,
		"context":     "set_volume_permission",
	})

	containerID, err := d.createContainer(ctx, volumeName, labels)
	if err != nil {
		return fmt.Errorf("create permission container for volume %q: %w", volumeName, err)
	}

	defer func() {
		removeErr := d.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
		if removeErr != nil {
			d.logger.WithError(removeErr).
				WithField("container_id", containerID).
				Debug("Failed to remove permission set container")
		}
	}()

	err = d.runContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("running permission container %q for volume %q: %w", containerID, volumeName, err)
	}

	return nil
}

func (d *dockerLinuxSetter) createContainer(
	ctx context.Context,
	volumeName string,
	labels map[string]string,
) (string, error) {
	volumeBinding := fmt.Sprintf("%s:%s", volumeName, dstMount)

	config := &container.Config{
		Image:  d.helperImage.ID,
		Cmd:    []string{"gitlab-runner-helper", "cache-init", dstMount},
		Labels: labels,
	}

	hostConfig := &container.HostConfig{
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
		Binds: []string{volumeBinding},
	}

	uuid, err := helpers.GenerateRandomUUID(8)
	if err != nil {
		return "", fmt.Errorf("generting uuid for permission container: %v", err)
	}

	containerName := fmt.Sprintf("%s-set-permission-%s", volumeName, uuid)
	c, err := d.client.ContainerCreate(ctx, config, hostConfig, nil, containerName)
	if err != nil {
		return "", err
	}
	d.logger.WithField("container_id", c.ID).Debug("Created container to set volume permissions")

	return c.ID, err
}

func (d *dockerLinuxSetter) runContainer(ctx context.Context, containerID string) error {
	err := d.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("starting permission container: %w", err)
	}

	err = d.waiter.Wait(ctx, containerID)
	if err != nil {
		return fmt.Errorf("waiting for permission container to finish: %w", err)
	}
	d.logger.WithField("container_id", containerID).Debug("Updated volume permissions")

	return nil
}

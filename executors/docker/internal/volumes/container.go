package volumes

import (
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type containerClient interface {
	LabelContainer(container *container.Config, containerType string, otherLabels ...string)
	CreateContainer(config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error)
	StartContainer(containerID string, options types.ContainerStartOptions) error
	InspectContainer(containerName string) (types.ContainerJSON, error)
	WaitForContainer(id string) error
	RemoveContainer(id string) error
}

type ContainerManager interface {
	FindExistingCacheContainer(containerName string, containerPath string) string
	CreateCacheContainer(containerName string, containerPath string) (string, error)
	FailedContainerIDs() []string
}

type containerManager struct {
	logger common.BuildLogger

	containerClient containerClient

	helperImage         *types.ImageInspect
	outdatedHelperImage bool
	failedContainerIDs  []string
}

func NewContainerManager(logger common.BuildLogger, cClient containerClient, helperImage *types.ImageInspect, outdatedHelperImage bool) ContainerManager {
	return &containerManager{
		logger:              logger,
		containerClient:     cClient,
		helperImage:         helperImage,
		outdatedHelperImage: outdatedHelperImage,
	}
}

func (m *containerManager) FindExistingCacheContainer(containerName string, containerPath string) string {
	inspected, err := m.containerClient.InspectContainer(containerName)
	if err != nil {
		return ""
	}

	// check if we have valid cache, if not remove the broken container
	_, ok := inspected.Config.Volumes[containerPath]
	if !ok {
		m.logger.Debugln(fmt.Sprintf("Removing broken cache container for %q path", containerPath))
		err = m.containerClient.RemoveContainer(inspected.ID)
		m.logger.Debugln(fmt.Sprintf("Cache container for %q path removed with %v", containerPath, err))

		return ""
	}

	return inspected.ID
}

func (m *containerManager) CreateCacheContainer(containerName string, containerPath string) (string, error) {
	containerID, err := m.createCacheContainer(containerName, containerPath)
	if err != nil {
		return "", err
	}

	err = m.startCacheContainer(containerID)
	if err != nil {
		return "", err
	}

	return containerID, nil
}

func (m *containerManager) createCacheContainer(containerName string, containerPath string) (string, error) {
	config := &container.Config{
		Image: m.helperImage.ID,
		Cmd:   m.getCacheCommand(containerPath),
		Volumes: map[string]struct{}{
			containerPath: {},
		},
	}
	m.containerClient.LabelContainer(config, "cache", "cache.dir="+containerPath)

	hostConfig := &container.HostConfig{
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
	}

	resp, err := m.containerClient.CreateContainer(config, hostConfig, nil, containerName)
	if err != nil {
		if resp.ID != "" {
			m.failedContainerIDs = append(m.failedContainerIDs, resp.ID)
		}

		return "", err
	}

	return resp.ID, nil
}

func (m *containerManager) getCacheCommand(containerPath string) []string {
	// TODO: Remove in 12.0 to start using the command from `gitlab-runner-helper`
	if m.outdatedHelperImage {
		m.logger.Debugln("Falling back to old gitlab-runner-cache command")
		return []string{"gitlab-runner-cache", containerPath}
	}

	return []string{"gitlab-runner-helper", "cache-init", containerPath}

}

func (m *containerManager) startCacheContainer(containerID string) error {
	m.logger.Debugln(fmt.Sprintf("Starting cache container %q...", containerID))
	err := m.containerClient.StartContainer(containerID, types.ContainerStartOptions{})
	if err != nil {
		m.failedContainerIDs = append(m.failedContainerIDs, containerID)

		return err
	}

	m.logger.Debugln(fmt.Sprintf("Waiting for cache container %q...", containerID))
	err = m.containerClient.WaitForContainer(containerID)
	if err != nil {
		m.failedContainerIDs = append(m.failedContainerIDs, containerID)

		return err
	}

	return nil
}

func (m *containerManager) FailedContainerIDs() []string {
	return m.failedContainerIDs
}

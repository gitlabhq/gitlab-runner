package volumes

import (
	"crypto/md5"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type containerManager interface {
	LabelContainer(container *container.Config, containerType string, otherLabels ...string)
	CreateContainer(config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error)
	StartContainer(containerID string, options types.ContainerStartOptions) error
	InspectContainer(containerName string) (types.ContainerJSON, error)
	WaitForContainer(id string) error
	RemoveContainer(id string) error
}

type helperImageResolver interface {
	ResolveHelperImage() (*types.ImageInspect, error)
}

type Manager interface {
	CreateUserVolumes(volumes []string) error
	CreateBuildVolume(volumes []string) error
	VolumeBindings() []string
	CacheContainerIDs() []string
	TmpContainerIDs() []string
}

type DefaultManagerConfig struct {
	CacheDir                string
	JobsRootDir             string
	FullProjectDir          string
	ProjectUniqName         string
	GitStrategy             common.GitStrategy
	DisableCache            bool
	OutdatedHelperImageUsed bool

	UseLegacyBuildsDirForDocker bool
}

type defaultManager struct {
	config DefaultManagerConfig

	logger common.BuildLogger

	containerManager    containerManager
	helperImageResolver helperImageResolver

	volumeBindings    []string
	cacheContainerIDs []string
	tmpContainerIDs   []string
}

func NewDefaultManager(logger common.BuildLogger, cManager containerManager, hiResolver helperImageResolver, config DefaultManagerConfig) Manager {
	return &defaultManager{
		config:              config,
		logger:              logger,
		containerManager:    cManager,
		helperImageResolver: hiResolver,
	}
}

func (m *defaultManager) CreateUserVolumes(volumes []string) error {
	for _, volume := range volumes {
		err := m.addVolume(volume)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *defaultManager) addVolume(volume string) error {
	hostVolume := strings.SplitN(volume, ":", 2)

	var err error
	switch len(hostVolume) {
	case 2:
		err = m.addHostVolume(hostVolume[0], hostVolume[1])
	case 1:
		// disable cache disables
		err = m.addCacheVolume(hostVolume[0])
	}

	if err != nil {
		m.logger.Errorln("Failed to create container volume for", volume, err)
	}

	return err
}

func (m *defaultManager) addHostVolume(hostPath string, containerPath string) error {
	containerPath = m.getAbsoluteContainerPath(containerPath)
	m.appendVolumeBind(hostPath, containerPath)

	return nil
}

func (m *defaultManager) getAbsoluteContainerPath(dir string) string {
	if path.IsAbs(dir) {
		return dir
	}

	return path.Join(m.config.FullProjectDir, dir)
}

func (m *defaultManager) appendVolumeBind(hostPath string, containerPath string) {
	m.logger.Debugln(fmt.Sprintf("Using host-based %q for %q...", hostPath, containerPath))

	bindDefinition := fmt.Sprintf("%v:%v", filepath.ToSlash(hostPath), containerPath)
	m.volumeBindings = append(m.volumeBindings, bindDefinition)
}

func (m *defaultManager) addCacheVolume(containerPath string) error {
	containerPath = m.getAbsoluteContainerPath(containerPath)

	// disable cache for automatic container cache,
	// but leave it for host volumes (they are shared on purpose)
	if m.config.DisableCache {
		m.logger.Debugln(fmt.Sprintf("Container cache for %q is disabled", containerPath))

		return nil
	}

	hash := md5.Sum([]byte(containerPath))
	if m.config.CacheDir != "" {
		return m.createHostBasedCacheVolume(containerPath, hash)
	}

	return m.createContainerBasedCacheVolume(containerPath, hash)
}

func (m *defaultManager) createHostBasedCacheVolume(containerPath string, hash [md5.Size]byte) error {
	hostPath := fmt.Sprintf("%s/%s/%x", m.config.CacheDir, m.config.ProjectUniqName, hash)
	hostPath, err := filepath.Abs(hostPath)
	if err != nil {
		return err
	}

	m.appendVolumeBind(hostPath, containerPath)

	return nil
}

func (m *defaultManager) createContainerBasedCacheVolume(containerPath string, hash [md5.Size]byte) error {
	containerName := fmt.Sprintf("%s-cache-%x", m.config.ProjectUniqName, hash)

	containerID := m.findExistingCacheContainer(containerName, containerPath)

	// create new cache container for that project
	if containerID == "" {
		var err error

		containerID, err = m.createCacheVolume(containerName, containerPath)
		if err != nil {
			return err
		}
	}

	m.logger.Debugln(fmt.Sprintf("Using container %q as cache %q...", containerID, containerPath))
	m.cacheContainerIDs = append(m.cacheContainerIDs, containerID)

	return nil
}

func (m *defaultManager) findExistingCacheContainer(containerName string, containerPath string) string {
	inspected, err := m.containerManager.InspectContainer(containerName)
	if err != nil {
		return ""
	}

	// check if we have valid cache,if not remove the broken container
	_, ok := inspected.Config.Volumes[containerPath]
	if !ok {
		m.logger.Debugln(fmt.Sprintf("Removing broken cache container for %q path", containerPath))
		err = m.containerManager.RemoveContainer(inspected.ID)
		m.logger.Debugln(fmt.Sprintf("Cache container for %q path removed with %v", containerPath, err))

		return ""
	}

	return inspected.ID
}

func (m *defaultManager) createCacheVolume(containerName string, containerPath string) (string, error) {
	cacheImage, err := m.helperImageResolver.ResolveHelperImage()
	if err != nil {
		return "", err
	}

	config := &container.Config{
		Image: cacheImage.ID,
		Cmd:   m.getCacheCommand(containerPath),
		Volumes: map[string]struct{}{
			containerPath: {},
		},
	}
	m.containerManager.LabelContainer(config, "cache", "cache.dir="+containerPath)

	hostConfig := &container.HostConfig{
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
	}

	resp, err := m.containerManager.CreateContainer(config, hostConfig, nil, containerName)
	if err != nil {
		if resp.ID != "" {
			m.tmpContainerIDs = append(m.tmpContainerIDs, resp.ID)
		}

		return "", err
	}

	m.logger.Debugln(fmt.Sprintf("Starting cache container %q...", resp.ID))
	err = m.containerManager.StartContainer(resp.ID, types.ContainerStartOptions{})
	if err != nil {
		m.tmpContainerIDs = append(m.tmpContainerIDs, resp.ID)

		return "", err
	}

	m.logger.Debugln(fmt.Sprintf("Waiting for cache container %q...", resp.ID))
	err = m.containerManager.WaitForContainer(resp.ID)
	if err != nil {
		m.tmpContainerIDs = append(m.tmpContainerIDs, resp.ID)

		return "", err
	}

	return resp.ID, nil
}

func (m *defaultManager) getCacheCommand(containerPath string) []string {
	// TODO: Remove in 12.0 to start using the command from `gitlab-runner-helper`
	if m.config.OutdatedHelperImageUsed {
		m.logger.Debugln("Falling back to old gitlab-runner-cache command")
		return []string{"gitlab-runner-cache", containerPath}
	}

	return []string{"gitlab-runner-helper", "cache-init", containerPath}
}

func (m *defaultManager) CreateBuildVolume(volumes []string) error {
	parentDir := m.config.JobsRootDir

	if m.config.UseLegacyBuildsDirForDocker {
		// Cache Git sources:
		// take path of the projects directory,
		// because we use `rm -rf` which could remove the mounted volume
		parentDir = path.Dir(m.config.FullProjectDir)
	}

	if !path.IsAbs(parentDir) && parentDir != "/" {
		return common.MakeBuildError("build directory needs to be absolute and non-root path")
	}

	if IsHostMountedVolume(parentDir, volumes...) {
		// If builds directory is within a volume mounted manually by user
		// it will be added by CreateUserVolumes(), so nothing more to do
		// here
		return nil
	}

	if m.config.GitStrategy == common.GitFetch && !m.config.DisableCache {
		// create persistent cache container
		return m.addVolume(parentDir)
	}

	// create temporary cache container
	id, err := m.createCacheVolume("", parentDir)
	if err != nil {
		return err
	}

	m.cacheContainerIDs = append(m.cacheContainerIDs, id)
	m.tmpContainerIDs = append(m.tmpContainerIDs, id)

	return nil
}

func (m *defaultManager) VolumeBindings() []string {
	return m.volumeBindings
}

func (m *defaultManager) CacheContainerIDs() []string {
	return m.cacheContainerIDs
}

func (m *defaultManager) TmpContainerIDs() []string {
	return m.tmpContainerIDs
}

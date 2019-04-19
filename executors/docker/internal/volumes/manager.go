package volumes

import (
	"crypto/md5"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type Manager interface {
	AddVolume(volume string) error
	CreateBuildVolume(jobsRootDir string, volumes []string) error
	VolumeBindings() []string
	CacheContainerIDs() []string
	TmpContainerIDs() []string
}

type ManagerConfig struct {
	CacheDir        string
	FullProjectDir  string
	ProjectUniqName string
	GitStrategy     common.GitStrategy
	DisableCache    bool

	UseLegacyBuildsDirForDocker bool
}

type manager struct {
	config ManagerConfig

	logger           common.BuildLogger
	containerManager ContainerManager

	volumeBindings    []string
	cacheContainerIDs []string
	tmpContainerIDs   []string
}

func NewManager(logger common.BuildLogger, cManager ContainerManager, config ManagerConfig) Manager {
	return &manager{
		config:            config,
		logger:            logger,
		containerManager:  cManager,
		volumeBindings:    make([]string, 0),
		cacheContainerIDs: make([]string, 0),
		tmpContainerIDs:   make([]string, 0),
	}
}

func (m *manager) AddVolume(volume string) error {
	if len(volume) < 1 {
		return nil
	}

	hostVolume := strings.SplitN(volume, ":", 2)

	var err error
	switch len(hostVolume) {
	case 2:
		err = m.addHostVolume(hostVolume[0], hostVolume[1])
	case 1:
		err = m.addCacheVolume(hostVolume[0])
	}

	if err != nil {
		m.logger.Errorln("Failed to create container volume for", volume, err)
	}

	return err
}

func (m *manager) addHostVolume(hostPath string, containerPath string) error {
	containerPath = m.getAbsoluteContainerPath(containerPath)
	m.appendVolumeBind(hostPath, containerPath)

	return nil
}

func (m *manager) getAbsoluteContainerPath(dir string) string {
	if path.IsAbs(dir) {
		return dir
	}

	return path.Join(m.config.FullProjectDir, dir)
}

func (m *manager) appendVolumeBind(hostPath string, containerPath string) {
	m.logger.Debugln(fmt.Sprintf("Using host-based %q for %q...", hostPath, containerPath))

	bindDefinition := fmt.Sprintf("%v:%v", filepath.ToSlash(hostPath), containerPath)
	m.volumeBindings = append(m.volumeBindings, bindDefinition)
}

func (m *manager) addCacheVolume(containerPath string) error {
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

func (m *manager) createHostBasedCacheVolume(containerPath string, hash [md5.Size]byte) error {
	hostPath := fmt.Sprintf("%s/%s/%x", m.config.CacheDir, m.config.ProjectUniqName, hash)
	hostPath, err := filepath.Abs(hostPath)
	if err != nil {
		return err
	}

	m.appendVolumeBind(hostPath, containerPath)

	return nil
}

func (m *manager) createContainerBasedCacheVolume(containerPath string, hash [md5.Size]byte) error {
	containerName := fmt.Sprintf("%s-cache-%x", m.config.ProjectUniqName, hash)

	containerID := m.containerManager.FindExistingCacheContainer(containerName, containerPath)

	// create new cache container for that project
	if containerID == "" {
		var err error

		containerID, err = m.containerManager.CreateCacheContainer(containerName, containerPath)
		if err != nil {
			return err
		}
	}

	m.logger.Debugln(fmt.Sprintf("Using container %q as cache %q...", containerID, containerPath))
	m.cacheContainerIDs = append(m.cacheContainerIDs, containerID)

	return nil
}

func (m *manager) CreateBuildVolume(jobsRootDir string, volumes []string) error {
	parentDir := jobsRootDir

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
		return m.AddVolume(parentDir)
	}

	// create temporary cache container
	id, err := m.containerManager.CreateCacheContainer("", parentDir)
	if err != nil {
		return err
	}

	m.cacheContainerIDs = append(m.cacheContainerIDs, id)
	m.tmpContainerIDs = append(m.tmpContainerIDs, id)

	return nil
}

func (m *manager) VolumeBindings() []string {
	return m.volumeBindings
}

func (m *manager) CacheContainerIDs() []string {
	return m.cacheContainerIDs
}

func (m *manager) TmpContainerIDs() []string {
	return append(m.tmpContainerIDs, m.containerManager.FailedContainerIDs()...)
}

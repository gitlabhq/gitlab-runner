package volumes

import (
	"context"
	"crypto/md5"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type ErrVolumeAlreadyDefined struct {
	containerPath string
}

func (e *ErrVolumeAlreadyDefined) Error() string {
	return fmt.Sprintf("volume for container path %q is already defined", e.containerPath)
}

func NewErrVolumeAlreadyDefined(containerPath string) *ErrVolumeAlreadyDefined {
	return &ErrVolumeAlreadyDefined{
		containerPath: containerPath,
	}
}

type Manager interface {
	Create(volume string) error
	CreateBuildVolume(jobsRootDir string, volumes []string) error
	Binds() []string
	ContainerIDs() []string
	Cleanup(ctx context.Context) chan bool
}

type ManagerConfig struct {
	CacheDir        string
	FullProjectDir  string
	ProjectUniqName string
	GitStrategy     common.GitStrategy
	DisableCache    bool
}

type manager struct {
	config ManagerConfig

	logger           common.BuildLogger
	containerManager ContainerManager

	volumeBindings    []string
	cacheContainerIDs []string
	tmpContainerIDs   []string

	managedVolumes map[string]bool
}

func NewManager(logger common.BuildLogger, cManager ContainerManager, config ManagerConfig) Manager {
	return &manager{
		config:            config,
		logger:            logger,
		containerManager:  cManager,
		volumeBindings:    make([]string, 0),
		cacheContainerIDs: make([]string, 0),
		tmpContainerIDs:   make([]string, 0),
		managedVolumes:    make(map[string]bool, 0),
	}
}

func (m *manager) Create(volume string) error {
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

	err := m.rememberVolume(containerPath)
	if err != nil {
		return err
	}

	m.appendVolumeBind(hostPath, containerPath)

	return nil
}

func (m *manager) getAbsoluteContainerPath(dir string) string {
	if path.IsAbs(dir) {
		return dir
	}

	return path.Join(m.config.FullProjectDir, dir)
}

func (m *manager) rememberVolume(containerPath string) error {
	if m.managedVolumes[containerPath] {
		return NewErrVolumeAlreadyDefined(containerPath)
	}

	m.managedVolumes[containerPath] = true

	return nil
}

func (m *manager) appendVolumeBind(hostPath string, containerPath string) {
	m.logger.Debugln(fmt.Sprintf("Using host-based %q for %q...", hostPath, containerPath))

	bindDefinition := fmt.Sprintf("%v:%v", filepath.ToSlash(hostPath), containerPath)
	m.volumeBindings = append(m.volumeBindings, bindDefinition)
}

func (m *manager) addCacheVolume(containerPath string) error {
	containerPath = m.getAbsoluteContainerPath(containerPath)

	err := m.rememberVolume(containerPath)
	if err != nil {
		return err
	}

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
	if IsHostMountedVolume(jobsRootDir, volumes...) {
		// If builds directory is within a volume mounted manually by user
		// it will be added by CreateUserVolumes(), so nothing more to do
		// here
		return nil
	}

	if m.config.GitStrategy == common.GitFetch && !m.config.DisableCache {
		// create persistent cache container
		return m.Create(jobsRootDir)
	}

	// create temporary cache container
	id, err := m.containerManager.CreateCacheContainer("", jobsRootDir)
	if err != nil {
		return err
	}

	m.cacheContainerIDs = append(m.cacheContainerIDs, id)
	m.tmpContainerIDs = append(m.tmpContainerIDs, id)

	return nil
}

func (m *manager) Binds() []string {
	return m.volumeBindings
}

func (m *manager) ContainerIDs() []string {
	return m.cacheContainerIDs
}

func (m *manager) Cleanup(ctx context.Context) chan bool {
	done := make(chan bool, 1)

	remove := func(wg *sync.WaitGroup, containerID string) {
		wg.Add(1)
		go func() {
			_ = m.containerManager.RemoveCacheContainer(ctx, containerID)
			wg.Done()
		}()
	}

	go func() {
		wg := new(sync.WaitGroup)
		for _, id := range m.tmpContainerIDs {
			remove(wg, id)
		}

		wg.Wait()
		done <- true
	}()

	return done
}

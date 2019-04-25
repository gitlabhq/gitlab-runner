package volumes

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

var ErrCacheVolumesDisabled = errors.New("cache volumes feature disabled")

type Manager interface {
	Create(volume string) error
	CreateTemporary(containerPath string) error
	Binds() []string
	ContainerIDs() []string
	Cleanup(ctx context.Context) chan bool
}

type ManagerConfig struct {
	CacheDir          string
	BaseContainerPath string
	UniqName          string
	DisableCache      bool
}

type manager struct {
	config ManagerConfig
	logger debugLogger

	cacheContainersManager CacheContainersManager

	volumeBindings    []string
	cacheContainerIDs []string
	tmpContainerIDs   []string

	managedVolumes pathList
}

func NewManager(logger debugLogger, ccManager CacheContainersManager, config ManagerConfig) Manager {
	return &manager{
		config:                 config,
		logger:                 logger,
		cacheContainersManager: ccManager,
		volumeBindings:         make([]string, 0),
		cacheContainerIDs:      make([]string, 0),
		tmpContainerIDs:        make([]string, 0),
		managedVolumes:         pathList{},
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

	return err
}

func (m *manager) addHostVolume(hostPath string, containerPath string) error {
	containerPath = m.getAbsoluteContainerPath(containerPath)

	err := m.managedVolumes.Add(containerPath)
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

	return path.Join(m.config.BaseContainerPath, dir)
}

func (m *manager) appendVolumeBind(hostPath string, containerPath string) {
	m.logger.Debugln(fmt.Sprintf("Using host-based %q for %q...", hostPath, containerPath))

	bindDefinition := fmt.Sprintf("%v:%v", filepath.ToSlash(hostPath), containerPath)
	m.volumeBindings = append(m.volumeBindings, bindDefinition)
}

func (m *manager) addCacheVolume(containerPath string) error {
	// disable cache for automatic container cache,
	// but leave it for host volumes (they are shared on purpose)
	if m.config.DisableCache {
		m.logger.Debugln("Cache containers feature is disabled")

		return ErrCacheVolumesDisabled
	}

	if m.config.CacheDir != "" {
		return m.createHostBasedCacheVolume(containerPath)
	}

	_, err := m.createContainerBasedCacheVolume(containerPath)

	return err
}

func (m *manager) createHostBasedCacheVolume(containerPath string) error {
	containerPath = m.getAbsoluteContainerPath(containerPath)

	err := m.managedVolumes.Add(containerPath)
	if err != nil {
		return err
	}

	hostPath := fmt.Sprintf("%s/%s/%s", m.config.CacheDir, m.config.UniqName, hashContainerPath(containerPath))
	hostPath, err = filepath.Abs(hostPath)
	if err != nil {
		return err
	}

	m.appendVolumeBind(hostPath, containerPath)

	return nil
}

func (m *manager) createContainerBasedCacheVolume(containerPath string) (string, error) {
	containerPath = m.getAbsoluteContainerPath(containerPath)

	err := m.managedVolumes.Add(containerPath)
	if err != nil {
		return "", err
	}

	containerName := fmt.Sprintf("%s-cache-%s", m.config.UniqName, hashContainerPath(containerPath))
	containerID := m.cacheContainersManager.FindOrCleanExisting(containerName, containerPath)

	// create new cache container for that project
	if containerID == "" {
		var err error

		containerID, err = m.cacheContainersManager.Create(containerName, containerPath)
		if err != nil {
			return "", err
		}
	}

	m.logger.Debugln(fmt.Sprintf("Using container %q as cache %q...", containerID, containerPath))
	m.cacheContainerIDs = append(m.cacheContainerIDs, containerID)

	return containerID, nil
}

func (m *manager) CreateTemporary(containerPath string) error {
	id, err := m.createContainerBasedCacheVolume(containerPath)
	if err != nil {
		return err
	}

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
	return m.cacheContainersManager.Cleanup(ctx, m.tmpContainerIDs)
}

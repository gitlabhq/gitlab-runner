package volumes

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
)

var ErrCacheVolumesDisabled = errors.New("cache volumes feature disabled")

type parserProvider interface {
	CreateParser() (parser.Parser, error)
}

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
	UniqueName        string
	DisableCache      bool
}

type manager struct {
	config ManagerConfig
	logger debugLogger

	cacheContainersManager CacheContainersManager
	parserProvider         parserProvider

	volumeBindings    []string
	cacheContainerIDs []string
	tmpContainerIDs   []string

	managedVolumes pathList
}

func NewManager(logger debugLogger, pProvider parserProvider, ccManager CacheContainersManager, config ManagerConfig) Manager {
	return &manager{
		config:                 config,
		logger:                 logger,
		parserProvider:         pProvider,
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

	volumeParser, err := m.parserProvider.CreateParser()
	if err != nil {
		return err
	}

	parsedVolume, err := volumeParser.ParseVolume(volume)
	if err != nil {
		return err
	}

	switch parsedVolume.Len() {
	case 2:
		err = m.addHostVolume(parsedVolume)
	case 1:
		err = m.addCacheVolume(parsedVolume)
	}

	return err
}

func (m *manager) addHostVolume(volume *parser.Volume) error {
	volume.Destination = m.getAbsoluteContainerPath(volume.Destination)

	err := m.managedVolumes.Add(volume.Destination)
	if err != nil {
		return err
	}

	m.appendVolumeBind(volume)

	return nil
}

func (m *manager) getAbsoluteContainerPath(dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}

	return filepath.Join(m.config.BaseContainerPath, dir)
}

func (m *manager) appendVolumeBind(volume *parser.Volume) {
	m.logger.Debugln(fmt.Sprintf("Using host-based %q for %q...", volume.Source, volume.Destination))
	m.volumeBindings = append(m.volumeBindings, volume.Definition())
}

func (m *manager) addCacheVolume(volume *parser.Volume) error {
	// disable cache for automatic container cache,
	// but leave it for host volumes (they are shared on purpose)
	if m.config.DisableCache {
		m.logger.Debugln("Cache containers feature is disabled")

		return ErrCacheVolumesDisabled
	}

	if m.config.CacheDir != "" {
		return m.createHostBasedCacheVolume(volume.Destination)
	}

	_, err := m.createContainerBasedCacheVolume(volume.Destination)

	return err
}

func (m *manager) createHostBasedCacheVolume(containerPath string) error {
	containerPath = m.getAbsoluteContainerPath(containerPath)

	err := m.managedVolumes.Add(containerPath)
	if err != nil {
		return err
	}

	hostPath := filepath.Join(m.config.CacheDir, m.config.UniqueName, hashContainerPath(containerPath))
	hostPath, err = filepath.Abs(hostPath)
	if err != nil {
		return err
	}

	m.appendVolumeBind(&parser.Volume{
		Source:      hostPath,
		Destination: containerPath,
	})

	return nil
}

func (m *manager) createContainerBasedCacheVolume(containerPath string) (string, error) {
	containerPath = m.getAbsoluteContainerPath(containerPath)

	err := m.managedVolumes.Add(containerPath)
	if err != nil {
		return "", err
	}

	containerName := fmt.Sprintf("%s-cache-%s", m.config.UniqueName, hashContainerPath(containerPath))
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

package volumes

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/api/types/volume"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

var ErrCacheVolumesDisabled = errors.New("cache volumes feature disabled")

type Manager interface {
	Create(ctx context.Context, volume string) error
	CreateTemporary(ctx context.Context, destination string) error
	Binds() []string
}

type ManagerConfig struct {
	CacheDir     string
	BasePath     string
	UniqueName   string
	DisableCache bool
}

type manager struct {
	config ManagerConfig
	logger debugLogger
	parser parser.Parser
	client docker.Client

	volumeBindings []string
	managedVolumes pathList
}

func NewManager(logger debugLogger, volumeParser parser.Parser, c docker.Client, config ManagerConfig) Manager {
	return &manager{
		config:         config,
		logger:         logger,
		parser:         volumeParser,
		client:         c,
		volumeBindings: make([]string, 0),
		managedVolumes: pathList{},
	}
}

func (m *manager) Create(ctx context.Context, volume string) error {
	if len(volume) < 1 {
		return nil
	}

	parsedVolume, err := m.parser.ParseVolume(volume)
	if err != nil {
		return fmt.Errorf("parse volume: %w", err)
	}

	switch parsedVolume.Len() {
	case 2:
		err = m.addHostVolume(parsedVolume)
		if err != nil {
			err = fmt.Errorf("adding host volume: %w", err)
		}
	case 1:
		err = m.addCacheVolume(ctx, parsedVolume)
		if err != nil {
			err = fmt.Errorf("adding cache volume: %w", err)
		}
	default:
		err = fmt.Errorf("unsupported volume definition %s", volume)
	}

	return err
}

func (m *manager) addHostVolume(volume *parser.Volume) error {
	var err error

	volume.Destination, err = m.absolutePath(volume.Destination)
	if err != nil {
		return fmt.Errorf("defining absolute path: %w", err)
	}

	err = m.managedVolumes.Add(volume.Destination)
	if err != nil {
		return fmt.Errorf("updating managed volume list: %w", err)
	}

	m.appendVolumeBind(volume)

	return nil
}

func (m *manager) absolutePath(dir string) (string, error) {
	if m.parser.Path().IsRoot(dir) {
		return "", errDirectoryIsRootPath
	}

	if m.parser.Path().IsAbs(dir) {
		return dir, nil
	}

	return m.parser.Path().Join(m.config.BasePath, dir), nil
}

func (m *manager) appendVolumeBind(volume *parser.Volume) {
	m.logger.Debugln(fmt.Sprintf("Using host-based %q for %q...", volume.Source, volume.Destination))

	m.volumeBindings = append(m.volumeBindings, volume.Definition())
}

func (m *manager) addCacheVolume(ctx context.Context, volume *parser.Volume) error {
	// disable cache for automatic container cache,
	// but leave it for host volumes (they are shared on purpose)
	if m.config.DisableCache {
		m.logger.Debugln("Cache containers feature is disabled")

		return ErrCacheVolumesDisabled
	}

	if m.config.CacheDir != "" {
		return m.createHostBasedCacheVolume(volume.Destination)
	}

	return m.createCacheVolume(ctx, volume.Destination)
}

func (m *manager) createHostBasedCacheVolume(destination string) error {
	destination, err := m.absolutePath(destination)
	if err != nil {
		return fmt.Errorf("defining absolute path: %w", err)
	}

	err = m.managedVolumes.Add(destination)
	if err != nil {
		return fmt.Errorf("updating managed volumes list: %w", err)
	}

	hostPath := m.parser.Path().Join(m.config.CacheDir, m.config.UniqueName, hashPath(destination))

	m.appendVolumeBind(&parser.Volume{
		Source:      hostPath,
		Destination: destination,
	})

	return nil
}

func (m *manager) createCacheVolume(ctx context.Context, destination string) error {
	destination, err := m.absolutePath(destination)
	if err != nil {
		return fmt.Errorf("defining absolute path:%w", err)
	}

	err = m.managedVolumes.Add(destination)
	if err != nil {
		return fmt.Errorf("updating managed volumes list: %w", err)
	}

	volumeName := fmt.Sprintf("%s-cache-%s", m.config.UniqueName, hashPath(destination))
	vBody := volume.VolumeCreateBody{
		Name: volumeName,
	}

	v, err := m.client.VolumeCreate(ctx, vBody)
	if err != nil {
		return fmt.Errorf("creating docker volume: %w", err)
	}

	m.appendVolumeBind(&parser.Volume{
		Source:      v.Name,
		Destination: destination,
	})
	m.logger.Debugln(fmt.Sprintf("Using volume %q as cache %q...", v.Name, destination))

	return nil
}

func (m *manager) CreateTemporary(ctx context.Context, destination string) error {
	err := m.createCacheVolume(ctx, destination)
	if err != nil {
		return fmt.Errorf("creating cache volume: %w", err)
	}

	return nil
}

func (m *manager) Binds() []string {
	return m.volumeBindings
}

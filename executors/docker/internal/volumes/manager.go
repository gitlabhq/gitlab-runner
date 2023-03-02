package volumes

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/api/types/volume"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/permission"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

var ErrCacheVolumesDisabled = errors.New("cache volumes feature disabled")

//go:generate mockery --name=Manager --inpackage
type Manager interface {
	Create(ctx context.Context, volume string) error
	CreateTemporary(ctx context.Context, destination string) error
	RemoveTemporary(ctx context.Context) error
	Binds() []string
}

type ManagerConfig struct {
	CacheDir         string
	BasePath         string
	UniqueName       string
	TemporaryName    string
	DisableCache     bool
	PermissionSetter permission.Setter
	DriverOpts       map[string]string
}

type manager struct {
	config           ManagerConfig
	logger           debugLogger
	parser           parser.Parser
	client           docker.Client
	permissionSetter permission.Setter
	labeler          labels.Labeler

	volumeBindings   []string
	temporaryVolumes []string
	managedVolumes   pathList
}

func NewManager(
	logger debugLogger,
	volumeParser parser.Parser,
	c docker.Client,
	config ManagerConfig,
	labeler labels.Labeler,
) Manager {
	return &manager{
		config:           config,
		logger:           logger,
		parser:           volumeParser,
		client:           c,
		volumeBindings:   make([]string, 0),
		managedVolumes:   pathList{},
		permissionSetter: config.PermissionSetter,
		labeler:          labeler,
	}
}

// Create will create a new Docker volume bind for the specified volume. The
// volume can either be a host volume `/src:/dst`, meaning it will mount
// something from the host to the container or `/dst` which will create a Docker
// volume and mount it to the specified path.
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

	_, err := m.createCacheVolume(ctx, volume.Destination, true, m.config.DriverOpts)

	return err
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

func (m *manager) createCacheVolume(
	ctx context.Context,
	destination string,
	reusable bool,
	driverOps map[string]string,
) (string, error) {
	destination, err := m.absolutePath(destination)
	if err != nil {
		return "", fmt.Errorf("defining absolute path: %w", err)
	}

	err = m.managedVolumes.Add(destination)
	if err != nil {
		return "", fmt.Errorf("updating managed volumes list: %w", err)
	}

	name := m.config.TemporaryName
	if reusable {
		name = m.config.UniqueName
	}

	volumeName := fmt.Sprintf("%s-cache-%s", name, hashPath(destination))
	vBody := volume.CreateOptions{
		Name:       volumeName,
		DriverOpts: driverOps,
		Labels:     m.labeler.Labels(map[string]string{"type": "cache"}),
	}

	v, err := m.client.VolumeCreate(ctx, vBody)
	if err != nil {
		return "", fmt.Errorf("creating docker volume: %w", err)
	}

	if m.permissionSetter != nil {
		err = m.permissionSetter.Set(ctx, v.Name, m.labeler.Labels(map[string]string{"type": "cache-init"}))
		if err != nil {
			return "", fmt.Errorf("set volume permissions: %w", err)
		}
	}

	m.appendVolumeBind(&parser.Volume{
		Source:      v.Name,
		Destination: destination,
	})
	m.logger.Debugln(fmt.Sprintf("Using volume %q as cache %q...", v.Name, destination))

	return volumeName, nil
}

// CreateTemporary will create a volume, and mark it as temporary. When a volume
// is marked as temporary it means that it should be cleaned up at some point.
// It's up to the caller to clean up the temporary volumes by calling
// `RemoveTemporary`.
func (m *manager) CreateTemporary(ctx context.Context, destination string) error {
	volumeName, err := m.createCacheVolume(ctx, destination, false, m.config.DriverOpts)
	if err != nil {
		return fmt.Errorf("creating cache volume: %w", err)
	}

	m.temporaryVolumes = append(m.temporaryVolumes, volumeName)

	return nil
}

// RemoveTemporary will remove all the volumes that are marked as temporary. If
// the volume is not found the error is ignored, any other error is returned to
// the caller.
func (m *manager) RemoveTemporary(ctx context.Context) error {
	for _, v := range m.temporaryVolumes {
		err := m.client.VolumeRemove(ctx, v, true)
		if docker.IsErrNotFound(err) {
			m.logger.Debugln(fmt.Sprintf("volume not found: %q", v))
			continue
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// Binds returns all the bindings that the volume manager is aware of.
func (m *manager) Binds() []string {
	return m.volumeBindings
}

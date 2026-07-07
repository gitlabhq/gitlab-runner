package networks

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/moby/moby/api/types/container"
	network "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

var errBuildNetworkExists = errors.New("build network is not empty")

type Manager interface {
	Create(ctx context.Context, networkMode string, enableIPv6 bool) (container.NetworkMode, error)
	Inspect(ctx context.Context) (network.Inspect, error)
	Cleanup(ctx context.Context) error
	// Adopt takes ownership of an existing network from a previously suspended
	// build. User-defined networks are inspected to verify they still exist;
	// built-in modes (bridge, none, default, etc.) are recorded as-is.
	Adopt(ctx context.Context, networkMode container.NetworkMode) error
}

type manager struct {
	logger  debugLogger
	client  docker.Client
	build   *common.Build
	labeler labels.Labeler

	networkMode  container.NetworkMode
	buildNetwork network.Inspect
	perBuild     bool
}

func NewManager(logger debugLogger, dockerClient docker.Client, build *common.Build, labeler labels.Labeler) Manager {
	return &manager{
		logger:  logger,
		client:  dockerClient,
		build:   build,
		labeler: labeler,
	}
}

func (m *manager) Create(ctx context.Context, networkMode string, enableIPv6 bool) (container.NetworkMode, error) {
	m.networkMode = container.NetworkMode(networkMode)
	m.perBuild = false

	if networkMode != "" {
		return m.networkMode, nil
	}

	if !m.build.IsFeatureFlagOn(featureflags.NetworkPerBuild) {
		return m.networkMode, nil
	}

	if m.buildNetwork.ID != "" {
		return "", errBuildNetworkExists
	}

	networkName := m.build.GetNetworkName()

	m.logger.Debugln("Creating build network ", networkName)

	networkResponse, err := m.client.NetworkCreate(
		ctx,
		networkName,
		client.NetworkCreateOptions{
			Labels:     m.labeler.Labels(map[string]string{}),
			EnableIPv6: &enableIPv6,
			Options:    networkOptionsFromConfig(m.build.Runner.Docker),
		},
	)
	if err != nil {
		return "", err
	}

	// Inspect the created network to save its details
	m.buildNetwork, err = m.client.NetworkInspect(ctx, networkResponse.ID)
	if err != nil {
		return "", err
	}

	m.networkMode = container.NetworkMode(networkName)
	m.perBuild = true

	return m.networkMode, nil
}

func networkOptionsFromConfig(config *common.DockerConfig) map[string]string {
	networkOptions := make(map[string]string)
	if config != nil && config.NetworkMTU != 0 {
		networkOptions["com.docker.network.driver.mtu"] = strconv.Itoa(config.NetworkMTU)
	}

	return networkOptions
}

func (m *manager) Inspect(ctx context.Context) (network.Inspect, error) {
	if !m.perBuild {
		return network.Inspect{}, nil
	}

	m.logger.Debugln("Inspect docker network: ", m.buildNetwork.ID)

	return m.client.NetworkInspect(ctx, m.buildNetwork.ID)
}

func (m *manager) Cleanup(ctx context.Context) error {
	if !m.build.IsFeatureFlagOn(featureflags.NetworkPerBuild) {
		return nil
	}

	if !m.perBuild {
		return nil
	}

	m.logger.Debugln("Removing network: ", m.buildNetwork.ID)

	err := m.client.NetworkRemove(ctx, m.buildNetwork.ID)
	if err != nil {
		return fmt.Errorf("docker remove network %s: %w", m.buildNetwork.ID, err)
	}

	return nil
}

func (m *manager) Adopt(ctx context.Context, mode container.NetworkMode) error {
	// On Windows, IsHost() always returns false and IsUserDefined() does
	// not exclude "host". In practice, Windows containers cannot use host networking.
	if !mode.IsUserDefined() {
		m.networkMode = mode
		m.perBuild = false
		return nil
	}

	inspect, err := m.client.NetworkInspect(ctx, string(mode))
	if err != nil {
		return fmt.Errorf("build network %s not found: %w", mode, err)
	}
	m.buildNetwork = inspect
	m.networkMode = mode
	m.perBuild = true
	return nil
}

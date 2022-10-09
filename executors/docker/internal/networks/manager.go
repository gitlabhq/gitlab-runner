package networks

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

var errBuildNetworkExists = errors.New("build network is not empty")

//go:generate mockery --name=Manager --inpackage
type Manager interface {
	Create(ctx context.Context, networkMode string, enableIPv6 bool) (container.NetworkMode, error)
	Inspect(ctx context.Context) (types.NetworkResource, error)
	Cleanup(ctx context.Context) error
}

type manager struct {
	logger  debugLogger
	client  docker.Client
	build   *common.Build
	labeler labels.Labeler

	networkMode  container.NetworkMode
	buildNetwork types.NetworkResource
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

	networkName := fmt.Sprintf("%s-job-%d-network", m.build.ProjectUniqueName(), m.build.ID)

	m.logger.Debugln("Creating build network ", networkName)

	networkResponse, err := m.client.NetworkCreate(
		ctx,
		networkName,
		types.NetworkCreate{
			Labels:     m.labeler.Labels(map[string]string{}),
			EnableIPv6: enableIPv6,
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

func (m *manager) Inspect(ctx context.Context) (types.NetworkResource, error) {
	if !m.perBuild {
		return types.NetworkResource{}, nil
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

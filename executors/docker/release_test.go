//go:build !integration

package docker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"
	autoscaler "gitlab.com/gitlab-org/gitlab-runner/executors/internal/autoscaler"
	docker "gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

// containerListAllTrue matches ContainerListOptions where All is true.
var containerListAllTrue = mock.MatchedBy(func(opts client.ContainerListOptions) bool {
	return opts.All
})

func TestReleaseByLabel_EmptyKey(t *testing.T) {
	c := docker.NewMockClient(t)
	// No list calls should be made.
	err := releaseByLabel(t.Context(), c, 1, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acquisition key must not be empty")
}

func TestReleaseByLabel_ZeroRunnerID(t *testing.T) {
	c := docker.NewMockClient(t)
	// No list calls should be made.
	err := releaseByLabel(t.Context(), c, 0, "some-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "runner ID must be positive")
}

func TestReleaseByLabel_NegativeRunnerID(t *testing.T) {
	c := docker.NewMockClient(t)
	err := releaseByLabel(t.Context(), c, -5, "some-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "runner ID must be positive")
}

func TestReleaseByLabel_ContainerListError_Fatal(t *testing.T) {
	c := docker.NewMockClient(t)
	listErr := errors.New("daemon unreachable")

	// ContainerList fails — neither NetworkList nor VolumeList may be called.
	c.On("ContainerList", mock.Anything, containerListAllTrue).Return(nil, listErr).Once()

	err := releaseByLabel(t.Context(), c, 1, "acq-key")
	require.ErrorContains(t, err, "list containers for release")
	require.ErrorIs(t, err, listErr)
}

// Every list call must filter by both the acquisition key and the runner ID,
// so a runner manager never removes another runner's environment.
func TestReleaseByLabel_FiltersByAcquisitionKeyAndRunnerID(t *testing.T) {
	c := docker.NewMockClient(t)
	ours := func(f client.Filters) bool {
		return f["label"][labels.AcquisitionKeyLabel+"=acq-key"] &&
			f["label"][common.RuntimeEnvironmentRunnerIDLabel+"=7"] && len(f["label"]) == 2
	}

	c.On("ContainerList", mock.Anything, mock.MatchedBy(func(o client.ContainerListOptions) bool {
		return o.All && ours(o.Filters)
	})).Return([]container.Summary{}, nil).Once()
	c.On("NetworkList", mock.Anything, mock.MatchedBy(func(o client.NetworkListOptions) bool {
		return ours(o.Filters)
	})).Return([]network.Summary{}, nil).Once()
	c.On("VolumeList", mock.Anything, mock.MatchedBy(func(o client.VolumeListOptions) bool {
		return ours(o.Filters)
	})).Return(client.VolumeListResult{}, nil).Once()

	err := releaseByLabel(t.Context(), c, 7, "acq-key")
	require.ErrorIs(t, err, autoscaler.ErrEnvironmentNotFound)
}

func TestReleaseByLabel_ZeroContainers_ReturnsErrEnvironmentNotFound(t *testing.T) {
	c := docker.NewMockClient(t)

	c.On("ContainerList", mock.Anything, containerListAllTrue).
		Return([]container.Summary{}, nil).Once()
	c.On("NetworkList", mock.Anything, mock.Anything).
		Return([]network.Summary{}, nil).Once()
	c.On("VolumeList", mock.Anything, mock.Anything).
		Return(client.VolumeListResult{}, nil).Once()

	err := releaseByLabel(t.Context(), c, 1, "acq-key")
	require.Error(t, err)
	require.ErrorIs(t, err, autoscaler.ErrEnvironmentNotFound)
}

func TestReleaseByLabel_ZeroContainers_NetworkRemoveError_ReturnsJoined(t *testing.T) {
	// Zero containers + a network removal error: should NOT return ErrEnvironmentNotFound.
	c := docker.NewMockClient(t)
	netErr := errors.New("network remove boom")

	c.On("ContainerList", mock.Anything, containerListAllTrue).
		Return([]container.Summary{}, nil).Once()
	c.On("NetworkList", mock.Anything, mock.Anything).
		Return([]network.Summary{{Network: network.Network{ID: "net-1"}}}, nil).Once()
	c.On("NetworkInspect", mock.Anything, "net-1").
		Return(network.Inspect{}, nil).Once()
	c.On("NetworkRemove", mock.Anything, "net-1").Return(netErr).Once()
	c.On("VolumeList", mock.Anything, mock.Anything).
		Return(client.VolumeListResult{}, nil).Once()

	err := releaseByLabel(t.Context(), c, 1, "acq-key")
	require.Error(t, err)
	require.ErrorIs(t, err, netErr)
	require.NotErrorIs(t, err, autoscaler.ErrEnvironmentNotFound)
}

func TestReleaseByLabel_Order_ContainersThenNetworksThenVolumes(t *testing.T) {
	// Verify removal order: containers first, then networks, then volumes.
	c := docker.NewMockClient(t)
	var callOrder []string

	c.On("ContainerList", mock.Anything, containerListAllTrue).
		Return([]container.Summary{{ID: "ctr-1"}}, nil).Once().
		Run(func(_ mock.Arguments) { callOrder = append(callOrder, "ContainerList") })
	c.On("ContainerRemove", mock.Anything, "ctr-1", mock.Anything).
		Return(nil).Once().
		Run(func(_ mock.Arguments) { callOrder = append(callOrder, "ContainerRemove") })
	c.On("NetworkList", mock.Anything, mock.Anything).
		Return([]network.Summary{{Network: network.Network{ID: "net-1"}}}, nil).Once().
		Run(func(_ mock.Arguments) { callOrder = append(callOrder, "NetworkList") })
	c.On("NetworkInspect", mock.Anything, "net-1").
		Return(network.Inspect{}, nil).Once().
		Run(func(_ mock.Arguments) { callOrder = append(callOrder, "NetworkInspect") })
	c.On("NetworkRemove", mock.Anything, "net-1").
		Return(nil).Once().
		Run(func(_ mock.Arguments) { callOrder = append(callOrder, "NetworkRemove") })
	c.On("VolumeList", mock.Anything, mock.Anything).
		Return(client.VolumeListResult{Items: []volume.Volume{{Name: "vol-1"}}}, nil).Once().
		Run(func(_ mock.Arguments) { callOrder = append(callOrder, "VolumeList") })
	c.On("VolumeRemove", mock.Anything, "vol-1", false).
		Return(nil).Once().
		Run(func(_ mock.Arguments) { callOrder = append(callOrder, "VolumeRemove") })

	err := releaseByLabel(t.Context(), c, 1, "acq-key")
	require.NoError(t, err)

	require.Equal(t, []string{
		"ContainerList", "ContainerRemove",
		"NetworkList", "NetworkInspect", "NetworkRemove",
		"VolumeList", "VolumeRemove",
	}, callOrder, "removal order must be containers → networks → volumes")
}

func TestReleaseByLabel_NetworkEndpoints_ForceDisconnected(t *testing.T) {
	c := docker.NewMockClient(t)

	c.On("ContainerList", mock.Anything, containerListAllTrue).
		Return([]container.Summary{}, nil).Once()
	c.On("NetworkList", mock.Anything, mock.Anything).
		Return([]network.Summary{{Network: network.Network{ID: "net-1"}}}, nil).Once()
	c.On("NetworkInspect", mock.Anything, "net-1").
		Return(network.Inspect{
			Containers: map[string]network.EndpointResource{
				"ep-1": {},
				"ep-2": {},
			},
		}, nil).Once()
	// Both endpoints must be force-disconnected (force=true).
	c.On("NetworkDisconnect", mock.Anything, "net-1", "ep-1", true).Return(nil).Once()
	c.On("NetworkDisconnect", mock.Anything, "net-1", "ep-2", true).Return(nil).Once()
	c.On("NetworkRemove", mock.Anything, "net-1").Return(nil).Once()
	c.On("VolumeList", mock.Anything, mock.Anything).
		Return(client.VolumeListResult{}, nil).Once()

	err := releaseByLabel(t.Context(), c, 1, "acq-key")
	require.ErrorIs(t, err, autoscaler.ErrEnvironmentNotFound)
}

func TestReleaseByLabel_ReusableVolume_PrefixedLabelSkipped(t *testing.T) {
	// A volume with the prefixed reusable label (ReusableVolumeLabel=true) is skipped.
	// A volume with bare "reusable=true" (different key) is NOT skipped.
	c := docker.NewMockClient(t)

	reusableVol := volume.Volume{
		Name:   "cache-vol",
		Labels: map[string]string{labels.ReusableVolumeLabel: "true"},
	}
	bareReusableVol := volume.Volume{
		Name:   "bare-reusable-vol",
		Labels: map[string]string{"reusable": "true"}, // bare key — not the prefixed one
	}

	c.On("ContainerList", mock.Anything, containerListAllTrue).
		Return([]container.Summary{{ID: "ctr-1"}}, nil).Once()
	c.On("ContainerRemove", mock.Anything, "ctr-1", mock.Anything).Return(nil).Once()
	c.On("NetworkList", mock.Anything, mock.Anything).
		Return([]network.Summary{}, nil).Once()
	c.On("VolumeList", mock.Anything, mock.Anything).
		Return(client.VolumeListResult{Items: []volume.Volume{reusableVol, bareReusableVol}}, nil).Once()
	// Only the bare-reusable-vol gets removed; the prefixed-label one is skipped.
	c.On("VolumeRemove", mock.Anything, "bare-reusable-vol", false).Return(nil).Once()

	err := releaseByLabel(t.Context(), c, 1, "acq-key")
	require.NoError(t, err)
}

func TestReleaseByLabel_VolumeConflict_Skipped(t *testing.T) {
	c := docker.NewMockClient(t)

	c.On("ContainerList", mock.Anything, containerListAllTrue).
		Return([]container.Summary{{ID: "ctr-1"}}, nil).Once()
	c.On("ContainerRemove", mock.Anything, "ctr-1", mock.Anything).Return(nil).Once()
	c.On("NetworkList", mock.Anything, mock.Anything).
		Return([]network.Summary{}, nil).Once()
	c.On("VolumeList", mock.Anything, mock.Anything).
		Return(client.VolumeListResult{Items: []volume.Volume{{Name: "vol-conflict"}}}, nil).Once()
	// Conflict is skipped; no error returned.
	c.On("VolumeRemove", mock.Anything, "vol-conflict", false).
		Return(fmt.Errorf("wrap: %w", errdefs.ErrConflict)).Once()

	err := releaseByLabel(t.Context(), c, 1, "acq-key")
	require.NoError(t, err)
}

func TestReleaseByLabel_NotFound_CountsAsOK(t *testing.T) {
	c := docker.NewMockClient(t)

	c.On("ContainerList", mock.Anything, containerListAllTrue).
		Return([]container.Summary{{ID: "ctr-gone"}}, nil).Once()
	// NotFound on remove: counts as already gone, no error.
	c.On("ContainerRemove", mock.Anything, "ctr-gone", mock.Anything).
		Return(fmt.Errorf("wrap: %w", errdefs.ErrNotFound)).Once()
	c.On("NetworkList", mock.Anything, mock.Anything).
		Return([]network.Summary{}, nil).Once()
	c.On("VolumeList", mock.Anything, mock.Anything).
		Return(client.VolumeListResult{}, nil).Once()

	err := releaseByLabel(t.Context(), c, 1, "acq-key")
	require.NoError(t, err)
}

func TestReleaseByLabel_OtherErrors_Joined(t *testing.T) {
	c := docker.NewMockClient(t)
	rmErr := errors.New("remove boom")
	netErr := errors.New("net remove boom")

	c.On("ContainerList", mock.Anything, containerListAllTrue).
		Return([]container.Summary{{ID: "ctr-1"}}, nil).Once()
	c.On("ContainerRemove", mock.Anything, "ctr-1", mock.Anything).Return(rmErr).Once()
	c.On("NetworkList", mock.Anything, mock.Anything).
		Return([]network.Summary{{Network: network.Network{ID: "net-1"}}}, nil).Once()
	c.On("NetworkInspect", mock.Anything, "net-1").
		Return(network.Inspect{}, nil).Once()
	c.On("NetworkRemove", mock.Anything, "net-1").Return(netErr).Once()
	c.On("VolumeList", mock.Anything, mock.Anything).
		Return(client.VolumeListResult{}, nil).Once()

	err := releaseByLabel(t.Context(), c, 1, "acq-key")
	require.Error(t, err)
	require.ErrorIs(t, err, rmErr)
	require.ErrorIs(t, err, netErr)
}

func TestReleaseEnvironment_VMIsolation_Noop(t *testing.T) {
	p := executorProvider{}
	cfg := &common.RunnerConfig{}
	cfg.Docker = &common.DockerConfig{}
	cfg.Autoscaler = &common.AutoscalerConfig{
		VMIsolation: common.VMIsolation{Enabled: true},
	}

	// No docker calls should be made; a nil conn is fine.
	err := p.ReleaseEnvironment(t.Context(), cfg, nil, "any-key")
	require.NoError(t, err)
}

func TestReleaseEnvironment_NilDockerConfig(t *testing.T) {
	p := executorProvider{}
	cfg := &common.RunnerConfig{}

	err := p.ReleaseEnvironment(t.Context(), cfg, nil, "any-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker config is nil")
}

// ReleaseEnvironment must reach the daemon the job used. The two dial modes
// can reach different daemons, and a release that reaches the wrong one finds
// no containers, which callers treat as already released.
func TestReleaseEnvironment_DialMode(t *testing.T) {
	for _, dialStdio := range []bool{true, false} {
		t.Run(map[bool]string{true: "dial-stdio", false: "native tunnel"}[dialStdio], func(t *testing.T) {
			t.Setenv("DOCKER_HOST", "")
			dialErr := errors.New("dial refused")

			// .Maybe() with assertions afterwards: a strict mock fails inside
			// the HTTP transport goroutine and the test hangs instead of failing.
			conn := &executors.MockClient{}
			conn.On("DialRun", mock.Anything, mock.Anything).Return(nil, dialErr).Maybe()
			conn.On("Dial", mock.Anything, mock.Anything).Return((net.Conn)(nil), dialErr).Maybe()

			cfg := &common.RunnerConfig{
				RunnerCredentials: common.RunnerCredentials{ID: 7},
				RunnerSettings: common.RunnerSettings{
					Docker:       &common.DockerConfig{},
					FeatureFlags: map[string]bool{featureflags.UseDockerAutoscalerDialStdio: dialStdio},
				},
			}

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			err := executorProvider{}.ReleaseEnvironment(ctx, cfg, conn, "acq-key")
			require.ErrorIs(t, err, dialErr)

			if dialStdio {
				conn.AssertCalled(t, "DialRun", mock.Anything, "docker system dial-stdio")
				conn.AssertNotCalled(t, "Dial", mock.Anything, mock.Anything)
			} else {
				// with DOCKER_HOST unset, the tunnel dials the SDK's per-OS default
				u, parseErr := client.ParseHostURL(client.DefaultDockerHost)
				require.NoError(t, parseErr)
				conn.AssertCalled(t, "Dial", u.Scheme, u.Host)
				conn.AssertNotCalled(t, "DialRun", mock.Anything, mock.Anything)
			}
			assert.NotErrorIs(t, err, context.DeadlineExceeded)
		})
	}
}

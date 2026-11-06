package docker

import (
	"context"
	"errors"
	"fmt"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"
	autoscaler "gitlab.com/gitlab-org/gitlab-runner/executors/internal/autoscaler"
	dockerhelper "gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

var _ autoscaler.EnvironmentReleaser = executorProvider{}

// ReleaseEnvironment removes all docker objects (containers, networks, volumes)
// belonging to the environment identified by acquisitionKey from the instance
// reached through conn. It is a no-op when vm_isolation is enabled because the
// nested VM is torn down at tunnel close, not by label-based removal.
//
// Release is provider-level because docker has no connection to the instance
// without a job.
func (p executorProvider) ReleaseEnvironment(
	ctx context.Context,
	cfg *common.RunnerConfig,
	conn executors.Client,
	acquisitionKey string,
) error {
	if cfg.Docker == nil {
		return errors.New("release environment: docker config is nil")
	}

	// vm_isolation tears down the nested VM when the tunnel closes; docker
	// objects inside it are gone with the VM and cannot be listed via labels.
	if cfg.Autoscaler != nil && cfg.Autoscaler.VMIsolation.Enabled {
		return nil
	}

	dialStdio := cfg.IsFeatureFlagOn(featureflags.UseDockerAutoscalerDialStdio)
	opts, creds, err := tunnelOpts(ctx, conn, cfg.Docker.Credentials, dialStdio)
	if err != nil {
		return fmt.Errorf("create tunnel dial options: %w", err)
	}

	c, err := dockerhelper.New(creds, opts...)
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer func() { _ = c.Close() }()

	return releaseByLabel(ctx, c, cfg.ID, acquisitionKey)
}

// tunnelOpts builds the docker client options and (possibly modified)
// credentials needed to dial through an executors.Client tunnel. It is shared
// by newDockerTunnel and ReleaseEnvironment.
//
// It creates a new dial context for Docker's client and overrides the daemon
// hostname. For the "npipe", "unix" and "dial-stdio" schemes a fake host is
// needed: otherwise, when dialing from Linux to Windows or vice-versa, docker
// complains because it doesn't think Linux can support "npipe" or Windows
// "unix".
func tunnelOpts(
	ctx context.Context,
	tc executors.Client,
	creds dockerhelper.Credentials,
	dialStdio bool,
) ([]client.Opt, dockerhelper.Credentials, error) {
	scheme, dialer, err := environmentDialContext(ctx, tc, creds.Host, dialStdio)
	if err != nil {
		return nil, creds, fmt.Errorf("create environment dial context: %w", err)
	}

	switch scheme {
	case "unix", "npipe", "dial-stdio":
		creds.Host = internalFakeTunnelHostname
	}

	return []client.Opt{client.WithDialContext(dialer)}, creds, nil
}

// releaseByLabel removes every docker container, network, and named volume
// that carries both the acquisition-key label and the runner-id label matching
// runnerID. Called from Cleanup (when releaseOnCleanup is set) and from
// ReleaseEnvironment.
//
// Order: containers first (so networks and volumes can be detached), then
// networks, then volumes. Only non-reusable volumes are deleted: volumes
// labeled com.gitlab.gitlab-runner.reusable=true are persistent cache volumes
// shared across jobs and must not be removed. volume_keep does not apply; the
// environment is expired or broken.
//
// A ContainerList error is fatal and returned immediately — we cannot know
// what to remove, and removing networks or volumes before their containers
// are gone would fail. NetworkList and VolumeList errors are non-fatal and
// joined: containers are already gone, so remaining objects are best-effort.
//
// NotFound errors count as success. All other errors are collected and joined.
// When no containers are found and there are no other errors,
// ErrEnvironmentNotFound is returned (wrapped) to signal a lost environment.
func releaseByLabel(ctx context.Context, c dockerhelper.Client, runnerID int64, acqKey string) error {
	if acqKey == "" {
		return errors.New("releaseByLabel: acquisition key must not be empty")
	}
	if runnerID <= 0 {
		return fmt.Errorf("releaseByLabel: runner ID must be positive, got %d", runnerID)
	}

	f := client.Filters{}.
		Add("label", fmt.Sprintf("%s=%s", labels.AcquisitionKeyLabel, acqKey)).
		Add("label", fmt.Sprintf("%s=%d", common.RuntimeEnvironmentRunnerIDLabel, runnerID))

	ctrs, err := c.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: f})
	if err != nil {
		return fmt.Errorf("list containers for release: %w", err)
	}

	var errs []error
	for _, ctr := range ctrs {
		if err := c.ContainerRemove(ctx, ctr.ID, client.ContainerRemoveOptions{
			Force:         true,
			RemoveVolumes: true, // removes anonymous volumes only
		}); err != nil && !dockerhelper.IsErrNotFound(err) {
			errs = append(errs, fmt.Errorf("remove container %s: %w", ctr.ID, err))
		}
	}

	// Networks and volumes are processed even when no containers matched,
	// because they may be stranded from a partial earlier release.
	errs = append(errs, releaseNetworks(ctx, c, f)...)
	errs = append(errs, releaseVolumes(ctx, c, f)...)

	if len(errs) == 0 && len(ctrs) == 0 {
		return fmt.Errorf("acquisition %s: %w", acqKey, autoscaler.ErrEnvironmentNotFound)
	}
	return errors.Join(errs...)
}

// releaseNetworks removes all networks matching f, first disconnecting any
// remaining container endpoints.
func releaseNetworks(ctx context.Context, c dockerhelper.Client, f client.Filters) []error {
	nets, err := c.NetworkList(ctx, client.NetworkListOptions{Filters: f})
	if err != nil {
		return []error{fmt.Errorf("list networks for release: %w", err)}
	}

	var errs []error
	for _, net := range nets {
		errs = append(errs, disconnectAndRemoveNetwork(ctx, c, net.ID)...)
	}
	return errs
}

// disconnectAndRemoveNetwork disconnects all endpoints from a network and then
// removes it.
func disconnectAndRemoveNetwork(ctx context.Context, c dockerhelper.Client, netID string) []error {
	netInspect, err := c.NetworkInspect(ctx, netID)
	if err != nil {
		if dockerhelper.IsErrNotFound(err) {
			return nil
		}
		return []error{fmt.Errorf("inspect network %s: %w", netID, err)}
	}

	var errs []error
	for ep := range netInspect.Containers {
		if err := c.NetworkDisconnect(ctx, netID, ep, true); err != nil && !dockerhelper.IsErrNotFound(err) {
			errs = append(errs, fmt.Errorf("disconnect container %s from network %s: %w", ep, netID, err))
		}
	}
	if err := c.NetworkRemove(ctx, netID); err != nil && !dockerhelper.IsErrNotFound(err) {
		errs = append(errs, fmt.Errorf("remove network %s: %w", netID, err))
	}
	return errs
}

// releaseVolumes removes all non-reusable volumes matching f. A volume that
// conflicts is skipped, not logged: it is still attached to a container the
// release did not remove, and the release continues.
func releaseVolumes(ctx context.Context, c dockerhelper.Client, f client.Filters) []error {
	volResult, err := c.VolumeList(ctx, client.VolumeListOptions{Filters: f})
	if err != nil {
		return []error{fmt.Errorf("list volumes for release: %w", err)}
	}

	var errs []error
	for _, vol := range volResult.Items {
		if vol.Labels[labels.ReusableVolumeLabel] == "true" {
			continue
		}
		if err := c.VolumeRemove(ctx, vol.Name, false); err != nil {
			switch {
			case dockerhelper.IsErrNotFound(err):
				// already gone
			case errdefs.IsConflict(err):
				// still attached to a container outside this release
			default:
				errs = append(errs, fmt.Errorf("remove volume %s: %w", vol.Name, err))
			}
		}
	}
	return errs
}

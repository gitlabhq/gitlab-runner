//go:build integration && network_faults

package docker_test

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	mobyclient "github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/test"
)

// pickFreePort finds an ephemeral local port by briefly binding to port 0 and
// closing the listener. There's an inherent (tiny) race between closing this
// listener and the registry container binding the same port later, which is
// the standard accepted trade-off for this technique in tests.
func pickFreePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// startAndSeedRegistry starts a real registry:2 container bound to port on
// the same docker daemon the runner will pull through, pushes testImage into
// it, then stops (but does not remove) the container -- freeing the port
// while preserving the pushed image data in the container's own writable
// layer, so it can be restarted later with the same content already present.
// Returns the (stopped) container's ID.
func startAndSeedRegistry(t *testing.T, client docker.Client, port int, testImage string) string {
	ctx := context.Background()

	// ContainerCreate, unlike `docker run`, does not implicitly pull a
	// missing image -- without this, the container create below fails with
	// "No such image: registry:2" on any daemon that doesn't already happen
	// to have it cached (e.g. a fresh CI dind).
	err := client.ImagePullBlocking(ctx, "registry:2", mobyclient.ImagePullOptions{})
	require.NoError(t, err, "pulling registry:2")

	config := &container.Config{
		Image: "registry:2",
		ExposedPorts: network.PortSet{
			network.MustParsePort("5000/tcp"): {},
		},
	}
	hostConfig := &container.HostConfig{
		PortBindings: network.PortMap{
			network.MustParsePort("5000/tcp"): {{HostPort: fmt.Sprintf("%d", port)}},
		},
	}

	ctr, err := client.ContainerCreate(ctx, config, hostConfig, nil, nil, fmt.Sprintf("pull-retry-registry-%d", port))
	require.NoError(t, err, "creating seed registry container")

	err = client.ContainerStart(ctx, ctr.ID, mobyclient.ContainerStartOptions{})
	require.NoError(t, err, "starting seed registry container")

	// Wait for the registry to be ready by retrying `docker tag`/`docker
	// push` themselves, rather than a raw TCP dial from the test process to
	// "localhost:<port>". `docker` CLI commands go through the daemon via
	// the shared DOCKER_HOST socket, so this depends only on the same
	// daemon connection everything else here already relies on. A raw dial
	// from the test process itself would instead depend on the test
	// process and the daemon sharing a network namespace/loopback, which
	// isn't guaranteed in a dind CI topology where the job container and
	// dind run as separate containers.
	registryAddr := fmt.Sprintf("localhost:%d", port)
	targetRef := registryAddr + "/pull-retry-test:latest"
	out, err := exec.Command("docker", "tag", testImage, targetRef).CombinedOutput()
	require.NoError(t, err, "tagging test image: %s", out)

	deadline := time.Now().Add(15 * time.Second)
	for {
		out, err = exec.Command("docker", "push", targetRef).CombinedOutput()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			require.NoError(t, err, "registry never became ready to accept a push: %s", out)
		}
		time.Sleep(500 * time.Millisecond)
	}

	err = client.ContainerStop(ctx, ctr.ID, mobyclient.ContainerStopOptions{})
	require.NoError(t, err, "stopping seed registry container")

	return ctr.ID
}

// TestDockerCommandPullRetriesTransientRegistryFailure exercises the real
// image-pull retry path (executors/docker/internal/pull/manager.go's
// shouldRetryImagePull/defaultPullMaxAttempts) end to end: the target
// registry is deliberately not listening when the build starts (so early
// pull attempts fail with a genuine "connection refused", which
// common/classify_image_pull_failure.go classifies as
// RunnerExternalDependencyFailure and is therefore retried), then a real
// registry:2 container is started -- on the same docker daemon the runner
// pulls through, so "localhost:<port>" resolves correctly from dockerd's own
// point of view -- partway through the retry backoff window, and the build
// is asserted to succeed once it does.
//
// This is inherently timing-sensitive: it waits through part of the retry
// backoff (defaultPullMaxAttempts=3, 2-10s jittered backoff between
// attempts) before the registry becomes available. The delay below is
// chosen to comfortably land after attempt 1 (which fails immediately) and
// before attempt 3 is exhausted, but is not a hard real-time guarantee.
func TestDockerCommandPullRetriesTransientRegistryFailure(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	client, err := docker.New(docker.Credentials{})
	require.NoError(t, err)
	defer client.Close()

	port := pickFreePort(t)
	registryContainerID := startAndSeedRegistry(t, client, port, common.TestAlpineImage)
	defer func() {
		_ = client.ContainerRemove(context.Background(), registryContainerID, mobyclient.ContainerRemoveOptions{Force: true})
	}()

	imageRef := fmt.Sprintf("localhost:%d/pull-retry-test:latest", port)

	build := getBuildForOS(t, func() (spec.Job, error) {
		return common.GetRemoteBuildResponse("echo done")
	})
	build.Runner.Docker.Image = imageRef
	build.Runner.Docker.PullPolicy = common.StringOrArray{common.PullPolicyAlways}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Land comfortably inside the retry backoff window (attempt 1 fails
		// immediately; backoff before attempt 2 is 2-10s jittered).
		time.Sleep(3 * time.Second)
		startErr := client.ContainerStart(context.Background(), registryContainerID, mobyclient.ContainerStartOptions{})
		assert.NoError(t, startErr, "restarting seeded registry container")
	}()
	defer wg.Wait()

	out, err := buildtest.RunBuildReturningOutput(t, &build)
	assert.NoError(t, err, "build should eventually succeed once the registry becomes reachable")
	assert.Regexp(t, `(?i)retr(y|ying)`, out,
		"trace should show evidence that at least one retry actually happened, not just an immediate success")
}

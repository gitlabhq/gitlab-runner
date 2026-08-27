//go:build integration

package docker_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	docker_executor "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/test"
)

// These are local, opt-in tests for
// https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/31043, which
// asks for GitLab Runner to actually be tested against Podman rather than
// only Docker. There is currently no Podman installation in CI, so these
// tests skip themselves (via podmanHost, below) whenever a reachable Podman
// API socket can't be found, and never run in the pipeline. They're meant
// to be run locally by a contributor who has Podman installed, e.g.:
//
//	go test -tags integration ./executors/docker/... -run TestPodman -v
//
// Each test points the Docker executor's DockerConfig.Credentials.Host
// directly at the discovered Podman socket, so it exercises the real
// executor and the real helpers/docker.Client, not a mock, regardless of
// whatever DOCKER_HOST is (or isn't) set to in the environment.
//
// This covers all four of 31043's test-matrix cells (rootful and
// rootless Podman, root and non-root container user) plus a regression
// test for https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/39608.
// It doesn't touch services with FF_NETWORK_PER_BUILD.

// podmanHost finds the Docker-compatible API socket of a locally reachable
// Podman installation, trying the two ways Podman commonly exposes it:
//
//   - a running `podman machine` (used on macOS/Windows, where Podman runs
//     inside a VM and forwards its socket to a host-side path)
//   - a native Podman socket (used on Linux, where Podman runs directly on
//     the host, e.g. via `systemctl --user start podman.socket`)
//
// It skips the test, rather than starting anything itself, if neither is
// available -- this test must never fail a contributor's or CI's run just
// because Podman happens not to be installed or running. There is
// currently no Podman installation in CI, so this always skips there.
func podmanHost(t *testing.T) string {
	t.Helper()

	podmanPath, err := exec.LookPath("podman")
	if err != nil {
		t.Skip("podman is not installed, skipping Podman integration test")
	}

	if host, ok := podmanMachineHost(podmanPath); ok {
		return host
	}
	if host, ok := podmanNativeHost(podmanPath); ok {
		return host
	}

	t.Skip("no reachable Podman API socket found (tried `podman machine inspect` and `podman info`); " +
		"start one with `podman machine start` or `systemctl --user start podman.socket`, then retry")
	return ""
}

func podmanMachineHost(podmanPath string) (string, bool) {
	state, err := exec.Command(podmanPath, "machine", "inspect", "--format", "{{.State}}").Output()
	if err != nil || strings.TrimSpace(string(state)) != "running" {
		return "", false
	}

	sock, err := exec.Command(podmanPath, "machine", "inspect", "--format", "{{.ConnectionInfo.PodmanSocket.Path}}").Output()
	path := strings.TrimSpace(string(sock))
	if err != nil || path == "" {
		return "", false
	}

	return "unix://" + path, true
}

func podmanNativeHost(podmanPath string) (string, bool) {
	out, err := exec.Command(podmanPath, "info", "--format", "{{.Host.RemoteSocket.Path}}").Output()
	path := strings.TrimSpace(string(out))
	if err != nil || path == "" {
		return "", false
	}
	if _, err := os.Stat(path); err != nil {
		return "", false
	}

	return "unix://" + path, true
}

// rootfulPodmanSocket is the conventional path of a Linux system-wide
// (rootful) Podman API socket.
const rootfulPodmanSocket = "/run/podman/podman.sock"

// podmanRootfulHost finds a reachable rootful Podman socket. Rootful only
// exists on Linux, and reaching its root-owned socket requires running as
// root, so this skips otherwise rather than failing. Run locally with:
//
//	sudo -E go test -tags integration ./executors/docker/... -run TestPodmanRootful -v
func podmanRootfulHost(t *testing.T) string {
	t.Helper()

	if runtime.GOOS != "linux" {
		t.Skip("rootful Podman only exists as a concept on Linux -- on macOS/Windows, " +
			"`podman machine` always runs a rootless daemon inside its VM, skipping Podman rootful integration test")
	}

	if os.Geteuid() != 0 {
		t.Skip("reaching the root-owned rootful Podman socket requires the test binary to run as root; " +
			"rerun as e.g. `sudo -E go test -tags integration ./executors/docker/... -run TestPodmanRootful -v`, " +
			"skipping Podman rootful integration test")
	}

	conn, err := net.DialTimeout("unix", rootfulPodmanSocket, time.Second)
	if err != nil {
		t.Skip("no reachable rootful Podman API socket found at " + rootfulPodmanSocket + "; " +
			"start one with `sudo systemctl start podman.socket` or `sudo podman system service`, then retry")
	}
	conn.Close()

	return "unix://" + rootfulPodmanSocket
}

func podmanRunnerConfig(host string) *common.RunnerConfig {
	return &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Executor: "docker",
			Docker: &common.DockerConfig{
				Credentials: docker.Credentials{Host: host},
				PullPolicy:  common.StringOrArray{common.PullPolicyIfNotPresent},
			},
		},
	}
}

// TestPodmanCommandBasicRun covers the simplest of the scenarios listed in
// work item 31043's test matrix: a job runs successfully against a Podman
// backend at all.
func TestPodmanCommandBasicRun(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	host := podmanHost(t)

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	require.NoError(t, err)
	successfulBuild.Image.Name = common.TestAlpineImage

	build := &common.Build{
		Job:              successfulBuild,
		Runner:           podmanRunnerConfig(host),
		ExecutorProvider: docker_executor.NewProvider(),
	}

	var buf bytes.Buffer
	err = build.Run(context.Background(), &common.Config{}, &common.Trace{Writer: &buf})
	require.NoError(t, err, buf.String())
}

// TestPodmanCommandWithNonRootUser covers another scenario named explicitly
// in work item 31043's test matrix: a container process running as a
// non-root user. Mirrors TestDockerCommandWithUser.
func TestPodmanCommandWithNonRootUser(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	host := podmanHost(t)

	successfulBuild, err := common.GetRemoteBuildResponse("whoami")
	require.NoError(t, err)

	successfulBuild.Image.Name = common.TestAlpineImage
	successfulBuild.Image.ExecutorOptions.Docker.User = "squid"

	build := &common.Build{
		Job:              successfulBuild,
		Runner:           podmanRunnerConfig(host),
		ExecutorProvider: docker_executor.NewProvider(),
	}

	var buf bytes.Buffer
	require.NoError(t, build.Run(context.Background(), &common.Config{}, &common.Trace{Writer: &buf}))
	assert.Regexp(t, "whoami.*\n.*squid", buf.String())
}

// TestPodmanCommandWithPlatformKey is an end-to-end regression test for
// https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/39608, one level
// up from the client-layer unit tests in helpers/docker: it runs a full job
// through the real Docker executor, against a real local Podman daemon,
// with image:docker:platform: set. Before the fix, this failed on every run
// with `"platform" requires API version 1.49, but the Docker daemon API
// version is <podman's version>`.
func TestPodmanCommandWithPlatformKey(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	host := podmanHost(t)

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	require.NoError(t, err)
	successfulBuild.Image.Name = common.TestAlpineImage
	// Containers are always linux, regardless of the host OS this test runs
	// on (e.g. darwin when run locally on macOS against a Podman machine),
	// so the OS must be specified explicitly rather than left for platform
	// parsing to default from the host.
	successfulBuild.Image.ExecutorOptions.Docker.Platform = "linux/" + runtime.GOARCH

	build := &common.Build{
		Job:              successfulBuild,
		Runner:           podmanRunnerConfig(host),
		ExecutorProvider: docker_executor.NewProvider(),
	}

	var buf bytes.Buffer
	err = build.Run(context.Background(), &common.Config{}, &common.Trace{Writer: &buf})
	require.NoError(t, err, buf.String())
}

func TestPodmanRootfulCommandBasicRun(t *testing.T) {
	host := podmanRootfulHost(t)

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	require.NoError(t, err)
	successfulBuild.Image.Name = common.TestAlpineImage

	build := &common.Build{
		Job:              successfulBuild,
		Runner:           podmanRunnerConfig(host),
		ExecutorProvider: docker_executor.NewProvider(),
	}

	var buf bytes.Buffer
	err = build.Run(context.Background(), &common.Config{}, &common.Trace{Writer: &buf})
	require.NoError(t, err, buf.String())
}

func TestPodmanRootfulCommandWithNonRootUser(t *testing.T) {
	host := podmanRootfulHost(t)

	successfulBuild, err := common.GetRemoteBuildResponse("whoami")
	require.NoError(t, err)

	successfulBuild.Image.Name = common.TestAlpineImage
	successfulBuild.Image.ExecutorOptions.Docker.User = "squid"

	build := &common.Build{
		Job:              successfulBuild,
		Runner:           podmanRunnerConfig(host),
		ExecutorProvider: docker_executor.NewProvider(),
	}

	var buf bytes.Buffer
	require.NoError(t, build.Run(context.Background(), &common.Config{}, &common.Trace{Writer: &buf}))
	assert.Regexp(t, "whoami.*\n.*squid", buf.String())
}

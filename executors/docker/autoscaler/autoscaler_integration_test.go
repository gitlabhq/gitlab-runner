//go:build integration

package autoscaler_test

import (
	"context"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

func newRunnerConfig(t *testing.T, shell string) *common.RunnerConfig {
	helpers.SkipIntegrationTests(t, "fleeting-plugin-static", "--version")

	// In theory, pwsh should work if getImage() is upgraded to use the alpine powershell image,
	// however, in practice, we get errors in CI with the pwsh helper image selected.
	// TODO: fix this for pwsh when using pwsh helper image
	if shell == "pwsh" || shell == "powershell" {
		t.Skip()
	}

	dir := t.TempDir()

	t.Log("Build directory:", dir)

	srv, err := ssh.NewStubServer("root", "password")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, srv.Stop())
	})

	srv.ExecuteLocal = true

	image := getImage()

	return &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "runner-token",
		},
		RunnerSettings: common.RunnerSettings{
			BuildsDir: dir,
			Executor:  "docker-autoscaler",
			Shell:     shell,
			Cache:     &cacheconfig.Config{},
			Docker: &common.DockerConfig{
				Image: image,
			},
			Autoscaler: &common.AutoscalerConfig{
				MaxUseCount:         1,
				CapacityPerInstance: 1,
				MaxInstances:        1,
				Plugin:              "fleeting-plugin-static",
				PluginConfig: common.AutoscalerSettingsMap{
					"instances": map[string]map[string]string{
						"local": {
							"username":      srv.User,
							"password":      srv.Password,
							"timeout":       "1m",
							"external_addr": srv.Host() + ":" + srv.Port(),
							"internal_addr": srv.Host() + ":" + srv.Port(),
						},
					},
				},
			},
		},
	}
}

func setupAcquireBuild(t *testing.T, build *common.Build) {
	provider := common.GetExecutorProvider("docker-autoscaler")
	data, err := provider.Acquire(build.Runner)
	require.NoError(t, err)

	build.ExecutorData = data
	t.Cleanup(func() {
		provider.Release(build.Runner, build.ExecutorData)

		if shutdownable, ok := provider.(common.ManagedExecutorProvider); ok {
			shutdownable.Shutdown(context.Background())
		}
	})
}

func TestBuildSuccess(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		require.NoError(t, err)

		build := &common.Build{
			Job:    successfulBuild,
			Runner: newRunnerConfig(t, shell),
		}
		setupAcquireBuild(t, build)

		require.NoError(t, buildtest.RunBuild(t, build))
	})
}

func TestBuildTimeout(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		require.NoError(t, err)
		successfulBuild.Steps[0].Script = []string{"sleep 60"}
		successfulBuild.RunnerInfo.Timeout = 15

		build := &common.Build{
			Job:    successfulBuild,
			Runner: newRunnerConfig(t, shell),
		}
		setupAcquireBuild(t, build)

		runnerID := rand.Intn(999999999)
		build.ProjectRunnerID = runnerID
		build.Variables = append(successfulBuild.Variables, spec.Variable{
			Key:   featureflags.NetworkPerBuild,
			Value: "true",
		})

		// run a job that times out
		err = buildtest.RunBuild(t, build)
		require.ErrorContains(t, err, "execution took longer than 15s seconds")

		// new docker client
		client, err := docker.New(docker.Credentials{})
		require.NoError(t, err, "creating docker client")
		defer client.Close()

		nameFilter := filters.Arg("name", "-"+strconv.Itoa(runnerID)+"-")

		// ensure no build/predefine containers for this job were left behind
		containers, err := client.ContainerList(context.Background(), container.ListOptions{
			Filters: filters.NewArgs(nameFilter),
		})
		require.NoError(t, err)
		assert.Empty(t, containers)

		// ensure no networks for this job were left behind
		networks, err := client.NetworkList(context.Background(), network.ListOptions{
			Filters: filters.NewArgs(nameFilter),
		})
		require.NoError(t, err)
		assert.Empty(t, networks)

		// ensure no volumes for this job were left behind
		// unfortunately there isn't an API to list volumes...
	})
}

func TestBuildSuccessUsingDockerHost(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		require.NoError(t, err)

		build := &common.Build{
			Job:    successfulBuild,
			Runner: newRunnerConfig(t, shell),
		}

		// explicitly set the docker host, which will override the use of connecting
		// via "dial-stdio" to ensure this method of connecting is still possible.
		if host := os.Getenv("DOCKER_HOST"); host != "" {
			build.Runner.Docker.Host = host
		} else {
			build.Runner.Docker.Host = client.DefaultDockerHost
		}

		setupAcquireBuild(t, build)

		require.NoError(t, buildtest.RunBuild(t, build))
	})
}

func TestBuildSuccessUsingDockerHostLegacyTunnel(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		require.NoError(t, err)

		successfulBuild.Variables = append(successfulBuild.Variables, spec.Variable{
			Key:   featureflags.UseDockerAutoscalerDialStdio,
			Value: "false",
		})

		build := &common.Build{
			Job:    successfulBuild,
			Runner: newRunnerConfig(t, shell),
		}

		setupAcquireBuild(t, build)

		require.NoError(t, buildtest.RunBuild(t, build))
	})
}

func TestBuildCancel(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		buildtest.RunBuildWithCancel(t, newRunnerConfig(t, shell), setupAcquireBuild)
	})
}

func TestBuildMasking(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		buildtest.RunBuildWithMasking(t, newRunnerConfig(t, shell), setupAcquireBuild)
	})
}

func TestBuildExpandedFileVariable(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		buildtest.RunBuildWithExpandedFileVariable(t, newRunnerConfig(t, shell), setupAcquireBuild)
	})
}

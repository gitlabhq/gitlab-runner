//go:build integration

package instance_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/executors/instance"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

const integrationTestInstanceExecutor = "instance-integration-test"

func TestMain(m *testing.M) {
	code := 1
	defer func() {
		os.Exit(code)
	}()

	fmt.Println("Compiling gitlab-runner binary for tests")

	targetDir, err := os.MkdirTemp("", "test_executor")
	if err != nil {
		panic("Error on preparing tmp directory for test executor binary")
	}
	defer os.RemoveAll(targetDir)

	path := buildtest.MustBuildBinary("../..", filepath.Join(targetDir, "gitlab-runner-integration"))

	instance.RegisterExecutor(integrationTestInstanceExecutor, path)

	code = m.Run()
}

func newRunnerConfig(t *testing.T, shell string, opts ...ssh.Option) *common.RunnerConfig {
	helpers.SkipIntegrationTests(t, "fleeting-plugin-static", "--version")

	dir := t.TempDir()

	t.Log("Build directory:", dir)

	srv, err := ssh.NewStubServer("root", "password", append([]ssh.Option{ssh.WithExecuteLocal()}, opts...)...)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, srv.Stop())
	})

	switch shell {
	case "bash", "sh":
		srv.Shell = []string{shell, "-c"}
	case "pwsh", "powershell":
		srv.Shell = []string{shell, "-Command"}
	}

	return &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "runner-token",
		},
		RunnerSettings: common.RunnerSettings{
			BuildsDir: dir,
			Executor:  integrationTestInstanceExecutor,
			Shell:     shell,
			Cache:     &cacheconfig.Config{},
			Autoscaler: &common.AutoscalerConfig{
				CapacityPerInstance: 10,
				MaxInstances:        1,
				Plugin:              "fleeting-plugin-static",
				ConnectorConfig: common.ConnectorConfig{
					Timeout: time.Minute,
				},
				PluginConfig: common.AutoscalerSettingsMap{
					"instances": map[string]map[string]string{
						"local": {
							"username":      srv.User,
							"password":      srv.Password,
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
	provider := common.GetExecutorProvider(integrationTestInstanceExecutor)
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

func TestInstanceReadyCommand(t *testing.T) {
	tests := map[string]struct {
		command string
		success bool
	}{
		"no command": {command: "", success: true},
		"exit 0":     {command: "exit 0", success: true},

		// we skip non-success codes for now, as this causes instance churn
		// that is currently difficult to detect.
		// "exit 1": {command: "exit 1", success: false},
		// "exit 128":   {command: "exit 128", success: false},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			shellstest.OnEachShell(t, func(t *testing.T, shell string) {
				successfulBuild, err := common.GetRemoteSuccessfulBuild()
				require.NoError(t, err)

				cfg := newRunnerConfig(t, shell)
				cfg.Autoscaler.InstanceReadyCommand = tc.command

				build := &common.Build{
					Job:    successfulBuild,
					Runner: cfg,
				}
				setupAcquireBuild(t, build)

				err = buildtest.RunBuild(t, build)
				if tc.success {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			})
		})
	}
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

func TestConnectionFailed(t *testing.T) {
	shell := "bash"
	if runtime.GOOS == "windows" {
		shell = "pwsh"
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	require.NoError(t, err)

	build := &common.Build{
		Job:    successfulBuild,
		Runner: newRunnerConfig(t, shell, ssh.WithDontAcceptConnections()),
	}
	build.Runner.Autoscaler.ConnectorConfig.Timeout = 5 * time.Second
	setupAcquireBuild(t, build)

	require.ErrorContains(t, buildtest.RunBuild(t, build), "creating instance environment: dial ssh:")
	require.ErrorContains(t, buildtest.RunBuild(t, build), "ssh: handshake failed: read tcp")
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

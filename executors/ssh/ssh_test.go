//go:build !integration

package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	sshHelpers "gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
)

var (
	executorOptions = executors.ExecutorOptions{
		SharedBuildsDir:  false,
		DefaultBuildsDir: "builds",
		DefaultCacheDir:  "cache",
		Shell: common.ShellScriptInfo{
			Shell:         "bash",
			Type:          common.NormalShell,
			RunnerCommand: "/usr/bin/gitlab-runner-helper",
		},
		ShowHostname: true,
	}
)

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

	executorOptions.Shell.RunnerCommand = buildtest.MustBuildBinary("../..", filepath.Join(targetDir, "gitlab-runner-integration"))

	code = m.Run()
}

func TestPrepare(t *testing.T) {
	tempDir := t.TempDir()
	knownHostsFilePath := filepath.Join(tempDir, "known-hosts-file")
	host := "127.0.0.1"

	runnerConfig := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Executor: "ssh",
			SSH: &common.SshConfig{
				User:           "user",
				Password:       "pass",
				Host:           host,
				KnownHostsFile: knownHostsFilePath,
			},
		},
	}

	build := &common.Build{
		Job: spec.Job{
			GitInfo: spec.GitInfo{
				Sha: "1234567890",
			},
		},
		Runner: &common.RunnerConfig{},
	}

	sshConfig := runnerConfig.RunnerSettings.SSH
	server, err := sshHelpers.NewStubServer(sshConfig.User, sshConfig.Password)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, server.Stop())
	}()

	require.NoError(t, os.WriteFile(
		knownHostsFilePath,
		[]byte(fmt.Sprintf("[%s]:%s %s\n", host, server.Port(), sshHelpers.TestSSHKeyPair.PublicKey)),
		0o644,
	))

	sshConfig.Port = server.Port()

	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executorOptions,
		},
	}

	prepareOptions := common.ExecutorPrepareOptions{
		Config:  runnerConfig,
		Build:   build,
		Context: t.Context(),
	}

	err = e.Prepare(prepareOptions)
	assert.NoError(t, err)
}

func TestSharedEnv(t *testing.T) {
	provider := common.GetExecutorProvider("ssh")
	features := &common.FeaturesInfo{}

	_ = provider.GetFeatures(features)
	assert.True(t, features.Shared)
}

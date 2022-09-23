//go:build !integration

package ssh

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
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

func TestPrepare(t *testing.T) {
	tempDir := t.TempDir()
	knownHostsFilePath := filepath.Join(tempDir, "known-hosts-file")
	host := "127.0.0.1"

	runnerConfig := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Executor: "ssh",
			SSH: &sshHelpers.Config{
				User:           "user",
				Password:       "pass",
				Host:           host,
				KnownHostsFile: knownHostsFilePath,
			},
		},
	}

	build := &common.Build{
		JobResponse: common.JobResponse{
			GitInfo: common.GitInfo{
				Sha: "1234567890",
			},
		},
		Runner: &common.RunnerConfig{},
	}

	sshConfig := runnerConfig.RunnerSettings.SSH
	server, err := sshHelpers.NewStubServer(sshConfig.User, sshConfig.Password)
	assert.NoError(t, err)

	require.NoError(t, os.WriteFile(
		knownHostsFilePath,
		[]byte(fmt.Sprintf("[%s]:%s %s\n", host, server.Port(), sshHelpers.TestSSHKeyPair.PublicKey)),
		0o644,
	))

	defer server.Stop()

	sshConfig.Port = server.Port()

	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executorOptions,
		},
	}

	prepareOptions := common.ExecutorPrepareOptions{
		Config:  runnerConfig,
		Build:   build,
		Context: context.TODO(),
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

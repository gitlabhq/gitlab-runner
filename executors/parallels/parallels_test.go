package parallels_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
)

const prlImage = "ubuntu-runner"
const prlCtl = "prlctl"

var prlSSHConfig = &ssh.Config{
	User:     "vagrant",
	Password: "vagrant",
}

func TestParallelsExecutorRegistered(t *testing.T) {
	executorNames := common.GetExecutorNames()
	assert.Contains(t, executorNames, "parallels")
}

func TestParallelsCreateExecutor(t *testing.T) {
	executor := common.NewExecutor("parallels")
	assert.NotNil(t, executor)
}

func TestParallelsSuccessRun(t *testing.T) {
	if helpers.SkipIntegrationTests(t, prlCtl, "--version") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "parallels",
				Parallels: &common.ParallelsConfig{
					BaseName:         prlImage,
					DisableSnapshots: true,
				},
				SSH: prlSSHConfig,
			},
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err, "Make sure that you have done 'make -C tests/ubuntu parallels'")
}

func TestParallelsBuildFail(t *testing.T) {
	if helpers.SkipIntegrationTests(t, prlCtl, "--version") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "parallels",
				Parallels: &common.ParallelsConfig{
					BaseName:         prlImage,
					DisableSnapshots: true,
				},
				SSH: prlSSHConfig,
			},
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err, "error")
	assert.IsType(t, err, &common.BuildError{})
	assert.Contains(t, err.Error(), "Process exited with: 1")
}

func TestParallelsMissingImage(t *testing.T) {
	if helpers.SkipIntegrationTests(t, prlCtl, "--version") {
		return
	}

	build := &common.Build{
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "parallels",
				Parallels: &common.ParallelsConfig{
					BaseName:         "non-existing-image",
					DisableSnapshots: true,
				},
				SSH: prlSSHConfig,
			},
		},
	}

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Could not find a registered machine named")
}

func TestParallelsMissingSSHCredentials(t *testing.T) {
	if helpers.SkipIntegrationTests(t, prlCtl, "--version") {
		return
	}

	build := &common.Build{
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "parallels",
				Parallels: &common.ParallelsConfig{
					BaseName:         "non-existing-image",
					DisableSnapshots: true,
				},
			},
		},
	}

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Missing SSH config")
}

func TestParallelsBuildAbort(t *testing.T) {
	if helpers.SkipIntegrationTests(t, prlCtl, "--version") {
		return
	}

	longRunningBuild, err := common.GetRemoteLongRunningBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: longRunningBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "parallels",
				Parallels: &common.ParallelsConfig{
					BaseName:         prlImage,
					DisableSnapshots: true,
				},
				SSH: prlSSHConfig,
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
	}

	abortTimer := time.AfterFunc(time.Second, func() {
		t.Log("Interrupt")
		build.SystemInterrupt <- os.Interrupt
	})
	defer abortTimer.Stop()

	timeoutTimer := time.AfterFunc(time.Minute, func() {
		t.Log("Timedout")
		t.FailNow()
	})
	defer timeoutTimer.Stop()

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "aborted: interrupt")
}

func TestParallelsBuildCancel(t *testing.T) {
	if helpers.SkipIntegrationTests(t, prlCtl, "--version") {
		return
	}

	longRunningBuild, err := common.GetRemoteLongRunningBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: longRunningBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "parallels",
				Parallels: &common.ParallelsConfig{
					BaseName:         prlImage,
					DisableSnapshots: true,
				},
				SSH: prlSSHConfig,
			},
		},
	}

	trace := &common.Trace{Writer: os.Stdout}

	abortTimer := time.AfterFunc(time.Second, func() {
		t.Log("Interrupt")
		trace.CancelFunc()
	})
	defer abortTimer.Stop()

	timeoutTimer := time.AfterFunc(time.Minute, func() {
		t.Log("Timedout")
		t.FailNow()
	})
	defer timeoutTimer.Stop()

	err = build.Run(&common.Config{}, trace)
	assert.IsType(t, err, &common.BuildError{})
	assert.EqualError(t, err, "canceled")
}

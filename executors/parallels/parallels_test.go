package parallels_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
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
	helpers.SkipIntegrationTests(t, prlCtl, "--version")

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
	assert.NoError(t, err, "Make sure that you have done 'make development_setup'")
}

func TestParallelsSuccessRunRawVariable(t *testing.T) {
	helpers.SkipIntegrationTests(t, prlCtl, "--version")

	successfulBuild, err := common.GetRemoteBuildResponse("echo $TEST")
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

	value := "$VARIABLE$WITH$DOLLARS$$"
	build.Variables = append(build.Variables, common.JobVariable{
		Key:   "TEST",
		Value: value,
		Raw:   true,
	})

	out, err := buildtest.RunBuildReturningOutput(t, build)
	require.NoError(t, err, "Make sure that you have done 'make development_setup'")
	assert.Contains(t, out, value)
}

func TestParallelsBuildFail(t *testing.T) {
	helpers.SkipIntegrationTests(t, prlCtl, "--version")

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
	assert.IsType(t, &common.BuildError{}, err)
	assert.Contains(t, err.Error(), "Process exited with status 1")
}

func TestParallelsMissingImage(t *testing.T) {
	helpers.SkipIntegrationTests(t, prlCtl, "--version")

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
	helpers.SkipIntegrationTests(t, prlCtl, "--version")

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
	assert.Contains(t, err.Error(), "missing SSH config")
}

func TestParallelsBuildCancel(t *testing.T) {
	helpers.SkipIntegrationTests(t, prlCtl, "--version")

	config := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Executor: "parallels",
			Parallels: &common.ParallelsConfig{
				BaseName:         prlImage,
				DisableSnapshots: true,
			},
			SSH: prlSSHConfig,
		},
	}

	buildtest.RunBuildWithCancel(t, config, nil)
}

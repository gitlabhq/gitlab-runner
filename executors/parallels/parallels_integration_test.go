//go:build integration

package parallels_test

import (
	"os"
	"testing"

	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
)

const (
	prlImage = "ubuntu-runner"
	prlCtl   = "prlctl"
)

var prlSSHConfig = &ssh.Config{
	User:     "vagrant",
	Password: "vagrant",
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

func TestBuildScriptSections(t *testing.T) {
	helpers.SkipIntegrationTests(t, prlCtl, "--version")

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		if shell == "cmd" || shell == "pwsh" || shell == "powershell" {
			// support for pwsh and powershell tracked in https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28119
			t.Skip("CMD, pwsh, powershell not supported")
		}

		successfulBuild, err := common.GetRemoteBuildResponse("echo Hello World")

		build := &common.Build{
			JobResponse: successfulBuild,
			Runner: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Executor: "parallels",
					Parallels: &common.ParallelsConfig{
						BaseName:         prlImage,
						DisableSnapshots: true,
					},
					SSH:   prlSSHConfig,
					Shell: shell,
				},
			},
		}

		require.NoError(t, err)
		buildtest.RunBuildWithSections(t, build)
	})
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
	var buildError *common.BuildError
	assert.ErrorAs(t, err, &buildError)
	assert.Contains(t, err.Error(), "Process exited with status 1")
	assert.Equal(t, 1, buildError.ExitCode)
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

func TestBuildLogLimitExceeded(t *testing.T) {
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

	buildtest.RunRemoteBuildWithJobOutputLimitExceeded(t, config, nil)
}

func TestParallelsBuildMasking(t *testing.T) {
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

	buildtest.RunBuildWithMasking(t, config, nil)
}

func getTestBuild(t *testing.T, getJobResp func() (common.JobResponse, error)) *common.Build {
	jobResponse, err := getJobResp()
	require.NoError(t, err)

	return &common.Build{
		JobResponse: jobResponse,
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
}

func TestCleanupProjectGitClone(t *testing.T) {
	helpers.SkipIntegrationTests(t, prlCtl, "--version")

	buildtest.RunBuildWithCleanupGitClone(t, getTestBuild(t, common.GetRemoteSuccessfulBuild))
}

func TestCleanupProjectGitFetch(t *testing.T) {
	helpers.SkipIntegrationTests(t, prlCtl, "--version")

	untrackedFilename := "untracked"

	build := getTestBuild(t, func() (common.JobResponse, error) {
		return common.GetRemoteBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(untrackedFilename, "", "")...,
		)
	})

	buildtest.RunBuildWithCleanupGitFetch(t, build, untrackedFilename)
}

func TestCleanupProjectGitSubmoduleNormal(t *testing.T) {
	helpers.SkipIntegrationTests(t, prlCtl, "--version")

	untrackedFile := "untracked"
	untrackedSubmoduleFile := "untracked_submodule"

	build := getTestBuild(t, func() (common.JobResponse, error) {
		return common.GetRemoteBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(untrackedFile, untrackedSubmoduleFile, "")...,
		)
	})

	buildtest.RunBuildWithCleanupNormalSubmoduleStrategy(t, build, untrackedFile, untrackedSubmoduleFile)
}

func TestCleanupProjectGitSubmoduleRecursive(t *testing.T) {
	helpers.SkipIntegrationTests(t, prlCtl, "--version")

	untrackedFile := "untracked"
	untrackedSubmoduleFile := "untracked_submodule"
	untrackedSubSubmoduleFile := "untracked_submodule_submodule"

	build := getTestBuild(t, func() (common.JobResponse, error) {
		return common.GetRemoteBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(
				untrackedFile,
				untrackedSubmoduleFile,
				untrackedSubSubmoduleFile)...,
		)
	})

	buildtest.RunBuildWithCleanupRecursiveSubmoduleStrategy(
		t,
		build,
		untrackedFile,
		untrackedSubmoduleFile,
		untrackedSubSubmoduleFile,
	)
}

func TestBuildExpandedFileVariable(t *testing.T) {
	helpers.SkipIntegrationTests(t, prlCtl, "--version")

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
		buildtest.RunBuildWithExpandedFileVariable(t, build.Runner, nil)
	})
}

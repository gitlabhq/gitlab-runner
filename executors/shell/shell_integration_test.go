package shell_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/test"
	"gitlab.com/gitlab-org/gitlab-runner/session"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

func gitInDir(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	return cmd.Output()
}

func skipOnGitWithMessage(t *testing.T, constraints string, message string) {
	out, err := gitInDir("", "version")
	if err != nil {
		t.Fatal("Can't detect git version", err)
		return
	}

	gitVersionOut := string(out)
	split := strings.SplitN(gitVersionOut, " ", 3)
	if len(split) < 3 {
		t.Fatal("Can't extract git version from", gitVersionOut)
		return
	}

	// Take the first 3 components of the version (so we ignore suffix in e.g. 2.23.0.windows.1)
	versionComponents := strings.Split(strings.TrimSpace(split[2]), ".")
	versionStr := strings.Join(versionComponents[:3], ".")
	gitVersion, err := version.NewVersion(versionStr)
	if err != nil {
		t.Fatal("Can't detect git version", err)
		return
	}

	rules, err := version.NewConstraint(constraints)
	if err != nil {
		t.Fatal("Invalid constraint", err)
		return
	}

	shouldSkip := rules.Check(gitVersion)
	if shouldSkip {
		t.Skipf("Git %q found, skipping the test; %s", constraints, message)
	}
}

func skipIfGitDoesNotSupportLFS(t *testing.T) {
	skipOnGitWithMessage(t, "< 1.8.2", "available git version doesn't support LFS")
}

func skipOnGit(t *testing.T, constraints string) {
	skipOnGitWithMessage(t, constraints, "")
}

func skipOnGit17x(t *testing.T) {
	skipOnGit(t, "< 1.8")
}

func newBuild(t *testing.T, getBuildResponse common.JobResponse, shell string) (*common.Build, func()) {
	dir, err := ioutil.TempDir("", "gitlab-runner-shell-executor-test")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Build directory:", dir)

	build := &common.Build{
		JobResponse: getBuildResponse,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				BuildsDir: dir,
				Executor:  "shell",
				Shell:     shell,
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
		Session: &session.Session{
			DisconnectCh: make(chan error),
			TimeoutCh:    make(chan error),
		},
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return build, cleanup
}

func TestBuildSuccess(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestMultistepBuild(t *testing.T) {
	successfulBuild, err := common.GetRemoteSuccessfulMultistepBuild()
	require.NoError(t, err)

	failingScriptBuild, err := common.GetRemoteFailingMultistepBuild(common.StepNameScript)
	require.NoError(t, err)

	failingReleaseBuild, err := common.GetRemoteFailingMultistepBuild("release")
	require.NoError(t, err)

	tests := map[string]struct {
		jobResponse    common.JobResponse
		expectedOutput []string
		unwantedOutput []string
		errExpected    bool
	}{
		"Successful build with release and after_script step": {
			jobResponse: successfulBuild,
			expectedOutput: []string{
				"echo Hello World",
				"echo Release",
				"echo After Script",
			},
			errExpected: false,
		},
		"Failure on script step. Release is skipped. After script runs.": {
			jobResponse: failingScriptBuild,
			expectedOutput: []string{
				"echo Hello World",
				"echo After Script",
			},
			unwantedOutput: []string{
				"echo Release",
			},
			errExpected: true,
		},
		"Failure on release step. After script runs.": {
			jobResponse: failingReleaseBuild,
			expectedOutput: []string{
				"echo Hello World",
				"echo Release",
				"echo After Script",
			},
			errExpected: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			shellstest.OnEachShell(t, func(t *testing.T, shell string) {
				build, cleanup := newBuild(t, tt.jobResponse, shell)
				defer cleanup()

				out, err := buildtest.RunBuildReturningOutput(t, build)

				for _, output := range tt.expectedOutput {
					assert.Contains(t, out, output)
				}

				for _, output := range tt.unwantedOutput {
					assert.NotContains(t, out, output)
				}

				if tt.errExpected {
					var buildErr *common.BuildError
					assert.True(t, errors.As(err, &buildErr), "expected %T, got %T", buildErr, err)
					return
				}
				assert.NoError(t, err)
			})
		})
	}
}

func TestBuildJobStatusEnvVars(t *testing.T) {
	tests := map[string]struct {
		fail   bool
		assert func(t *testing.T, err error, build *common.Build, out string)
	}{
		"state and stage on failure": {
			fail: true,
			assert: func(t *testing.T, err error, build *common.Build, out string) {
				assert.Error(t, err)
				assert.Contains(t, out, "CI_JOB_STATUS=failed")
				assert.Equal(t, common.BuildRunRuntimeFailed, build.CurrentState())
			},
		},
		"state and stage on success": {
			fail: false,
			assert: func(t *testing.T, err error, build *common.Build, out string) {
				assert.NoError(t, err)
				assert.Contains(t, out, "CI_JOB_STATUS=success")
				assert.Equal(t, common.BuildRunRuntimeSuccess, build.CurrentState())
			},
		},
	}

	expectedStages := []common.BuildStage{
		common.BuildStagePrepare,
		common.BuildStage("step_env"),
		common.BuildStageAfterScript,
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			shellstest.OnEachShell(t, func(t *testing.T, shell string) {
				multistepBuildScript, err := common.GetRemoteFailingMultistepBuildWithEnvs(shell, tc.fail)
				require.NoError(t, err)

				build, cleanup := newBuild(t, multistepBuildScript, shell)
				defer cleanup()

				out, err := buildtest.RunBuildReturningOutput(t, build)

				assert.Contains(t, out, "CI_JOB_STATUS=running")
				for _, stage := range expectedStages {
					assert.Contains(t, out, common.GetStageDescription(stage))
				}

				tc.assert(t, err, build, out)
			})
		})
	}
}

func TestRawVariableOutput(t *testing.T) {
	tests := map[string]struct {
		command string
	}{
		"bash": {
			command: "echo $TEST",
		},
		"powershell": {
			command: "echo $env:TEST",
		},
		"pwsh": {
			command: "echo $env:TEST",
		},
	}

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		test, ok := tests[shell]
		if !ok {
			t.Skip()
		}

		successfulBuild, err := common.GetLocalBuildResponse(test.command)
		require.NoError(t, err)

		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		value := "$VARIABLE$WITH$DOLLARS$$"
		build.Variables = append(build.Variables, common.JobVariable{
			Key:   "TEST",
			Value: value,
			Raw:   true,
		})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, value)
	})
}

func TestBuildCancel(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build, cleanup := newBuild(t, common.JobResponse{}, shell)
		defer cleanup()

		updateSleepForCMD := func(build *common.Build) {
			if shell != "cmd" {
				return
			}

			resp, err := common.GetRemoteLongRunningBuildWithAfterScriptCMD()
			require.NoError(t, err)

			build.JobResponse = resp
		}

		buildtest.RunBuildWithCancel(t, build.Runner, updateSleepForCMD)
	})
}

func TestBuildWithIndexLock(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)

		build.JobResponse.AllowGitFetch = true
		err = ioutil.WriteFile(build.BuildDir+"/.git/index.lock", []byte{}, os.ModeSticky)
		require.NoError(t, err)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildWithShallowLock(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(
			build.Variables,
			common.JobVariable{Key: "GIT_DEPTH", Value: "1"},
			common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"},
		)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)

		err = ioutil.WriteFile(build.BuildDir+"/.git/shallow.lock", []byte{}, os.ModeSticky)
		require.NoError(t, err)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildWithHeadLock(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)

		build.JobResponse.AllowGitFetch = true
		err = ioutil.WriteFile(build.BuildDir+"/.git/HEAD.lock", []byte{}, os.ModeSticky)
		require.NoError(t, err)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildWithGitLFSHook(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)

		gitLFSPostCheckoutHook := "#!/bin/sh\necho 'running git lfs hook' >&2\nexit 2\n"

		err = os.MkdirAll(build.BuildDir+"/.git/hooks/", 0755)
		require.NoError(t, err)
		err = ioutil.WriteFile(build.BuildDir+"/.git/hooks/post-checkout", []byte(gitLFSPostCheckoutHook), 0777)
		require.NoError(t, err)
		build.JobResponse.AllowGitFetch = true

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)
	})
}

func assertLFSFileDownloaded(t *testing.T, build *common.Build) {
	lfsFilePath := filepath.Join(build.FullProjectDir(), "files", "lfs", "file_1.lfs")
	info, err := os.Stat(lfsFilePath)
	require.NoError(t, err)
	assert.Equal(t, common.FilesLFSFile1LFSsize, info.Size(), "invalid size of %q file", lfsFilePath)
}

func assertLFSFileNotDownloaded(t *testing.T, build *common.Build) {
	lfsFilePath := filepath.Join(build.FullProjectDir(), "files", "lfs", "file_1.lfs")
	info, err := os.Stat(lfsFilePath)
	require.NoError(t, err)
	assert.True(
		t,
		info.Size() < common.FilesLFSFile1LFSsize,
		"invalid size of %q file - expected to be less then downloaded LFS object",
		lfsFilePath,
	)
}

func assertLFSFileNotPresent(t *testing.T, build *common.Build) {
	lfsFilePath := filepath.Join(build.FullProjectDir(), "files", "lfs", "file_1.lfs")
	_, err := os.Stat(lfsFilePath)
	require.IsType(t, &os.PathError{}, err)
	assert.Equal(t, lfsFilePath, err.(*os.PathError).Path)
}

func TestBuildWithGitStrategyNoneWithoutLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "none"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "pre-clone-script")
		assert.NotContains(t, out, "Created fresh repository")
		assert.NotContains(t, out, "Fetching changes")
		assert.Contains(t, out, "Skipping Git repository setup")
	})
}

func TestBuildWithGitStrategyNoneWithLFS(t *testing.T) {
	skipIfGitDoesNotSupportLFS(t)

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulLFSBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "none"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Created fresh repository")
		assert.NotContains(t, out, "Fetching changes")
		assert.Contains(t, out, "Skipping Git repository setup")
		assertLFSFileNotPresent(t, build)
	})
}

func TestBuildWithGitStrategyFetchWithoutLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)

		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Fetching changes")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
		assert.Contains(t, out, "pre-clone-script")
	})
}

func TestBuildWithGitStrategyFetchWithLFS(t *testing.T) {
	skipIfGitDoesNotSupportLFS(t)

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
		assertLFSFileNotPresent(t, build)

		build.GitInfo = common.GetLFSGitInfo(build.GitInfo.RepoURL)

		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Fetching changes")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
		assertLFSFileDownloaded(t, build)
	})
}

func TestBuildWithGitStrategyFetchWithUserDisabledLFS(t *testing.T) {
	skipIfGitDoesNotSupportLFS(t)

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(
			build.Variables,
			common.JobVariable{Key: "GIT_LFS_SKIP_SMUDGE", Value: "1", Public: true},
			common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"},
		)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
		assertLFSFileNotPresent(t, build)

		build.GitInfo = common.GetLFSGitInfo(build.GitInfo.RepoURL)
		build.Variables = append(
			build.Variables,
			common.JobVariable{Key: "GIT_LFS_SKIP_SMUDGE", Value: "1", Public: true},
		)

		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Fetching changes")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
		assertLFSFileNotDownloaded(t, build)
	})
}

func TestBuildWithGitStrategyFetchNoCheckoutWithoutLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Variables = append(
			build.Variables,
			common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"},
			common.JobVariable{Key: "GIT_CHECKOUT", Value: "false"},
		)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.Contains(t, out, "Skipping Git checkout")

		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Fetching changes")
		assert.Contains(t, out, "Skipping Git checkout")
		assert.Contains(t, out, "pre-clone-script")
	})
}

func TestBuildWithGitStrategyFetchNoCheckoutWithLFS(t *testing.T) {
	skipIfGitDoesNotSupportLFS(t)

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulLFSBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(
			build.Variables,
			common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"},
			common.JobVariable{Key: "GIT_CHECKOUT", Value: "false"},
		)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.Contains(t, out, "Skipping Git checkout")
		assertLFSFileNotPresent(t, build)

		build.GitInfo = common.GetLFSGitInfo(build.GitInfo.RepoURL)

		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Fetching changes")
		assert.Contains(t, out, "Skipping Git checkout")
		assertLFSFileNotPresent(t, build)
	})
}

func TestBuildWithGitStrategyCloneWithoutLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "clone"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")

		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
		assert.Contains(t, out, "pre-clone-script")
	})
}

func TestBuildWithGitStrategyCloneWithLFS(t *testing.T) {
	skipIfGitDoesNotSupportLFS(t)

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulLFSBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "clone"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assertLFSFileDownloaded(t, build)
	})
}

func TestBuildWithGitStrategyCloneWithUserDisabledLFS(t *testing.T) {
	skipIfGitDoesNotSupportLFS(t)

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulLFSBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(
			build.Variables,
			common.JobVariable{Key: "GIT_STRATEGY", Value: "clone"},
			common.JobVariable{Key: "GIT_LFS_SKIP_SMUDGE", Value: "1", Public: true},
		)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assertLFSFileNotDownloaded(t, build)
	})
}

func TestBuildWithGitStrategyCloneNoCheckoutWithoutLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Variables = append(
			build.Variables,
			common.JobVariable{Key: "GIT_STRATEGY", Value: "clone"},
			common.JobVariable{Key: "GIT_CHECKOUT", Value: "false"},
		)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")

		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.Contains(t, out, "Skipping Git checkout")
		assert.Contains(t, out, "pre-clone-script")
	})
}

func TestBuildWithGitStrategyCloneNoCheckoutWithLFS(t *testing.T) {
	skipIfGitDoesNotSupportLFS(t)

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulLFSBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(
			build.Variables,
			common.JobVariable{Key: "GIT_STRATEGY", Value: "clone"},
			common.JobVariable{Key: "GIT_CHECKOUT", Value: "false"},
		)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.Contains(t, out, "Skipping Git checkout")
		assertLFSFileNotPresent(t, build)
	})
}

func TestBuildWithSubmoduleLFSPullsLFSObject(t *testing.T) {
	skipIfGitDoesNotSupportLFS(t)

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()
		build.Variables = append(
			build.Variables,
			common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"},
			common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"},
		)
		build.GitInfo = common.GetSubmoduleLFSGitInfo(build.GitInfo.RepoURL)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")

		f, err := os.Stat(filepath.Join(build.FullProjectDir(), "lfs", "1.lfs"))
		require.NoError(t, err)
		assert.Equal(t, common.FilesLFSFile1LFSsize, f.Size())
	})
}

func TestBuildWithSubmoduleLFSDisabledSmudging(t *testing.T) {
	skipIfGitDoesNotSupportLFS(t)

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()
		build.Variables = append(
			build.Variables,
			common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"},
			common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"},
			common.JobVariable{Key: "GIT_LFS_SKIP_SMUDGE", Value: "1", Public: true},
		)
		build.GitInfo = common.GetSubmoduleLFSGitInfo(build.GitInfo.RepoURL)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")

		f, err := os.Stat(filepath.Join(build.FullProjectDir(), "lfs", "1.lfs"))
		require.NoError(t, err)
		assert.True(t, f.Size() < common.FilesLFSFile1LFSsize)
	})
}

func TestBuildWithGitSubmoduleStrategyNone(t *testing.T) {
	for _, strategy := range []string{"none", ""} {
		t.Run("strategy "+strategy, func(t *testing.T) {
			shellstest.OnEachShell(t, func(t *testing.T, shell string) {
				successfulBuild, err := common.GetSuccessfulBuild()
				assert.NoError(t, err)
				build, cleanup := newBuild(t, successfulBuild, shell)
				defer cleanup()

				build.Variables = append(
					build.Variables,
					common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "none"},
				)

				out, err := buildtest.RunBuildReturningOutput(t, build)
				assert.NoError(t, err)
				assert.Contains(t, out, "Skipping Git submodules setup")
				assert.NotContains(t, out, "Updating/initializing submodules...")
				assert.NotContains(t, out, "Updating/initializing submodules recursively...")

				_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", ".git"))
				assert.Error(t, err, "Submodule not should have been initialized")

				_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", "tests", "example", ".git"))
				assert.Error(t, err, "The submodule's submodule should not have been initialized")
			})
		})
	}
}

func TestBuildWithGitSubmoduleStrategyNormal(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Skipping Git submodules setup")
		assert.Contains(t, out, "Updating/initializing submodules...")
		assert.NotContains(t, out, "Updating/initializing submodules recursively...")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", ".git"))
		assert.NoError(t, err, "Submodule should have been initialized")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", "tests", "example", ".git"))
		assert.Error(t, err, "The submodule's submodule should not have been initialized")
	})
}

func TestBuildWithGitSubmoduleStrategyRecursive(t *testing.T) {
	skipOnGit17x(t)

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Skipping Git submodules setup")
		assert.NotContains(t, out, "Updating/initializing submodules...")
		assert.Contains(t, out, "Updating/initializing submodules recursively...")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", ".git"))
		assert.NoError(t, err, "Submodule should have been initialized")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", "tests", "example", ".git"))
		assert.NoError(t, err, "The submodule's submodule should have been initialized")
	})
}

func TestBuildWithGitFetchSubmoduleStrategyRecursive(t *testing.T) {
	skipOnGit17x(t)

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(
			build.Variables,
			common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"},
			common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"},
		)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Skipping Git submodules setup")
		assert.NotContains(t, out, "Updating/initializing submodules...")
		assert.Contains(t, out, "Updating/initializing submodules recursively...")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", ".git"))
		assert.NoError(t, err, "Submodule should have been initialized")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", "tests", "example", ".git"))
		assert.NoError(t, err, "The submodule's submodule should have been initialized")

		// Create a file not tracked that should be cleaned in submodule.
		excludedFilePath := filepath.Join(build.BuildDir, "gitlab-grack", "excluded_file")
		err = ioutil.WriteFile(excludedFilePath, []byte{}, os.ModePerm)
		require.NoError(t, err)

		// Run second build, to run fetch.
		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Created fresh repository")
		assert.Contains(t, out, "Removing excluded_file")
	})
}

func TestBuildWithGitSubmoduleStrategyInvalid(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "invalid"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.EqualError(t, err, "unknown GIT_SUBMODULE_STRATEGY")
		assert.NotContains(t, out, "Skipping Git submodules setup")
		assert.NotContains(t, out, "Updating/initializing submodules...")
		assert.NotContains(t, out, "Updating/initializing submodules recursively...")
	})
}

func TestBuildWithGitSubmoduleStrategyRecursiveAndGitStrategyNone(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(
			build.Variables,
			common.JobVariable{Key: "GIT_STRATEGY", Value: "none"},
			common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"},
		)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Created fresh repository")
		assert.NotContains(t, out, "Fetching changes")
		assert.Contains(t, out, "Skipping Git repository setup")
		assert.NotContains(t, out, "Updating/initializing submodules...")
		assert.NotContains(t, out, "Updating/initializing submodules recursively...")
		assert.Contains(t, out, "Skipping Git submodules setup")
	})
}

func TestBuildWithGitSubmoduleModified(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Updating/initializing submodules...")

		submoduleDir := filepath.Join(build.BuildDir, "gitlab-grack")
		submoduleReadme := filepath.Join(submoduleDir, "README.md")

		// modify submodule and commit
		modifySubmoduleBeforeCommit := "committed change"
		err = ioutil.WriteFile(submoduleReadme, []byte(modifySubmoduleBeforeCommit), os.ModeSticky)
		require.NoError(t, err)
		_, err = gitInDir(submoduleDir, "add", "README.md")
		assert.NoError(t, err)
		_, err = gitInDir(submoduleDir, "config", "user.name", "test")
		assert.NoError(t, err)
		_, err = gitInDir(submoduleDir, "config", "user.email", "test@example.org")
		assert.NoError(t, err)
		_, err = gitInDir(submoduleDir, "commit", "-m", "modify submodule")
		assert.NoError(t, err)

		_, err = gitInDir(build.BuildDir, "add", "gitlab-grack")
		assert.NoError(t, err)
		_, err = gitInDir(build.BuildDir, "config", "user.name", "test")
		assert.NoError(t, err)
		_, err = gitInDir(build.BuildDir, "config", "user.email", "test@example.org")
		assert.NoError(t, err)
		_, err = gitInDir(build.BuildDir, "commit", "-m", "modify submodule")
		assert.NoError(t, err)

		// modify submodule without commit before second build
		modifySubmoduleAfterCommit := "not committed change"
		err = ioutil.WriteFile(submoduleReadme, []byte(modifySubmoduleAfterCommit), os.ModeSticky)
		require.NoError(t, err)

		build.JobResponse.AllowGitFetch = true
		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Your local changes to the following files would be overwritten by checkout")
		assert.NotContains(t, out, "Please commit your changes or stash them before you switch branches")
		assert.NotContains(t, out, "Aborting")
		assert.Contains(t, out, "Updating/initializing submodules...")
	})
}

func TestBuildWithoutDebugTrace(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		// The default build shouldn't have debug tracing enabled
		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotRegexp(t, `[^$] echo Hello World`, out)
	})
}
func TestBuildWithDebugTrace(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.JobVariable{Key: "CI_DEBUG_TRACE", Value: "true"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Regexp(t, `[^$] echo Hello World`, out)
	})
}

func TestBuildMultilineCommand(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)

	multilineBuild, err := common.GetMultilineBashBuild()
	assert.NoError(t, err)
	build, cleanup := newBuild(t, multilineBuild, "bash")
	defer cleanup()

	// The default build shouldn't have debug tracing enabled
	out, err := buildtest.RunBuildReturningOutput(t, build)
	assert.NoError(t, err)
	assert.NotContains(t, out, "bash")
	assert.Contains(t, out, "Hello World")
	assert.Contains(t, out, "collapsed multi-line command")
}

func TestBuildWithBrokenGitSSLCAInfo(t *testing.T) {
	skipOnGit17x(t)
	skipOnGit(t, ">= 2.10.2")

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteBrokenTLSBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.URL = "https://gitlab.com"

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.Error(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.NotContains(t, out, "Updating/initializing submodules")
	})
}

func TestBuildWithGoodGitSSLCAInfo(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteGitLabComTLSBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.URL = "https://gitlab.com"

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.Contains(t, out, "Updating/initializing submodules")
	})
}

// TestBuildWithGitSSLAndStrategyFetch describes issue https://gitlab.com/gitlab-org/gitlab-runner/issues/2991
func TestBuildWithGitSSLAndStrategyFetch(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteGitLabComTLSBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)

		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Fetching changes")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
		assert.Contains(t, out, "pre-clone-script")
	})
}

func TestBuildWithUntrackedDirFromPreviousBuild(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")

		err = os.MkdirAll(fmt.Sprintf("%s/.test", build.FullProjectDir()), 0644)
		require.NoError(t, err)

		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Removing .test/")
	})
}

func TestBuildChangesBranchesWhenFetchingRepo(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")

		// Another build using the same repo but different branch.
		build.GitInfo = common.GetLFSGitInfo(build.GitInfo.RepoURL)
		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Checking out 2371dd05 as add-lfs-object...")
	})
}

func TestBuildPowerShellCatchesExceptions(t *testing.T) {
	for _, shell := range []string{"powershell", "pwsh"} {
		t.Run(shell, func(t *testing.T) {
			helpers.SkipIntegrationTests(t, shell)

			successfulBuild, err := common.GetRemoteSuccessfulBuild()
			assert.NoError(t, err)
			build, cleanup := newBuild(t, successfulBuild, shell)
			defer cleanup()
			build.Variables = append(
				build.Variables,
				common.JobVariable{Key: "ErrorActionPreference", Value: "Stop"},
				common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"},
			)

			out, err := buildtest.RunBuildReturningOutput(t, build)
			assert.NoError(t, err)
			assert.Contains(t, out, "Created fresh repository")

			out, err = buildtest.RunBuildReturningOutput(t, build)
			assert.NoError(t, err)
			assert.NotContains(t, out, "Created fresh repository")
			assert.Regexp(t, "Checking out [a-f0-9]+ as", out)

			build.Variables = append(
				build.Variables,
				common.JobVariable{Key: "ErrorActionPreference", Value: "Continue"},
			)
			out, err = buildtest.RunBuildReturningOutput(t, build)
			assert.NoError(t, err)
			assert.NotContains(t, out, "Created fresh repository")
			assert.Regexp(t, "Checking out [a-f0-9]+ as", out)

			build.Variables = append(
				build.Variables,
				common.JobVariable{Key: "ErrorActionPreference", Value: "SilentlyContinue"},
			)
			out, err = buildtest.RunBuildReturningOutput(t, build)
			assert.NoError(t, err)
			assert.NotContains(t, out, "Created fresh repository")
			assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
		})
	}
}

func TestInteractiveTerminal(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)

	cases := []struct {
		app                string
		shell              string
		command            string
		expectedStatusCode int
	}{
		{
			app:                "bash",
			shell:              "bash",
			command:            "sleep 5",
			expectedStatusCode: http.StatusSwitchingProtocols,
		},
		{
			app:                "cmd.exe",
			shell:              "cmd",
			command:            "timeout 2",
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			app:                "powershell.exe",
			shell:              "powershell",
			command:            "Start-Sleep -s 2",
			expectedStatusCode: http.StatusInternalServerError,
		},
	}

	for _, c := range cases {
		t.Run(c.shell, func(t *testing.T) {
			helpers.SkipIntegrationTests(t, c.app)

			successfulBuild, err := common.GetLocalBuildResponse(c.command)
			require.NoError(t, err)
			build, cleanup := newBuild(t, successfulBuild, c.shell)
			defer cleanup()
			sess, err := session.NewSession(nil)
			build.Session = sess
			require.NoError(t, err)

			buildOut := make(chan string)

			go func() {
				buf := bytes.NewBuffer(nil)
				errRun := buildtest.RunBuildWithOptions(
					t,
					build,
					&common.Trace{Writer: buf},
					&common.Config{SessionServer: common.SessionServer{SessionTimeout: 2}},
				)
				require.NoError(t, errRun)

				buildOut <- buf.String()
			}()

			// Wait until the build starts.
			for build.Session.Mux() == nil {
				time.Sleep(10 * time.Millisecond)
			}

			srv := httptest.NewServer(build.Session.Mux())
			defer srv.Close()

			u := url.URL{
				Scheme: "ws",
				Host:   srv.Listener.Addr().String(),
				Path:   build.Session.Endpoint + "/exec",
			}
			headers := http.Header{
				"Authorization": []string{build.Session.Token},
			}
			conn, resp, err := websocket.DefaultDialer.Dial(u.String(), headers)
			assert.Equal(t, c.expectedStatusCode, resp.StatusCode)
			body, _ := ioutil.ReadAll(resp.Body)
			require.NoError(t, err, string(body))
			defer resp.Body.Close()

			defer func() {
				if conn != nil {
					conn.Close()
				}
			}()

			if c.expectedStatusCode == http.StatusSwitchingProtocols {
				_, message, err := conn.ReadMessage()
				assert.NoError(t, err)
				assert.NotEmpty(t, string(message))

				out := <-buildOut
				require.Contains(t, out, "Terminal is connected, will time out in 2s...")
				t.Log(out)
				return
			}

			out := <-buildOut
			require.NotContains(t, out, "Terminal is connected, will time out in 2s...")
			t.Log(out)
		})
	}
}

func TestBuildWithGitCleanFlags(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		jobResponse, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)

		build, cleanup := newBuild(t, jobResponse, shell)
		defer cleanup()

		build.Variables = append(
			build.Variables,
			common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"},
			common.JobVariable{Key: "GIT_CLEAN_FLAGS", Value: "-ffdx cleanup_file"},
		)

		// Run build and save file
		err = buildtest.RunBuild(t, build)
		require.NoError(t, err)

		excludedFilePath := filepath.Join(build.BuildDir, "excluded_file")
		cleanUpFilePath := filepath.Join(build.BuildDir, "cleanup_file")

		err = ioutil.WriteFile(excludedFilePath, []byte{}, os.ModePerm)
		require.NoError(t, err)
		err = ioutil.WriteFile(cleanUpFilePath, []byte{}, os.ModePerm)
		require.NoError(t, err)

		// Re-run build and ensure that file still exists
		err = buildtest.RunBuild(t, build)
		require.NoError(t, err)

		_, err = os.Stat(excludedFilePath)
		assert.NoError(t, err, "excluded_file does exist")
		_, err = os.Stat(cleanUpFilePath)
		assert.Error(t, err, "cleanup_file does not exist")
	})
}

func TestBuildFileVariablesRemoval(t *testing.T) {
	getJobResponse := func(t *testing.T, jobResponseRequester func() (common.JobResponse, error)) common.JobResponse {
		jobResponse, err := jobResponseRequester()
		require.NoError(t, err)

		return jobResponse
	}

	tests := map[string]struct {
		jobResponse common.JobResponse
	}{
		"succeeded job": {
			jobResponse: getJobResponse(t, common.GetSuccessfulBuild),
		},
		"failed job": {
			jobResponse: getJobResponse(t, common.GetFailedBuild),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			shellstest.OnEachShell(t, func(t *testing.T, shell string) {
				build, cleanup := newBuild(t, tt.jobResponse, shell)
				defer cleanup()

				testVariableName := "TEST_VARIABLE"

				build.Variables = append(
					build.Variables,
					common.JobVariable{Key: testVariableName, Value: "test", File: true},
				)

				_ = buildtest.RunBuild(t, build)

				tmpDir := fmt.Sprintf("%s.tmp", build.BuildDir)
				variableFile := filepath.Join(tmpDir, testVariableName)

				_, err := os.Stat(variableFile)
				assert.Error(t, err)
				assert.True(
					t,
					errors.Is(err, os.ErrNotExist),
					`Expected that os.Stat on the variable file will return the "doesn't exist" error`,
				)
			})
		})
	}
}

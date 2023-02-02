//go:build integration

package custom_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/executors/custom/command"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/session"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

var (
	testExecutorFile string
	testJobInfo      = common.JobInfo{
		Name:        "test job",
		Stage:       "test",
		ProjectID:   0,
		ProjectName: "test project",
	}
)

func TestMain(m *testing.M) {
	code := 1
	defer func() {
		os.Exit(code)
	}()

	fmt.Println("Compiling test executor")

	targetDir, err := os.MkdirTemp("", "test_executor")
	if err != nil {
		panic("Error on preparing tmp directory for test executor binary")
	}

	testExecutorFile = filepath.Join(targetDir, "main")
	testExecutorFile = buildtest.MustBuildBinary("testdata/test_executor/main.go", testExecutorFile)
	defer os.RemoveAll(targetDir)

	code = m.Run()
}

func newBuild(t *testing.T, jobResponse common.JobResponse, shell string) *common.Build {
	dir := t.TempDir()

	t.Log("Build directory:", dir)

	jobResponse.JobInfo = testJobInfo

	build := &common.Build{
		JobResponse: jobResponse,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				BuildsDir: filepath.Join(dir, "builds"),
				CacheDir:  filepath.Join(dir, "cache"),
				Executor:  "custom",
				Shell:     shell,
				Custom: &common.CustomConfig{
					ConfigExec:          testExecutorFile,
					ConfigArgs:          []string{shell, "config"},
					PrepareExec:         testExecutorFile,
					PrepareArgs:         []string{shell, "prepare"},
					RunExec:             testExecutorFile,
					RunArgs:             []string{shell, "run"},
					CleanupExec:         testExecutorFile,
					CleanupArgs:         []string{shell, "cleanup"},
					GracefulKillTimeout: timeoutInSeconds(10 * time.Second),
					ForceKillTimeout:    timeoutInSeconds(10 * time.Second),
				},
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
		Session: &session.Session{
			DisconnectCh: make(chan error),
			TimeoutCh:    make(chan error),
		},
	}

	return build
}

func timeoutInSeconds(duration time.Duration) *int {
	seconds := duration.Seconds()
	secondsInInt := int(seconds)

	return &secondsInInt
}

func TestBuildSuccess(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildScriptSections(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		if shell == "cmd" || shell == "pwsh" || shell == "powershell" {
			// support for pwsh and powershell tracked in https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28119
			t.Skip("CMD, pwsh, powershell not supported")
		}
		successfulBuild, err := common.GetSuccessfulBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		require.NoError(t, err)
		buildtest.RunBuildWithSections(t, build)
	})
}

func TestBuildSuccessRawVariable(t *testing.T) {
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

		successfulBuild, err := common.GetRemoteBuildResponse(test.command)
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

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

func TestBuildBuildFailure(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(build.Variables, common.JobVariable{
			Key:    "IS_BUILD_ERROR",
			Value:  "true",
			Public: true,
		})

		err = buildtest.RunBuild(t, build)
		assert.Error(t, err)
		var buildErr *common.BuildError
		assert.ErrorAs(t, err, &buildErr)
		assert.Equal(t, command.BuildFailureExitCode, buildErr.ExitCode)
	})
}

func TestBuildSystemFailure(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(build.Variables, common.JobVariable{
			Key:    "IS_SYSTEM_ERROR",
			Value:  "true",
			Public: true,
		})

		err = buildtest.RunBuild(t, build)
		assert.Error(t, err)
		var exitError *exec.ExitError
		assert.ErrorAs(t, err, &exitError)
		t.Log(err)
	})
}

func TestBuildUnknownFailure(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(build.Variables, common.JobVariable{
			Key:    "IS_UNKNOWN_ERROR",
			Value:  "true",
			Public: true,
		})

		err = buildtest.RunBuild(t, build)
		assert.Error(t, err)
		var errUnknownFailure *command.ErrUnknownFailure
		assert.ErrorAs(t, err, &errUnknownFailure)
		assert.Equal(t, 255, errUnknownFailure.ExitCode)
	})
}

func TestBuildCancel(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := newBuild(t, common.JobResponse{}, shell)

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

func TestBuildMasking(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := newBuild(t, common.JobResponse{}, shell)

		buildtest.RunBuildWithMasking(t, build.Runner, nil)
	})
}

func TestBuildWithGitStrategyCloneWithoutLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Runner.PostCloneScript = "echo post-clone-script"
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "clone"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")

		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
		assert.Contains(t, out, "pre-clone-script")
		assert.Contains(t, out, "post-clone-script")
	})
}

func TestBuildWithGitStrategyCloneNoCheckoutWithoutLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Runner.PostCloneScript = "echo post-clone-script"
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
		assert.Contains(t, out, "post-clone-script")
	})
}

func TestBuildWithGitSubmoduleStrategyRecursiveAndGitStrategyNone(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

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

func TestBuildWithoutDebugTrace(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		// The default build shouldn't have debug tracing enabled
		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotRegexp(t, `[^$] echo Hello World`, out)
	})
}

func TestBuildWithDebugTrace(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(build.Variables, common.JobVariable{Key: "CI_DEBUG_TRACE", Value: "true"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Regexp(t, `(>|[^$] )echo Hello World`, out)
	})
}

func TestBuildMultilineCommand(t *testing.T) {
	buildGenerators := map[string]func() (common.JobResponse, error){
		"bash":       common.GetMultilineBashBuild,
		"powershell": common.GetMultilineBashBuildPowerShell,
		"pwsh":       common.GetMultilineBashBuildPowerShell,
		"cmd":        common.GetMultilineBashBuildCmd,
	}

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		buildGenerator, ok := buildGenerators[shell]
		require.Truef(t, ok, "Missing build generator for shell %q", shell)

		multilineBuild, err := buildGenerator()
		require.NoError(t, err)

		build := newBuild(t, multilineBuild, shell)

		// The default build shouldn't have debug tracing enabled
		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "echo")
		assert.Contains(t, out, "Hello World")
		assert.Contains(t, out, "collapsed multi-line command")
	})
}

func TestBuildWithGoodGitSSLCAInfo(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		if shell == "cmd" {
			t.Skip("This test doesn't support Windows CMD (which is deprecated)")
		}

		successfulBuild, err := common.GetRemoteGitLabComTLSBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

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
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Runner.PostCloneScript = "echo post-clone-script"
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
		assert.Contains(t, out, "post-clone-script")
	})
}

func TestBuildChangesBranchesWhenFetchingRepo(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")

		// Another build using the same repo but different branch.
		build.GitInfo = common.GetLFSGitInfo(build.GitInfo.RepoURL)
		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Checking out c8f2a61d as detached HEAD (ref is add-lfs-object)...")
	})
}

func TestBuildPowerShellCatchesExceptions(t *testing.T) {
	for _, shell := range []string{"powershell", "pwsh"} {
		t.Run(shell, func(t *testing.T) {
			helpers.SkipIntegrationTests(t, shell)

			successfulBuild, err := common.GetRemoteSuccessfulBuild()
			require.NoError(t, err)

			build := newBuild(t, successfulBuild, shell)
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

func TestBuildOnCustomDirectory(t *testing.T) {
	commands := map[string]string{
		"bash":       "pwd",
		"powershell": "pwd",
		"pwsh":       "pwd",
	}

	tests := map[string]bool{
		"custom directory defined":     true,
		"custom directory not defined": false,
	}

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		if shell == "cmd" {
			t.Skip("This test doesn't support Windows CMD (which is deprecated)")
		}

		for testName, tt := range tests {
			t.Run(testName, func(t *testing.T) {
				cmd, ok := commands[shell]
				require.Truef(t, ok, "Missing command for shell %q", shell)

				dir := filepath.Join(os.TempDir(), "custom", "directory")
				expectedDirectory := filepath.Join(dir, "0")

				successfulBuild, err := common.GetSuccessfulBuild()
				require.NoError(t, err)

				successfulBuild.Steps[0].Script = common.StepScript{cmd}

				build := newBuild(t, successfulBuild, shell)

				if tt {
					build.Variables = append(build.Variables, common.JobVariable{
						Key:    "IS_RUN_ON_CUSTOM_DIR",
						Value:  dir,
						Public: true,
					})
				}

				out, err := buildtest.RunBuildReturningOutput(t, build)
				assert.NoError(t, err)

				if tt {
					assert.Contains(t, out, expectedDirectory)
				} else {
					assert.NotContains(t, out, expectedDirectory)
				}
			})
		}
	})
}

func TestBuildLogLimitExceeded(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := newBuild(t, common.JobResponse{}, shell)

		buildtest.RunBuildWithJobOutputLimitExceeded(t, build.Runner, nil)
	})
}

func TestBuildWithAccessToJobResponseFile(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		output, err := buildtest.RunBuildReturningOutput(t, build)
		require.NoError(t, err)

		assert.Contains(t, output, "job ID           => 0")
		assert.Contains(t, output, fmt.Sprintf("job name         => %s", testJobInfo.Name))
		assert.Contains(t, output, fmt.Sprintf("job stage        => %s", testJobInfo.Stage))
		assert.Contains(t, output, fmt.Sprintf("job project ID   => %d", testJobInfo.ProjectID))
		assert.Contains(t, output, fmt.Sprintf("job project name => %s", testJobInfo.ProjectName))
	})
}

func TestCleanupProjectGitClone(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		buildtest.RunBuildWithCleanupGitClone(t, build)
	})
}

func TestCleanupProjectGitFetch(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		untrackedFilename := "untracked"

		successfulBuild, err := common.GetRemoteBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(untrackedFilename, "", "")...,
		)
		require.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		buildtest.RunBuildWithCleanupGitFetch(t, build, untrackedFilename)
	})
}

func TestCleanupProjectGitSubmoduleNormal(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		untrackedFile := "untracked"
		untrackedSubmoduleFile := "untracked_submodule"

		successfulBuild, err := common.GetRemoteBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(untrackedFile, untrackedSubmoduleFile, "")...,
		)
		require.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		buildtest.RunBuildWithCleanupNormalSubmoduleStrategy(t, build, untrackedFile, untrackedSubmoduleFile)
	})
}

func TestCleanupProjectGitSubmoduleRecursive(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		untrackedFile := "untracked"
		untrackedSubmoduleFile := "untracked_submodule"
		untrackedSubSubmoduleFile := "untracked_submodule_submodule"

		successfulBuild, err := common.GetRemoteBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(
				untrackedFile,
				untrackedSubmoduleFile,
				untrackedSubSubmoduleFile,
			)...,
		)

		require.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		buildtest.RunBuildWithCleanupNormalSubmoduleStrategy(t, build, untrackedFile, untrackedSubmoduleFile)
	})
}

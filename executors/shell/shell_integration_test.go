//go:build integration

package shell_test

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors/shell"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/test"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
	"gitlab.com/gitlab-org/gitlab-runner/session"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

const integrationTestShellExecutor = "shell-integration-test"

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

	shell.RegisterExecutor(integrationTestShellExecutor, path)

	code = m.Run()
}

func gitInDir(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	return cmd.Output()
}

func skipOnGit(t *testing.T, constraints string) {
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
		t.Skipf("Git %q found, skipping the test", constraints)
	}
}

// This is an alternative implementation to t.TempDir() since that wouldn't work on Windows due to long file paths.
func tempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err, "creating temp dir for test %q", t.Name())

	t.Cleanup(func() {
		err := os.RemoveAll(dir)
		require.NoError(t, err, "removing temp dir %q for test %q", dir, t.Name())
	})

	return dir
}

func newBuild(t *testing.T, getBuildResponse spec.Job, shell string) *common.Build {
	dir := tempDir(t)

	t.Logf("setting 'builds_dir' to %q", dir)

	build := &common.Build{
		Job: getBuildResponse,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				BuildsDir:           dir,
				Executor:            integrationTestShellExecutor,
				Shell:               shell,
				GracefulKillTimeout: func(i int) *int { return &i }(5),
				ForceKillTimeout:    func(i int) *int { return &i }(1),
				Cache:               &cacheconfig.Config{},
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

func TestBuildSuccess(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		buildtest.WithEachFeatureFlag(t, func(t *testing.T, setup buildtest.BuildSetupFn) {
			successfulBuild, err := common.GetSuccessfulBuild()
			assert.NoError(t, err)
			build := newBuild(t, successfulBuild, shell)

			setup(t, build)

			err = buildtest.RunBuild(t, build)
			assert.NoError(t, err)
		}, featureflags.UsePowershellPathResolver)
	})
}

func TestBuildPassingEnvsMultistep(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := newBuild(t, spec.Job{}, shell)

		buildtest.RunBuildWithPassingEnvsMultistep(t, build.Runner, nil)
	})
}

func TestBuildPassingEnvsJobIsolation(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := newBuild(t, spec.Job{}, shell)

		buildtest.RunBuildWithPassingEnvsJobIsolation(t, build.Runner, nil)
	})
}

func TestMultistepBuild(t *testing.T) {
	successfulBuild, err := common.GetRemoteSuccessfulMultistepBuild()
	require.NoError(t, err)

	failingScriptBuild, err := common.GetRemoteFailingMultistepBuild(spec.StepNameScript)
	require.NoError(t, err)

	failingReleaseBuild, err := common.GetRemoteFailingMultistepBuild("release")
	require.NoError(t, err)

	failingAfterScriptBuild, err := common.GetRemoteFailingMultistepBuild(spec.StepNameAfterScript)
	require.NoError(t, err)

	tests := map[string]struct {
		jobResponse             spec.Job
		afterScriptIgnoreErrors bool
		expectedOutput          []string
		unwantedOutput          []string
		errExpected             bool
	}{
		"Successful build with release and after_script step": {
			jobResponse:             successfulBuild,
			afterScriptIgnoreErrors: true,
			expectedOutput: []string{
				"echo Hello World",
				"echo Release",
				"echo After Script",
			},
			errExpected: false,
		},
		"Failure on script step Release is skipped After script runs": {
			jobResponse:             failingScriptBuild,
			afterScriptIgnoreErrors: true,
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
			jobResponse:             failingReleaseBuild,
			afterScriptIgnoreErrors: true,
			expectedOutput: []string{
				"echo Hello World",
				"echo Release",
				"echo After Script",
			},
			errExpected: true,
		},
		"Failure in after script step (ignored).": {
			jobResponse:             failingAfterScriptBuild,
			afterScriptIgnoreErrors: true,
			expectedOutput: []string{
				"echo Hello World",
				"echo Release",
				"echo After Script",
			},
			errExpected: false,
		},
		"Failure in after script step (not ignored).": {
			jobResponse:             failingAfterScriptBuild,
			afterScriptIgnoreErrors: false,
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
				build := newBuild(t, tt.jobResponse, shell)
				if !tt.afterScriptIgnoreErrors {
					build.Variables = append(build.Variables, spec.Variable{
						Key:   "AFTER_SCRIPT_IGNORE_ERRORS",
						Value: "false",
					})
				}

				out, err := buildtest.RunBuildReturningOutput(t, build)

				for _, output := range tt.expectedOutput {
					assert.Contains(t, out, output)
				}

				for _, output := range tt.unwantedOutput {
					assert.NotContains(t, out, output)
				}

				if tt.errExpected {
					var buildErr *common.BuildError
					assert.ErrorAs(t, err, &buildErr)
					assert.Equal(t, 1, buildErr.ExitCode)
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
		"state on failure": {
			fail: true,
			assert: func(t *testing.T, err error, build *common.Build, out string) {
				assert.Error(t, err)
				assert.Contains(t, out, "CI_JOB_STATUS=failed")
				assert.Equal(t, common.BuildRunRuntimeFailed, build.CurrentState())
			},
		},
		"state on success": {
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
				multistepBuildScript, err := common.GetRemoteFailingMultistepBuildPrintVars(shell, tc.fail, "CI_JOB_STATUS")
				require.NoError(t, err)

				build := newBuild(t, multistepBuildScript, shell)

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

		build := newBuild(t, successfulBuild, shell)

		value := "$VARIABLE$WITH$DOLLARS$$"
		build.Variables = append(build.Variables, spec.Variable{
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
		build := newBuild(t, spec.Job{}, shell)

		buildtest.RunBuildWithCancel(t, build.Runner, nil)
	})
}

func TestBuildWithExecutorCancel(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := newBuild(t, spec.Job{}, shell)

		buildtest.RunBuildWithExecutorCancel(t, build.Runner, nil)
	})
}

func TestBuildMasking(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := newBuild(t, spec.Job{}, shell)

		buildtest.RunBuildWithMasking(t, build.Runner, nil)
	})
}

func TestBuildMaskingProxyExec(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := newBuild(t, spec.Job{}, shell)

		buildtest.RunBuildWithMaskingProxyExec(t, build.Runner, nil)
	})
}

func TestBuildExpandedFileVariable(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := newBuild(t, spec.Job{}, shell)
		buildtest.RunBuildWithExpandedFileVariable(t, build.Runner, nil)
	})
}

func TestBuildWithIndexLock(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)

		build.Job.AllowGitFetch = true
		err = os.WriteFile(build.BuildDir+"/.git/index.lock", []byte{}, os.ModeSticky)
		require.NoError(t, err)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildWithShallowLock(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"},
		)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)

		err = os.WriteFile(build.BuildDir+"/.git/shallow.lock", []byte{}, os.ModeSticky)
		require.NoError(t, err)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildWithHeadLock(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)

		build.Job.AllowGitFetch = true
		err = os.WriteFile(build.BuildDir+"/.git/HEAD.lock", []byte{}, os.ModeSticky)
		require.NoError(t, err)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildWithLeftoverConfigLock(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)

		build.Job.AllowGitFetch = true
		err = os.WriteFile(build.BuildDir+"/.git/config.lock", []byte{}, os.ModeSticky)
		require.NoError(t, err)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildWithGitLFSHook(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)

		gitLFSPostCheckoutHook := "#!/bin/sh\necho 'running git lfs hook' >&2\nexit 2\n"

		err = os.MkdirAll(build.BuildDir+"/.git/hooks/", 0755)
		require.NoError(t, err)
		err = os.WriteFile(build.BuildDir+"/.git/hooks/post-checkout", []byte(gitLFSPostCheckoutHook), 0777)
		require.NoError(t, err)
		build.Job.AllowGitFetch = true

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildWithRefLock(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)

		refDir := build.BuildDir + "/.git/refs/remotes/origin/"
		lockfile := "main.lock"
		build.Job.AllowGitFetch = true
		err = os.MkdirAll(refDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(refDir+lockfile, []byte{}, os.ModeSticky)
		require.NoError(t, err)

		err = buildtest.RunBuild(t, build)
		assert.NoError(t, err)
		assert.NoFileExists(t, refDir+lockfile)
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
		build := newBuild(t, successfulBuild, shell)

		build.Runner.PreGetSourcesScript = "echo pre-clone-script"
		build.Runner.PostGetSourcesScript = "echo post-clone-script"
		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_STRATEGY", Value: "none"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "pre-clone-script")
		assert.NotContains(t, out, "Created fresh repository")
		assert.NotContains(t, out, "Fetching changes")
		assert.NotContains(t, out, "post-clone-script")
		assert.Contains(t, out, "Skipping Git repository setup")
	})
}

func TestBuildWithGitStrategyNoneWithLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulLFSBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_STRATEGY", Value: "none"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Created fresh repository")
		assert.NotContains(t, out, "Fetching changes")
		assert.Contains(t, out, "Skipping Git repository setup")
		assertLFSFileNotPresent(t, build)
	})
}

func TestBuildWithGitStrategyEmptyWithoutLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Runner.PreGetSourcesScript = "echo pre-clone-script"
		build.Runner.PostGetSourcesScript = "echo post-clone-script"
		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_STRATEGY", Value: "empty"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Skipping Git repository setup and creating an empty build directory")
		assert.NotContains(t, out, "Created fresh repository")
		assert.NotContains(t, out, "Fetching changes")

		testFilePath := filepath.Join(build.BuildDir, "test.txt")
		err = os.WriteFile(testFilePath, []byte{}, os.ModePerm)
		require.NoError(t, err)

		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Skipping Git repository setup and creating an empty build directory")
		assert.NotContains(t, out, "Created fresh repository")
		assert.NotContains(t, out, "Fetching changes")
		assert.NotContains(t, out, "pre-clone-script")
		assert.NotContains(t, out, "post-clone-script")

		_, err = os.Stat(testFilePath)
		assert.Error(t, err)
		assert.ErrorIs(t, err, os.ErrNotExist, "build directory not cleaned before next build")
	})
}

func TestBuildWithGitStrategyEmptyWithLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulLFSBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_STRATEGY", Value: "empty"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Skipping Git repository setup and creating an empty build directory")
		assert.NotContains(t, out, "Created fresh repository")
		assert.NotContains(t, out, "Fetching changes")
		assertLFSFileNotPresent(t, build)

		testFilePath := filepath.Join(build.BuildDir, "test.txt")
		err = os.WriteFile(testFilePath, []byte{}, os.ModePerm)
		require.NoError(t, err)

		out, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Skipping Git repository setup and creating an empty build directory")
		assert.NotContains(t, out, "Created fresh repository")
		assert.NotContains(t, out, "Fetching changes")
		assert.NotContains(t, out, "pre-clone-script")
		assert.NotContains(t, out, "post-clone-script")
		assertLFSFileNotPresent(t, build)

		_, err = os.Stat(testFilePath)
		assert.Error(t, err)
		assert.ErrorIs(t, err, os.ErrNotExist, "build directory not cleaned before next build")
	})
}

func TestBuildWithGitStrategyFetchWithoutLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Runner.PreGetSourcesScript = "echo pre-clone-script"
		build.Runner.PostGetSourcesScript = "echo post-clone-script"
		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"})

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

func TestBuildWithGitStrategyFetchWithLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"})

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
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_LFS_SKIP_SMUDGE", Value: "1", Public: true},
			spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"},
		)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
		assertLFSFileNotPresent(t, build)

		build.GitInfo = common.GetLFSGitInfo(build.GitInfo.RepoURL)
		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_LFS_SKIP_SMUDGE", Value: "1", Public: true},
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
		build := newBuild(t, successfulBuild, shell)

		build.Runner.PreGetSourcesScript = "echo pre-clone-script"
		build.Runner.PostGetSourcesScript = "echo post-clone-script"
		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"},
			spec.Variable{Key: "GIT_CHECKOUT", Value: "false"},
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
		assert.Contains(t, out, "post-clone-script")
	})
}

func TestBuildWithGitStrategyFetchNoCheckoutWithLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulLFSBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"},
			spec.Variable{Key: "GIT_CHECKOUT", Value: "false"},
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
		build := newBuild(t, successfulBuild, shell)

		build.Runner.PreGetSourcesScript = "echo pre-clone-script"
		build.Runner.PostGetSourcesScript = "echo post-clone-script"
		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_STRATEGY", Value: "clone"})

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

func TestBuildWithGitStrategyCloneWithLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulLFSBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_STRATEGY", Value: "clone"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assertLFSFileDownloaded(t, build)
	})
}

func TestBuildWithGitStrategyCloneWithUserDisabledLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulLFSBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_STRATEGY", Value: "clone"},
			spec.Variable{Key: "GIT_LFS_SKIP_SMUDGE", Value: "1", Public: true},
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
		build := newBuild(t, successfulBuild, shell)

		build.Runner.PreGetSourcesScript = "echo pre-clone-script"
		build.Runner.PostGetSourcesScript = "echo post-clone-script"
		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_STRATEGY", Value: "clone"},
			spec.Variable{Key: "GIT_CHECKOUT", Value: "false"},
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

func TestBuildWithGitStrategyCloneNoCheckoutWithLFS(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulLFSBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_STRATEGY", Value: "clone"},
			spec.Variable{Key: "GIT_CHECKOUT", Value: "false"},
		)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Created fresh repository")
		assert.Contains(t, out, "Skipping Git checkout")
		assertLFSFileNotPresent(t, build)
	})
}

func TestBuildWithSubmoduleLFSPullsLFSObject(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)
		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"},
			spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"},
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
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)
		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"},
			spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"},
			spec.Variable{Key: "GIT_LFS_SKIP_SMUDGE", Value: "1", Public: true},
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
				build := newBuild(t, successfulBuild, shell)

				build.Variables = append(
					build.Variables,
					spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "none"},
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

func TestBuildWithGitSubmodulePaths(t *testing.T) {
	// Some of these fail on earlier versions of git
	// We can just skip it since we pass them directly to git and don't care for version support
	skipOnGit(t, "< 1.9")

	tests := map[string]struct {
		paths                   string
		expectedBuildError      bool
		expectedSubmoduleExists map[string]bool
	}{
		"include submodule": {
			paths:                   "gitlab-grack",
			expectedBuildError:      false,
			expectedSubmoduleExists: map[string]bool{"gitlab-grack": true, "simplegit": false},
		},
		"exclude submodule": {
			paths:                   ":(exclude)gitlab-grack",
			expectedBuildError:      false,
			expectedSubmoduleExists: map[string]bool{"gitlab-grack": false, "simplegit": true},
		},
		"include multiple submodule": {
			paths:                   "gitlab-grack simplegit",
			expectedBuildError:      false,
			expectedSubmoduleExists: map[string]bool{"gitlab-grack": true, "simplegit": true},
		},
		"exclude multiple submodule": {
			paths:                   ":(exclude)gitlab-grack :(exclude)simplegit",
			expectedBuildError:      false,
			expectedSubmoduleExists: map[string]bool{"gitlab-grack": false, "simplegit": false},
		},
		"ex/include multiple submodule": {
			paths:                   ":(exclude)gitlab-grack simplegit",
			expectedBuildError:      false,
			expectedSubmoduleExists: map[string]bool{"gitlab-grack": false, "simplegit": true},
		},
		"exclude submodule with single space": {
			paths:                   ":(exclude) gitlab-grack",
			expectedBuildError:      true,
			expectedSubmoduleExists: map[string]bool{"gitlab-grack": false, "simplegit": false},
		},
		"exclude submodule with multiple spaces": {
			paths:                   ":(exclude)  gitlab-grack",
			expectedBuildError:      true,
			expectedSubmoduleExists: map[string]bool{"gitlab-grack": false, "simplegit": false},
		},
		"exclude submodule with space between all statements": {
			paths:                   ": (exclude) gitlab-grack",
			expectedBuildError:      true,
			expectedSubmoduleExists: map[string]bool{"gitlab-grack": false, "simplegit": false},
		},
		"exclude submodule invalid": {
			paths:                   "::::(exclude) gitlab-grack",
			expectedBuildError:      true,
			expectedSubmoduleExists: map[string]bool{"gitlab-grack": false, "simplegit": false},
		},
		"empty": {
			paths:                   "    ",
			expectedBuildError:      false,
			expectedSubmoduleExists: map[string]bool{"gitlab-grack": true, "simplegit": true},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			shellstest.OnEachShell(t, func(t *testing.T, shell string) {
				successfulBuild, err := common.GetSuccessfulBuild()
				assert.NoError(t, err)

				build := newBuild(t, successfulBuild, shell)

				build.Variables = append(
					build.Variables,
					spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"},
					spec.Variable{Key: "GIT_SUBMODULE_PATHS", Value: tt.paths},
				)

				out, err := buildtest.RunBuildReturningOutput(t, build)
				if tt.expectedBuildError {
					assert.Error(t, err)
					return
				}

				assert.NoError(t, err)

				assert.NotContains(t, out, "Skipping Git submodules setup")
				assert.Contains(t, out, "Updating/initializing submodules...")

				for subModule, shouldExist := range tt.expectedSubmoduleExists {
					_, err = os.Stat(filepath.Join(build.BuildDir, subModule, ".git"))
					if shouldExist {
						require.NoError(t, err, "Submodule %v should have been initialized", subModule)
					} else {
						require.Error(t, err, "Submodule %v should not have been initialized", subModule)
					}
				}
			})
		})
	}
}

func TestBuildWithGitSubmoduleStrategyNormal(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Skipping Git submodules setup")
		assert.Contains(t, out, "Updating/initializing submodules...")
		assert.NotContains(t, out, "Updating/initializing submodules recursively...")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", ".git"))
		assert.NoError(t, err, "Submodule should have been initialized")

		_, err = os.Stat(filepath.Join(build.BuildDir, ".git", "modules", "gitlab-grack", "shallow"))
		assert.Error(t, err, "Submodule should not have been shallow cloned")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", "tests", "example", ".git"))
		assert.Error(t, err, "The submodule's submodule should not have been initialized")
	})
}

func TestBuildWithGitSubmoduleStrategyNormalAndGitSubmoduleDepth(t *testing.T) {
	skipOnGit(t, "< 1.9")

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"},
			spec.Variable{Key: "GIT_SUBMODULE_DEPTH", Value: "1"},
		)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Skipping Git submodules setup")
		assert.NotContains(t, out, "Updating/initializing submodules...")
		assert.Contains(t, out, "Updating/initializing submodules with git depth set to 1...")
		assert.NotContains(t, out, "Updating/initializing submodules recursively...")
		assert.NotContains(t, out, "Updating/initializing submodules recursively with git depth set to 1...")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", ".git"))
		assert.NoError(t, err, "Submodule should have been initialized")

		_, err = os.Stat(filepath.Join(build.BuildDir, ".git", "modules", "gitlab-grack", "shallow"))
		assert.NoError(t, err, "Submodule should have been shallow cloned")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", "tests", "example", ".git"))
		assert.Error(t, err, "The submodule's submodule should not have been initialized")
	})
}

func TestBuildWithGitSubmoduleStrategyRecursive(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Skipping Git submodules setup")
		assert.NotContains(t, out, "Updating/initializing submodules...")
		assert.Contains(t, out, "Updating/initializing submodules recursively...")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", ".git"))
		assert.NoError(t, err, "Submodule should have been initialized")

		_, err = os.Stat(filepath.Join(build.BuildDir, ".git", "modules", "gitlab-grack", "shallow"))
		assert.Error(t, err, "Submodule should not have been shallow cloned")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", "tests", "example", ".git"))
		assert.NoError(t, err, "The submodule's submodule should have been initialized")

		_, err = os.Stat(filepath.Join(build.BuildDir, ".git", "modules", "gitlab-grack", "tests", "example", "shallow"))
		assert.Error(t, err, "The submodule's submodule should not have been shallow cloned")
	})
}

func TestBuildWithGitSubmoduleStrategyRecursiveAndGitSubmoduleDepth(t *testing.T) {
	skipOnGit(t, "< 1.9")

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"},
			spec.Variable{Key: "GIT_SUBMODULE_DEPTH", Value: "1"},
		)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Skipping Git submodules setup")
		assert.NotContains(t, out, "Updating/initializing submodules...")
		assert.NotContains(t, out, "Updating/initializing submodules with git depth set to 1...")
		assert.NotContains(t, out, "Updating/initializing submodules recursively...")
		assert.Contains(t, out, "Updating/initializing submodules recursively with git depth set to 1...")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", ".git"))
		assert.NoError(t, err, "Submodule should have been initialized")

		_, err = os.Stat(filepath.Join(build.BuildDir, ".git", "modules", "gitlab-grack", "shallow"))
		assert.NoError(t, err, "Submodule should have been shallow cloned")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", "tests", "example", ".git"))
		assert.NoError(t, err, "The submodule's submodule should have been initialized")

		_, err = os.Stat(filepath.Join(build.BuildDir, ".git", "modules", "gitlab-grack", "modules", "tests", "example", "shallow"))
		assert.NoError(t, err, "The submodule's submodule should have been shallow cloned")
	})
}

func TestBuildWithGitFetchSubmoduleStrategyRecursive(t *testing.T) {
	tests := map[string]struct {
		cleanGitConfig         *bool
		expectFreshRepoMessage bool
	}{
		"no git cleanup": {
			// shell executor defaults to not clean up git configs
			expectFreshRepoMessage: false,
		},
		"git cleanup explicitly enabled": {
			cleanGitConfig:         &[]bool{true}[0],
			expectFreshRepoMessage: true,
		},
		"git cleanup explicitly disabled": {
			cleanGitConfig:         &[]bool{false}[0],
			expectFreshRepoMessage: false,
		},
	}

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		for name, test := range tests {
			t.Run(name, func(t *testing.T) {
				successfulBuild, err := common.GetSuccessfulBuild()
				assert.NoError(t, err)
				build := newBuild(t, successfulBuild, shell)

				build.Variables = append(
					build.Variables,
					spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"},
					spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"},
				)
				build.Runner.RunnerSettings.CleanGitConfig = test.cleanGitConfig

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
				err = os.WriteFile(excludedFilePath, []byte{}, os.ModePerm)
				require.NoError(t, err)

				// Run second build, to run fetch.
				out, err = buildtest.RunBuildReturningOutput(t, build)
				assert.NoError(t, err)

				checkFreshRepoMessage := assert.NotContains
				if test.expectFreshRepoMessage {
					checkFreshRepoMessage = assert.Contains
				}
				checkFreshRepoMessage(t, out, "Created fresh repository")

				assert.Contains(t, out, "Removing excluded_file")
			})
		}
	})
}

func TestBuildGitCloneStrategyCleanup(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)
		build.Runner.RunnerSettings.Shell = shell

		buildtest.RunBuildWithCleanupGitClone(t, build)

		_, err = os.Stat(build.FullProjectDir())
		assert.Error(t, err, "cleanup should have removed the entire build directory")
	})
}

func TestBuildGitFetchStrategyCleanup(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		untrackedFilename := "untracked"
		successfulBuild, err := common.GetLocalBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(untrackedFilename, "", "")...,
		)

		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)
		build.Runner.RunnerSettings.Shell = shell

		buildtest.RunBuildWithCleanupGitFetch(t, build, untrackedFilename)

		_, err = os.Stat(build.BuildDir)
		assert.NoError(t, err, "cleanup should not delete entire build directory")
		_, err = os.Stat(filepath.Join(build.BuildDir, untrackedFilename))
		assert.Error(t, err, "cleanup should have removed the untracked file")
	})
}

func TestBuildGitFetchStrategySubmoduleNormalCleanup(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		untrackedFilename, untrackedFileInSubmodule := "untracked", "untracked_in_submodule"
		successfulBuild, err := common.GetLocalBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(untrackedFilename, untrackedFileInSubmodule, "")...,
		)

		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		buildtest.RunBuildWithCleanupNormalSubmoduleStrategy(t, build, untrackedFilename, untrackedFileInSubmodule)

		_, err = os.Stat(build.BuildDir)
		assert.NoError(t, err, "cleanup should not delete entire build directory")
		_, err = os.Stat(filepath.Join(build.BuildDir, untrackedFilename))
		assert.Error(t, err, "cleanup should have removed untracked file in main repository")
		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", untrackedFileInSubmodule))
		assert.Error(t, err, "cleanup should have removed untracked file in submodule")
	})
}

func TestBuildGitFetchStrategySubmoduleRecursiveCleanup(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		untrackedFile := "untracked_file"
		untrackedFileInSubmodule := "untracked_file_in_submodule"
		untrackedFileInSubSubmodule := "untracked_file_in_sub_submodule"

		successfulBuild, err := common.GetLocalBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(untrackedFile, untrackedFileInSubmodule, untrackedFileInSubSubmodule)...,
		)

		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		buildtest.RunBuildWithCleanupRecursiveSubmoduleStrategy(
			t,
			build,
			untrackedFile,
			untrackedFileInSubmodule,
			untrackedFileInSubSubmodule,
		)

		_, err = os.Stat(build.BuildDir)
		assert.NoError(t, err, "cleanup should not delete entire build directory")
		_, err = os.Stat(filepath.Join(build.BuildDir, untrackedFile))
		assert.Error(t, err, "cleanup should have removed untracked file in main repository")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", untrackedFileInSubmodule))
		assert.Error(t, err, "cleanup should have removed untracked file in submodule")

		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", "tests", "example", untrackedFileInSubSubmodule))
		assert.Error(t, err, "cleanup should have removed untracked file in submodule's submodule")
	})
}

func TestBuildGitFetchStrategyFallback(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetLocalBuildResponse()
		assert.NoError(t, err)

		build := newBuild(t, successfulBuild, shell)

		// Perform a successful build that doesn't fetch submodules.
		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"},
		)
		_, err = buildtest.RunBuildReturningOutput(t, build)
		require.NoError(t, err)

		// Add a file in a submodule. This situation can arise naturally and in
		// this case there is no combination of `git clean` or `git for-each-submodule`
		// that can remove the file. Deleting all tracked files via `git rm -rf .`
		// will work though.
		testTxt := filepath.Join(build.BuildDir, "gitlab-grack", "test.txt")
		err = os.WriteFile(testTxt, []byte("content"), 0600)
		require.NoError(t, err)

		// Now do another build but this time try to fetch the submodules.
		// Updating the submodules will fail because `test.txt` exists, and
		// `git clean` won't remove it because it is in a submodule.
		//
		// But since we set `GET_SOURCES_ATTEMPTS` to 2, before trying for
		// the second time it will delete all the tracked and untracked files
		// so the second attempt will succeed.
		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GET_SOURCES_ATTEMPTS", Value: "2"},
			spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"},
		)
		out, err := buildtest.RunBuildReturningOutput(t, build)
		err = buildtest.RunBuild(t, build)
		require.NoError(t, err)
		require.Contains(t, out, "Deleting tracked and untracked files...")

		// Double check that the submodule was successfully checked out and `test.txt` is gone.
		_, err = os.Stat(filepath.Join(build.BuildDir, "gitlab-grack", "README.md"))
		require.NoError(t, err, "submodule was not checked out correctly")
		_, err = os.Stat(testTxt)
		require.Error(t, err, "fetch error did not result in a clean clone")
	})
}

func TestBuildWithGitSubmoduleStrategyInvalid(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "invalid"})

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
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_STRATEGY", Value: "none"},
			spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"},
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

func TestBuildWithGitSubmoduleStrategyRecursiveAndGitStrategyEmpty(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_STRATEGY", Value: "empty"},
			spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"},
		)

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Created fresh repository")
		assert.NotContains(t, out, "Fetching changes")
		assert.Contains(t, out, "Skipping Git repository setup and creating an empty build directory")
		assert.NotContains(t, out, "Updating/initializing submodules...")
		assert.NotContains(t, out, "Updating/initializing submodules recursively...")
		assert.Contains(t, out, "Skipping Git submodules setup")
	})
}

func TestBuildWithGitSubmoduleModified(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Updating/initializing submodules...")

		submoduleDir := filepath.Join(build.BuildDir, "gitlab-grack")
		submoduleReadme := filepath.Join(submoduleDir, "README.md")

		// modify submodule and commit
		modifySubmoduleBeforeCommit := "committed change"
		err = os.WriteFile(submoduleReadme, []byte(modifySubmoduleBeforeCommit), os.ModeSticky)
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
		err = os.WriteFile(submoduleReadme, []byte(modifySubmoduleAfterCommit), os.ModeSticky)
		require.NoError(t, err)

		build.Job.AllowGitFetch = true
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
		build := newBuild(t, successfulBuild, shell)

		// The default build shouldn't have debug tracing enabled
		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)

		switch shell {
		case "pwsh", "powershell":
			assert.NotRegexp(t, `>\s?echo Hello World`, out)
		default:
			assert.NotRegexp(t, `[^$] echo Hello World`, out)
		}
	})
}

func TestBuildWithDebugTrace(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Variables = append(build.Variables, spec.Variable{Key: "CI_DEBUG_TRACE", Value: "true"})

		out, err := buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		switch shell {
		case "pwsh", "powershell":
			assert.Regexp(t, `>\s?echo Hello World`, out)
		default:
			assert.Regexp(t, `[^$] echo Hello World`, out)
		}
	})
}

func TestBuildMultilineCommand(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)

	multilineBuild, err := common.GetMultilineBashBuild()
	assert.NoError(t, err)
	build := newBuild(t, multilineBuild, "bash")

	// The default build shouldn't have debug tracing enabled
	out, err := buildtest.RunBuildReturningOutput(t, build)
	assert.NoError(t, err)
	assert.Contains(t, out, "Hello World")
	assert.Contains(t, out, "collapsed multi-line command")
}

func TestBuildWithGoodGitSSLCAInfo(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteGitLabComTLSBuild()
		assert.NoError(t, err)
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
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)

		build.Runner.PreGetSourcesScript = "echo pre-clone-script"
		build.Runner.PostGetSourcesScript = "echo post-clone-script"
		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"})

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

func TestBuildWithUntrackedDirFromPreviousBuild(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteSuccessfulBuild()
		assert.NoError(t, err)
		build := newBuild(t, successfulBuild, shell)
		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"})

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
		build := newBuild(t, successfulBuild, shell)
		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"})

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
	tests := map[string]struct {
		cleanGitConfig         *bool
		expectFreshRepoMessage bool
	}{
		"no git cleanup": {
			// shell executor defaults to not clean up git configs
			expectFreshRepoMessage: false,
		},
		"git cleanup explicitly enabled": {
			cleanGitConfig:         &[]bool{true}[0],
			expectFreshRepoMessage: true,
		},
		"git cleanup explicitly disabled": {
			cleanGitConfig:         &[]bool{false}[0],
			expectFreshRepoMessage: false,
		},
	}

	for _, shell := range []string{"powershell", "pwsh"} {
		t.Run(shell, func(t *testing.T) {
			for name, test := range tests {
				t.Run(name, func(t *testing.T) {
					helpers.SkipIntegrationTests(t, shell)

					successfulBuild, err := common.GetRemoteSuccessfulBuild()
					assert.NoError(t, err)
					build := newBuild(t, successfulBuild, shell)
					build.Variables = append(
						build.Variables,
						spec.Variable{Key: "ErrorActionPreference", Value: "Stop"},
						spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"},
					)
					build.Runner.RunnerSettings.CleanGitConfig = test.cleanGitConfig

					checkFreshRepoMessage := assert.NotContains
					if test.expectFreshRepoMessage {
						checkFreshRepoMessage = assert.Contains
					}

					out, err := buildtest.RunBuildReturningOutput(t, build)
					assert.NoError(t, err)
					assert.Contains(t, out, "Created fresh repository")

					out, err = buildtest.RunBuildReturningOutput(t, build)
					assert.NoError(t, err)
					checkFreshRepoMessage(t, out, "Created fresh repository")
					assert.Regexp(t, "Checking out [a-f0-9]+ as", out)

					build.Variables = append(
						build.Variables,
						spec.Variable{Key: "ErrorActionPreference", Value: "Continue"},
					)
					out, err = buildtest.RunBuildReturningOutput(t, build)
					assert.NoError(t, err)
					checkFreshRepoMessage(t, out, "Created fresh repository")
					assert.Regexp(t, "Checking out [a-f0-9]+ as", out)

					build.Variables = append(
						build.Variables,
						spec.Variable{Key: "ErrorActionPreference", Value: "SilentlyContinue"},
					)
					out, err = buildtest.RunBuildReturningOutput(t, build)
					assert.NoError(t, err)
					checkFreshRepoMessage(t, out, "Created fresh repository")
					assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
				})
			}
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
			build := newBuild(t, successfulBuild, c.shell)
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

			srv := httptest.NewServer(build.Session.Handler())
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
			body, _ := io.ReadAll(resp.Body)
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

		build := newBuild(t, jobResponse, shell)

		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"},
			spec.Variable{Key: "GIT_CLEAN_FLAGS", Value: "-ffdx cleanup_file"},
		)

		// Run build and save file
		err = buildtest.RunBuild(t, build)
		require.NoError(t, err)

		excludedFilePath := filepath.Join(build.BuildDir, "excluded_file")
		cleanUpFilePath := filepath.Join(build.BuildDir, "cleanup_file")

		err = os.WriteFile(excludedFilePath, []byte{}, os.ModePerm)
		require.NoError(t, err)
		err = os.WriteFile(cleanUpFilePath, []byte{}, os.ModePerm)
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

func TestSanitizeGitDirectory(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		jobResponse, err := common.GetLocalBuildResponse(
			"git remote set-url origin /tmp/some/invalid/directory",
		)
		require.NoError(t, err, "getting job response")

		build := newBuild(t, jobResponse, shell)

		build.Variables = append(
			build.Variables,
			spec.Variable{Key: "GIT_STRATEGY", Value: "fetch"},
			spec.Variable{Key: featureflags.EnableJobCleanup, Value: "true"},
		)

		build.Runner.RunnerSettings.CleanGitConfig = &[]bool{true}[0]

		err = buildtest.RunBuild(t, build)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(build.FullProjectDir(), ".git", "config"))
		assert.True(t, errors.Is(err, os.ErrNotExist))

		out, err := gitInDir(build.BuildDir, "init", "--template", filepath.Join(build.BuildDir, "git-template"))
		assert.NoError(t, err)
		assert.Contains(t, string(out), "Reinitialized existing Git repository")

		_, err = gitInDir(build.BuildDir, "fsck")
		assert.NoError(t, err)
	})
}

func TestBuildFileVariablesRemoval(t *testing.T) {
	getJobResponse := func(t *testing.T, jobResponseRequester func() (spec.Job, error)) spec.Job {
		jobResponse, err := jobResponseRequester()
		require.NoError(t, err)

		return jobResponse
	}

	tests := map[string]struct {
		jobResponse spec.Job
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
				buildtest.WithEachFeatureFlag(t, func(t *testing.T, setup buildtest.BuildSetupFn) {
					build := newBuild(t, tt.jobResponse, shell)

					testVariableName := "TEST_VARIABLE"

					build.Variables = append(
						build.Variables,
						spec.Variable{Key: testVariableName, Value: "test", File: true},
					)

					setup(t, build)

					_ = buildtest.RunBuild(t, build)

					tmpDir := fmt.Sprintf("%s.tmp", build.BuildDir)
					variableFile := filepath.Join(tmpDir, testVariableName)

					_, err := os.Stat(variableFile)
					assert.Error(t, err)
					assert.ErrorIs(t, err, os.ErrNotExist)
				}, featureflags.UsePowershellPathResolver)
			})
		})
	}
}

func TestBuildLogLimitExceeded(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := newBuild(t, spec.Job{}, shell)

		buildtest.RunBuildWithJobOutputLimitExceeded(t, build.Runner, nil)
	})
}

func TestBuildInvokeBinaryHelper(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		buildtest.WithEachFeatureFlag(t, func(t *testing.T, setup buildtest.BuildSetupFn) {
			successfulBuild, err := common.GetRemoteSuccessfulBuild()
			require.NoError(t, err)

			build := newBuild(t, successfulBuild, shell)

			setup(t, build)

			dir := t.TempDir()

			build.Runner.RunnerSettings.BuildsDir = filepath.Join(dir, "build")
			build.Runner.RunnerSettings.CacheDir = filepath.Join(dir, "cache")

			build.Cache = append(build.Cache, spec.Cache{
				Key:    "cache",
				Paths:  []string{"*"},
				Policy: spec.CachePolicyPullPush,
			})

			out, err := buildtest.RunBuildReturningOutput(t, build)
			assert.NoError(t, err)
			assert.NotContains(t, out, "Extracting cache is disabled.")
			assert.NotContains(t, out, "Creating cache is disabled.")
			assert.Contains(t, out, "Created cache")
		}, featureflags.UsePowershellPathResolver)
	})
}

func TestGitCloneOrFetch(t *testing.T) {
	if !test.CommandVersionIsAtLeast(t, "2.49.0", "git", "version") {
		t.Skip("git version is not 2.49.0")
	}

	tests := map[string]struct {
		revision    string
		sha         string
		depth       int
		assertError bool
	}{
		"main branch matching sha": {
			revision: "main",
			sha:      "1ea27a9695f80d7816d9e8ce025d9b2df83d0dd7",
		},
		"main refs matching sha": {
			revision: "refs/heads/main",
			sha:      "1ea27a9695f80d7816d9e8ce025d9b2df83d0dd7",
		},
		"main refs matching sha with depth 1": {
			revision: "refs/heads/main",
			sha:      "1ea27a9695f80d7816d9e8ce025d9b2df83d0dd7",
			depth:    1,
		},
		"main refs previous sha with depth 1": {
			revision:    "refs/heads/main",
			sha:         "035c3a26fadbc7bd2f4101c84812a8b6e722f562",
			depth:       1,
			assertError: true,
		},
		"main refs wrong sha": {
			revision:    "refs/heads/main",
			sha:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			assertError: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()
			shellstest.OnEachShell(t, func(t *testing.T, shell string) {
				t.Parallel()
				buildtest.WithEachFeatureFlag(t, func(t *testing.T, setup buildtest.BuildSetupFn) {
					t.Parallel()

					jobResponse, err := common.GetRemoteBuildResponse(`echo "Hello World"`)

					assert.NoError(t, err)
					build := newBuild(t, jobResponse, shell)

					setup(t, build)

					build.GitInfo.Ref = tt.revision
					build.GitInfo.Sha = tt.sha
					if tt.depth > 0 {
						build.GitInfo.Depth = tt.depth
					}

					out, err := buildtest.RunBuildReturningOutput(t, build)
					if tt.assertError {
						assert.Error(t, err)
						return
					}

					assert.NoError(t, err)

					if build.IsFeatureFlagOn(featureflags.UseGitNativeClone) {
						assert.Contains(t, out, "Cloning into")
					} else {
						assert.Contains(t, out, "Fetching changes")
					}
					checkingOutHEAD := fmt.Sprintf("Checking out %s as detached HEAD", tt.sha[:8])
					assert.Contains(t, out, checkingOutHEAD)
				}, featureflags.UseGitNativeClone)
			})
		})
	}
}

func TestBuildPwshHandlesSyntaxErrors(t *testing.T) {
	helpers.SkipIntegrationTests(t, shells.SNPwsh)

	successfulBuild, err := common.GetLocalBuildResponse("some syntax error\nWrite-Output $PSVersionTable")
	require.NoError(t, err)

	build := newBuild(t, successfulBuild, shells.SNPwsh)

	out, err := buildtest.RunBuildReturningOutput(t, build)
	assert.Error(t, err)
	assert.NotContains(t, out, "PSEdition")
}

func TestBuildPwshHandlesScriptEncodingCorrectly(t *testing.T) {
	helpers.SkipIntegrationTests(t, shells.SNPwsh)

	successfulBuild, err := common.GetLocalBuildResponse("echo $Env:GL_Test1 | Format-Hex")
	require.NoError(t, err)

	build := newBuild(t, successfulBuild, shells.SNPwsh)

	build.Variables = append(build.Variables, spec.Variable{
		Key:   "GL_Test1",
		Value: "∅",
		Raw:   true,
	})

	out, err := buildtest.RunBuildReturningOutput(t, build)
	assert.NoError(t, err)
	assert.Contains(t, out, "E2 88 85")
}

func TestBuildScriptSections(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		if shell == "pwsh" || shell == "powershell" {
			// support for pwsh and powershell tracked in https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28119
			t.Skip("pwsh and powershell not supported")
		}
		build := newBuild(t, spec.Job{}, shell)

		successfulBuild, err := common.GetSuccessfulMultilineCommandBuild()
		require.NoError(t, err)
		build.Job = successfulBuild
		build.Runner.RunnerSettings.Shell = shell
		buildtest.RunBuildWithSections(t, build)
	})
}

func TestCloneBranchExpansion(t *testing.T) {
	const branch = "$(id)"

	_ = common.RunLocalRepoGitCommand("checkout", "-b", branch)
	defer func() {
		_ = common.RunLocalRepoGitCommand("branch", "-D", branch)
	}()

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := newBuild(t, spec.Job{}, shell)

		successfulBuild, err := common.GetLocalBuildResponse()
		require.NoError(t, err)

		build.Job = successfulBuild
		build.GitInfo.Ref = branch
		build.Runner.RunnerSettings.Shell = shell

		out, err := buildtest.RunBuildReturningOutput(t, build)
		t.Log(out)
		assert.NoError(t, err)
		assert.Contains(t, out, fmt.Sprintf("(ref is %s)", branch))
		assert.NotContains(t, out, "uid=")
		assert.Contains(t, out, "Job succeeded")
	})
}

func TestBuildCacheHelper(t *testing.T) {
	tests := []struct {
		name    string
		buildFn func(dir string, build *common.Build)

		expectedCacheCreated bool
	}{
		{
			name: "cache settings provided, job cache provided",
			buildFn: func(dir string, build *common.Build) {
				build.Runner.RunnerSettings.Cache = &cacheconfig.Config{}
				build.Cache = append(build.Cache, spec.Cache{
					Key:    "cache",
					Paths:  []string{"*"},
					Policy: spec.CachePolicyPullPush,
				})
			},
			expectedCacheCreated: true,
		},
		{
			name: "no cache settings defined, job cache provided",
			buildFn: func(dir string, build *common.Build) {
				build.Runner.RunnerSettings.Cache = nil
				build.Cache = append(build.Cache, spec.Cache{
					Key:    "cache",
					Paths:  []string{"*"},
					Policy: spec.CachePolicyPullPush,
				})
			},
			expectedCacheCreated: true,
		},
		{
			name: "cache settings provided, no job cache provided",
			buildFn: func(dir string, build *common.Build) {
				build.Runner.RunnerSettings.Cache = &cacheconfig.Config{}
				build.Cache = nil
			},
		},
		{
			name: "no cache settings provided, no job cache provided",
			buildFn: func(dir string, build *common.Build) {
				build.Runner.RunnerSettings.Cache = nil
				build.Cache = nil
			},
		},
	}

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				successfulBuild, err := common.GetRemoteSuccessfulBuild()
				require.NoError(t, err)

				build := newBuild(t, successfulBuild, shell)

				dir := t.TempDir()
				build.Runner.RunnerSettings.BuildsDir = filepath.Join(dir, "build")
				build.Runner.RunnerSettings.CacheDir = filepath.Join(dir, "cache")

				tc.buildFn(dir, build)

				out, err := buildtest.RunBuildReturningOutput(t, build)
				assert.NoError(t, err)

				if tc.expectedCacheCreated {
					assert.Contains(t, out, "Created cache")
				} else {
					assert.NotContains(t, out, "Created cache")
				}
			})
		}
	})
}

func TestBuildWithCustomClonePath(t *testing.T) {
	const gitClonePath = "$CI_BUILDS_DIR/go/src/gitlab.com/gitlab-org/repo"
	someTrue, someFalse := true, false

	tests := map[string]struct {
		customBuildDirConfig common.CustomBuildDir
		buildsDirConfig      string
		expectedError        string
	}{
		// shell executor defaults to not allowing custom build dirs, except when explicitly enabled
		"not set": {
			expectedError: "setting GIT_CLONE_PATH is not allowed, enable `custom_build_dir` feature",
		},
		"explicitly disabled": {
			customBuildDirConfig: common.CustomBuildDir{Enabled: &someFalse},
			expectedError:        "setting GIT_CLONE_PATH is not allowed, enable `custom_build_dir` feature",
		},
		"explicitly enabled, default builds dir": {
			customBuildDirConfig: common.CustomBuildDir{Enabled: &someTrue},
		},
		"explicitly enabled, custom builds dir": {
			customBuildDirConfig: common.CustomBuildDir{Enabled: &someTrue},
			buildsDirConfig:      "/foo/bar/baz",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			shellstest.OnEachShell(t, func(t *testing.T, shell string) {
				var cmd string
				switch shell {
				case "powershell", "pwsh":
					cmd = "Get-Item -Path " + gitClonePath
				default:
					cmd = "ls -al " + gitClonePath
				}

				jobResponse, err := common.GetRemoteBuildResponse(cmd)
				require.NoError(t, err)

				build := newBuild(t, jobResponse, shell)

				build.Runner.CustomBuildDir = tt.customBuildDirConfig
				build.Runner.BuildsDir += tt.buildsDirConfig

				build.Variables = append(
					build.Variables,
					spec.Variable{
						Key:   "GIT_CLONE_PATH",
						Value: gitClonePath,
					},
				)

				err = buildtest.RunBuild(t, build)
				if tt.expectedError == "" {
					assert.NoError(t, err)
				} else {
					assert.ErrorContains(t, err, tt.expectedError)
					var buildErr *common.BuildError
					assert.ErrorAs(t, err, &buildErr)
				}
			})
		})
	}
}

const (
	// a repo with a mixed bag of submodules: relative, private, public
	repoURLWithSubmodules = "https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/submodules/mixed-submodules-test"
	repoShaWithSubmodules = "0a1093ff08de939dbd1625689d86deef18126a74"
)

func TestCredSetup(t *testing.T) {
	const (
		markerForBuild      = "#build# "
		markerForHelper     = "#helper# "
		markerPreGetSource  = "pre_get_source"
		markerPostGetSource = "post_get_source"
	)

	listGitConfig := func(t *testing.T, shell, prefix string) string {
		switch shell {
		case shells.Bash:
			return fmt.Sprintf(`git config -l | sed 's/^/%s/g'`, prefix)
		case shells.SNPwsh, shells.SNPowershell:
			return fmt.Sprintf(`(git config -l) -replace '^','%s'`, prefix)
		default:
			t.Fatalf("shell %s not supported", shell)
		}
		return ""
	}
	getGitCred := func(t *testing.T, shell, prefix string) string {
		username := cmp.Or(os.Getenv("GITLAB_TEST_USERNAME"), "gitlab-ci-token")

		switch shell {
		case shells.Bash:
			return fmt.Sprintf(`echo -e "protocol=https\nhost=gitlab.com\nusername=%s" | git -c credential.interactive=never credential fill | sed 's/^/#%s# /g';`, username, prefix)
		case shells.SNPwsh, shells.SNPowershell:
			return fmt.Sprintf("$GitStdin = \"protocol=https`nhost=gitlab.com`nusername=%s\"; "+
				"$GitStdinFile = Join-Path ${CI_BUILDS_DIR} 'git_get_cred_stdin.txt' ; "+
				"$GitStdoutFile = Join-Path ${CI_BUILDS_DIR} 'git_get_cred_stdout.txt' ; "+
				"If(Test-Path $GitStdoutFile) { Remove-Item $GitStdoutFile } ; "+
				"[System.IO.File]::WriteAllText($GitStdinFile, $GitStdin) ; "+
				"Start-Process -FilePath 'git' -ArgumentList 'credential','fill' -RedirectStandardInput $GitStdinFile -RedirectStandardOutput $GitStdoutFile -NoNewWindow -Wait; "+
				"(Get-Content -Path $GitStdoutFile) -replace '^','#%s# ';", username, prefix)
		default:
			t.Fatalf("shell %s not supported", shell)
		}
		return ""
	}
	setGitCred := func(t *testing.T, shell string) string {
		username := cmp.Or(os.Getenv("GITLAB_TEST_USERNAME"), "gitlab-ci-token")
		const password = "fake_password"

		switch shell {
		case shells.Bash:
			return fmt.Sprintf(`echo -e "protocol=https\nhost=gitlab.com\nusername=%s\npassword=%s" | git credential approve;`, username, password)
		case shells.SNPwsh, shells.SNPowershell:
			return fmt.Sprintf("$GitStdin = \"protocol=https`nhost=gitlab.com`nusername=%s`npassword=%s\"; "+
				"$GitStdinFile = Join-Path ${CI_BUILDS_DIR} 'git_set_cred_stdin.txt' ; "+
				"[System.IO.File]::WriteAllText($GitStdinFile, $GitStdin) ; "+
				"Start-Process -FilePath 'git' -ArgumentList 'credential','approve' -RedirectStandardInput $GitStdinFile -NoNewWindow -Wait;", username, password)
		default:
			t.Fatalf("shell %s not supported", shell)
		}
		return ""
	}
	extractGitConfig := func(blob, prefix string) string {
		out := []string{}
		// Regex to match timestamp format + prefix: YYYY-MM-DDTHH:MM:SS.microsZ WORD prefix
		pattern := fmt.Sprintf(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z\s+\w+\s+%s`, regexp.QuoteMeta(prefix))
		timestampPattern := regexp.MustCompile(pattern)

		for _, line := range strings.Split(blob, "\n") {
			if timestampPattern.MatchString(line) {
				l := timestampPattern.ReplaceAllString(line, "")
				out = append(out, l)
			}
		}
		return strings.Join(out, "\n")
	}

	tests := []struct {
		gitUrlsWithoutTokens bool
		validator            func(t *testing.T, out string, remoteURL string, cachedGitCreds string, token string)
	}{
		{
			gitUrlsWithoutTokens: true,
			validator: func(t *testing.T, out string, remoteURL string, cachedGitCreds string, token string) {
				assert.NotContains(t, remoteURL, "@", "the remote URL should not contain any auth data")
				assert.NotContains(t, remoteURL, "gitlab-ci-token", "the remote URL should not contain the token user")

				remoteHost := onlyHost(t, remoteURL)

				// git cred helper is setup in the helper & build container
				for _, marker := range []string{markerForHelper, markerForBuild} {
					gitConfig := extractGitConfig(out, marker)
					assert.Contains(t, gitConfig, "credential."+remoteHost+".username=gitlab-ci-token", "should contain a username setting")
					assert.Contains(t, gitConfig, "credential."+remoteHost+".helper=!", "should contain a credential helper")
					assert.NotContains(t, gitConfig, "remote.origin.url=https://gitlab-ci-token:", "the origin URL should not contain any auth data")
					assert.NotContains(t, gitConfig, "url."+withMaskedPassword(t, remoteURL)+".insteadof="+remoteHost, "should not have an insteadOf URL with auth data")
					assert.Contains(t, gitConfig, "include.path=", "should include config file with creds")
				}

				// pre get-source: gitlab-ci-token password comes from the 1st global credential helper
				// post get-source: globally defined credential.helpers should not be run, we can fetch the current token with the local cred helper, but it is not cached
				assert.Contains(t, out, "#pre_get_source# password=fake_password", "before getting sources (& setting up the local cred helper), previously used creds are used")
				assert.Contains(t, out, "#post_get_source# password=[MASKED]", "after getting sources (& setting up the local cred helper), only its creds are used")
				content, err := os.ReadFile(cachedGitCreds)
				require.NoError(t, err)
				assert.Contains(t, "https://gitlab-ci-token:fake_password@gitlab.com\n", string(content), "per-build cred helper caches creds from before getting source (& setting up the local cred helper)")
			},
		},
		{
			gitUrlsWithoutTokens: false,
			validator: func(t *testing.T, out string, remoteURL string, cachedGitCreds string, token string) {
				assert.Contains(t, remoteURL, "@", "the remote URL should contain auth data")
				assert.Contains(t, remoteURL, "gitlab-ci-token", "remote URL should contain the token user")

				remoteHost := onlyHost(t, remoteURL)

				// git cred helper is neither setup in the helper or build container
				for _, marker := range []string{markerForHelper, markerForBuild} {
					gitConfig := extractGitConfig(out, marker)
					assert.NotContains(t, gitConfig, "credential."+remoteHost+".username=gitlab-ci-token", "should not contain a username setting")
					assert.NotContains(t, gitConfig, "credential."+remoteHost+".helper=!", "should not contain a credential helper")
					assert.NotContains(t, gitConfig, "remote.origin.url=https://gitlab-ci-token:", "should contain the origin URL including auth data")
					assert.Contains(t, gitConfig, "url."+withMaskedPassword(t, remoteURL)+".insteadof="+remoteHost, "should have an insteadOf URL with auth data")
					assert.Contains(t, gitConfig, "include.path=", "should include config file with creds")
				}

				// pre get-source: gitlab-ci-token password comes from the 1st global credential helper
				// post get-source: we can fetch creds from the global helpers, because we didn't set the local empty cred helper: feature flag is OFF
				// the CI_JOB_TOKEN is expected to be cached
				assert.Contains(t, out, "#pre_get_source# password=fake_password", "before getting sources, previously used creds are used")
				assert.Contains(t, out, "#post_get_source# password=[MASKED]", "after getting sources, the token is cached")
				content, err := os.ReadFile(cachedGitCreds)
				require.NoError(t, err)
				assert.Contains(t, fmt.Sprintf("https://gitlab-ci-token:%s@gitlab.com\n", token), string(content), "per-build cred helper caches the most recently used creds")
			},
		},
	}

	setupCachingCredHelpers(t)
	orgURL, err := url.Parse(repoURLWithSubmodules)
	require.NoError(t, err, "parsing original repo url")
	setInvalidGitCreds(t, *orgURL)

	gitStrategies := map[string]struct {
		featureFlags map[string]bool
		jobVariables spec.Variables
	}{
		"fetch": {},
		"clone": {
			jobVariables: spec.Variables{{Key: "GIT_STRATEGY", Value: "clone"}},
		},
		"nativeClone": {
			featureFlags: map[string]bool{featureflags.UseGitNativeClone: true},
			jobVariables: spec.Variables{{Key: "GIT_STRATEGY", Value: "clone"}},
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s:%t", featureflags.GitURLsWithoutTokens, test.gitUrlsWithoutTokens), func(t *testing.T) {
			t.Parallel()

			for gitStrategyName, gitStrategy := range gitStrategies {
				t.Run("GIT_STRATEGY:"+gitStrategyName, func(t *testing.T) {
					t.Parallel()

					shellstest.OnEachShell(t, func(t *testing.T, shell string) {
						t.Parallel()

						helpers.SkipIntegrationTests(t, shell)

						jobResponse, err := common.GetRemoteBuildResponse(listGitConfig(t, shell, markerForBuild))
						require.NoError(t, err)

						jobResponse.GitInfo.RepoURL = repoURLWithSubmodules
						jobResponse.GitInfo.Sha = repoShaWithSubmodules
						token, _ := buildtest.InjectJobTokenFromEnv(t, &jobResponse)

						jobResponse.Hooks = append(jobResponse.Hooks, spec.Hook{
							Name:   spec.HookPreGetSourcesScript,
							Script: spec.StepScript{setGitCred(t, shell), getGitCred(t, shell, markerPreGetSource)},
						})

						jobResponse.Hooks = append(jobResponse.Hooks, spec.Hook{
							Name:   spec.HookPostGetSourcesScript,
							Script: spec.StepScript{listGitConfig(t, shell, markerForHelper), getGitCred(t, shell, markerPostGetSource)},
						})

						jobResponse.Variables = append(jobResponse.Variables,
							spec.Variable{Key: "GIT_TRACE", Value: "1"},
							spec.Variable{Key: "GIT_CURL_VERBOSE", Value: "1"},
							spec.Variable{Key: "GIT_TRANSFER_TRACE", Value: "1"},
							// CI_DEBUG_TRACE causes shell tracing which can corrupt git config output with -race
							// spec.Variable{Key: "CI_DEBUG_TRACE", Value: "1"},
							spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"},
							spec.Variable{Key: "GIT_SUBMODULE_FORCE_HTTPS", Value: "1"},
							spec.Variable{Key: "CI_SERVER_HOST", Value: "gitlab.com"},
						)
						jobResponse.Variables = append(jobResponse.Variables, gitStrategy.jobVariables...)

						build := newBuild(t, jobResponse, shell)

						build.Runner.RunnerCredentials.URL = "https://gitlab.com/"

						buildtest.SetBuildFeatureFlag(build, featureflags.GitURLsWithoutTokens, test.gitUrlsWithoutTokens)
						for k, v := range gitStrategy.featureFlags {
							buildtest.SetBuildFeatureFlag(build, k, v)
						}

						out, err := buildtest.RunBuildReturningOutput(t, build)
						assert.NoError(t, err)

						assert.NotContains(t, out, token, "should not contain the token")

						remoteURL, err := build.GetRemoteURL()
						assert.NoError(t, err, "getting build's remote URL")

						// cached creds from the 1st helper created in setupTestCredHelpers
						cachedCreds := filepath.Join(build.Runner.BuildsDir, "git-credentials")
						test.validator(t, out, remoteURL.String(), cachedCreds, token)
					})
				})
			}
		})
	}
}

func TestSubmoduleAutoBump(t *testing.T) {
	const (
		// See: https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/submodules/mixed-submodules-branches/-/commit/b557eadceba20d40c6e10b274a1437e88051a4fd
		repoURL = "https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/submodules/mixed-submodules-branches"
		repoSha = "b557eadceba20d40c6e10b274a1437e88051a4fd"
	)

	// We'll just check out a couple of submodules and expect them to be checked out at specific revisions.
	expectedSubmoduleShas := map[string]string{
		// tip of `main`
		"private-repo-relative-main-branch-behind": "c17b10c540ab191766605db226af3d4e02f7c244",
		// tip of `non-default-branch`
		"private-repo-relative-non-default-branch-behind": "86ada27b869b34132b7e9d4f1e0bc732b6e223d3",
	}

	if test.CommandVersionIsAtLeast(t, "2.40.0", "git", "version") {
		// Older git versions default to not pick up the remote's default branch, but default to `origin/master`.
		// For these versions this just won't work, without explicitly setting the branch in `.gitmodules`.
		// Unfortunately, on the hosted windows runners we currently have git v2.23.0.windows1, so we need to skip this case
		// until we run a version we know supports that.
		//
		// Tested versions (did not bisect all versions, just some):
		// - ⚠ defaults to `origin/master`
		//   - git v2.23.0.windows1
		// - ✔ uses remote's default branch
		//   - v2.40.0.windows.1
		//   - v2.43.0.windows.1
		//   - v2.48.1.windows.1
		//   - v2.43.0 (ubuntu)

		// tip of default branch `orphaned-branch`
		expectedSubmoduleShas["private-repo-relative-default-branch-behind"] = "76be4b4f04c27a186a706908d3e9e884ccded543"
	}

	submodules := slices.Collect(maps.Keys(expectedSubmoduleShas))

	for _, gitUrlsWithoutTokens := range []bool{true, false} {
		name := fmt.Sprintf("gitUrlsWithoutTokens:%t", gitUrlsWithoutTokens)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			shellstest.OnEachShell(t, func(t *testing.T, shell string) {
				t.Parallel()

				if runtime.GOOS != shells.OSWindows && shell == shells.SNPowershell {
					t.Skip("powershell is not supported on non-windows platforms")
				}

				jobResponse, err := common.GetRemoteSuccessfulBuild()
				require.NoError(t, err)

				jobResponse.GitInfo.RepoURL = repoURL
				jobResponse.GitInfo.Sha = repoSha
				buildtest.InjectJobTokenFromEnv(t, &jobResponse)

				jobResponse.Variables = append(jobResponse.Variables,
					spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"},
					spec.Variable{Key: "GIT_SUBMODULE_UPDATE_FLAGS", Value: "--remote"},
					spec.Variable{Key: "GIT_SUBMODULE_PATHS", Value: strings.Join(submodules, " ")},
					spec.Variable{Key: "GIT_SUBMODULE_FORCE_HTTPS", Value: "1"},
					spec.Variable{Key: "CI_SERVER_HOST", Value: "gitlab.com"},
				)
				jobResponse.Hooks = append(jobResponse.Hooks, spec.Hook{
					Name:   "pre_get_sources_script",
					Script: spec.StepScript{"git version"},
				})

				build := newBuild(t, jobResponse, shell)

				build.Runner.RunnerCredentials.URL = "https://gitlab.com/"

				buildtest.SetBuildFeatureFlag(build, featureflags.GitURLsWithoutTokens, gitUrlsWithoutTokens)

				_, err = buildtest.RunBuildReturningOutput(t, build)
				assert.NoError(t, err)

				for submodule, expectedSha := range expectedSubmoduleShas {
					submoduleDir := filepath.Join(build.BuildDir, submodule)
					cmd := exec.Command("git", "rev-parse", "HEAD")
					cmd.Dir = submoduleDir

					actualSha, err := cmd.CombinedOutput()
					assert.NoError(t, err, "getting HEAD of %s", submodule)

					// this is not important for the test, but just to give more context in the error message and help with
					// debugging
					readmeName := "README.md"
					readmeContent, err := os.ReadFile(filepath.Join(submoduleDir, readmeName))
					if err != nil {
						readmeContent = []byte("ReadError: " + err.Error())
					}

					assert.Equal(t,
						expectedSha, string(bytes.Trim(actualSha, "\n\r")),
						"wrong rev for HEAD of %q\n----[ %s content ]----\n%s\n----", submodule, readmeName, readmeContent,
					)
				}
			})
		})
	}
}

func TestBuildWithCleanGitConfig(t *testing.T) {
	// only update a couple of submodules, to make the test a bit faster
	submodules := []string{"private-repo-ssh", "public-repo-relative"}
	require.GreaterOrEqual(t, len(submodules), 1, "must manage/update at least one submodule")

	assertFilesAreCleaned := func(t *testing.T, buildDir string) {
		dirs := []string{
			filepath.Join(buildDir, ".git"),
			filepath.Join(buildDir, "..", "mixed-submodules-test.tmp", "git-template"),
		}
		for _, m := range submodules {
			dirs = append(dirs, filepath.Join(buildDir, ".git", "modules", m))
		}
		for _, d := range dirs {
			assert.DirExists(t, d)
			assert.NoFileExists(t, filepath.Join(d, "config"))
			assert.NoDirExists(t, filepath.Join(d, "hooks"))
		}
	}

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		t.Parallel()

		jobResponse, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)

		jobResponse.Variables = append(jobResponse.Variables,
			spec.Variable{Key: "GIT_SUBMODULE_PATHS", Value: strings.Join(submodules, " ")},
			spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: string(common.SubmoduleRecursive)},
			spec.Variable{Key: "GIT_SUBMODULE_FORCE_HTTPS", Value: "1"},
			spec.Variable{Key: "CI_SERVER_HOST", Value: "gitlab.com"},
		)
		jobResponse.GitInfo.RepoURL = repoURLWithSubmodules
		jobResponse.GitInfo.Sha = repoShaWithSubmodules
		buildtest.InjectJobTokenFromEnv(t, &jobResponse)

		build := newBuild(t, jobResponse, shell)
		build.Runner.RunnerCredentials.URL = "https://gitlab.com/"
		build.Runner.RunnerSettings.CleanGitConfig = &[]bool{true}[0]

		_, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assertFilesAreCleaned(t, build.BuildDir)

		// run a second build to ensure submodules still work, even though we blew away their git config.
		_, err = buildtest.RunBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assertFilesAreCleaned(t, build.BuildDir)
	})
}

func TestGitIncludePaths(t *testing.T) {
	th := testOSHelper(runtime.GOOS)

	th.Parallel(t)

	const (
		repoURL = "https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/submodules/mixed-submodules-branches"
		repoSha = "b557eadceba20d40c6e10b274a1437e88051a4fd"
	)

	submodules := []string{
		"private-repo-git",
		"private-repo-relative",
	}

	assertIncludePaths := func(t *testing.T, expectedIncludes []string, buildDir string) {
		gitConfig := filepath.Join(buildDir, ".git", "config")

		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		cmd := exec.Command("git", "config", "--file", gitConfig, "--get-all", "include.path")
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		err := cmd.Run()
		require.NoError(t, err, "getting git 'include.path' settings\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())

		actualIncludes := strings.FieldsFunc(stdout.String(), func(r rune) bool {
			return r == '\n' || r == '\r'
		})

		for i, p := range actualIncludes {
			actualIncludes[i] = test.NormalizePath(p)
		}

		assert.Equal(t, expectedIncludes, actualIncludes, `unexpected "include.path"s`)
	}

	expectedIncludes := func(build *common.Build, pwd string, addIncludes ...string) []string {
		tmpProjectDir := test.NormalizePath(filepath.Join(pwd, build.BuildDir, "..", "mixed-submodules-branches.tmp"))

		// the main config, with insteadOfs
		includes := []string{filepath.Join(tmpProjectDir, ".gitlab-runner.ext.conf")}

		return append(includes, addIncludes...)
	}

	buildsDirPathOverrides := map[string]*string{
		"absolute": nil,
		"relative": &[]string{""}[0],
		// "relative-var": &[]string{"$PWD"}[0], // This is not supported
	}

	for name, buildsDirOverride := range buildsDirPathOverrides {
		t.Run("builds_dir:"+name, func(t *testing.T) {
			th.Parallel(t)

			for _, tokenFromEnv := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s:%t", featureflags.GitURLsWithoutTokens, tokenFromEnv), func(t *testing.T) {
					th.Parallel(t)

					shellstest.OnEachShell(t, func(t *testing.T, shell string) {
						buildtest.WithEachFeatureFlag(t, func(t *testing.T, setup buildtest.BuildSetupFn) {
							th.Parallel(t)

							jobResponse, err := common.GetSuccessfulBuild()
							require.NoError(t, err)

							jobResponse.GitInfo.RepoURL = repoURL
							jobResponse.GitInfo.Sha = repoSha
							buildtest.InjectJobTokenFromEnv(t, &jobResponse)

							build := newBuild(t, jobResponse, shell)
							buildtest.SetBuildFeatureFlag(build, featureflags.GitURLsWithoutTokens, tokenFromEnv)
							build.Runner.RunnerSettings.CleanGitConfig = &[]bool{false}[0]
							build.Variables.Set(spec.Variables{
								// {Key: "GIT_TRACE", Value: "2"},
								// {Key: "CI_DEBUG_TRACE", Value: "true"},
								{Key: "GIT_STRATEGY", Value: "fetch"},
							}...)
							setupForSubmoduleClone(build, "gitlab.com", submodules)

							var pwd string
							if buildsDirOverride != nil {
								var relBuildsDir string
								pwd, relBuildsDir = th.RelativeTempDir(t, "builds dir *")
								relBuildsDir = filepath.Join(*buildsDirOverride, relBuildsDir)
								t.Logf("overwriting 'builds_dir' to %q (in %q)", relBuildsDir, pwd)
								build.Runner.BuildsDir = relBuildsDir
							}

							randomInclude := "/some/random\\include/file"
							build.Runner.PostGetSourcesScript = fmt.Sprintf("git config --local --add include.path '%s'", randomInclude)

							for i := range 2 {
								name := fmt.Sprintf("run:%d", i)
								t.Run(name, func(t *testing.T) {
									_, err = buildtest.RunBuildReturningOutput(t, build)
									assert.NoError(t, err)
									expectedIncludes := expectedIncludes(build, pwd, slices.Repeat([]string{randomInclude}, i+1)...)
									assertIncludePaths(t, expectedIncludes, build.BuildDir)
								})
							}
						}, featureflags.UsePowershellPathResolver)
					})
				})
			}
		})
	}
}

func setupForSubmoduleClone(build *common.Build, serverHostname string, submodules []string) {
	build.Variables.Set(spec.Variables{
		{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"},
		{Key: "GIT_SUBMODULE_FORCE_HTTPS", Value: "1"},
		{Key: "CI_SERVER_HOST", Value: serverHostname},
	}...)

	if len(submodules) > 0 {
		build.Variables.Set(spec.Variable{
			Key: "GIT_SUBMODULE_PATHS", Value: strings.Join(submodules, " "),
		})
	}

	build.Runner.RunnerCredentials.URL = fmt.Sprintf("https://%s/", serverHostname)
}

// testOSHelper abstracts away some differences on how we want to run the tests on different OSs.
type testOSHelper string

// Parallel runs tests in Parallel, if not running on windows.
// We can't run in parallel on windows, because of the difference in [RelativeTempDir].
func (th testOSHelper) Parallel(t *testing.T) {
	switch th {
	case "windows":
		t.Logf("not using t.Parallel() because OS is %s", th)
	default:
		t.Parallel()
	}
}

// RelativeTempDir creates a temporary directory in $PWD, and returns $PWD and the relative path from there to this
// temporary directory.
//
// Default approach:
//
//	Nothing really special:
//	- get $PWD
//	- create a temporary directory there
//	- return $PWD and the relative path to the temporary directory
//
// Special case for windows:
//
//	Because there are file path length limitations, creating the temporary directory in $PWD and dropping the git repo
//	with submodules in there might exceed that limit.
//	Thus we use a different approach:
//	- create a temporary in the system's $TEMP
//	- cd to $TEMP
//	- return $TEMP (which is no $PWD) and the relative path to the temporary directory
//	Because we did the cd, test can't run in parallel anymore.
func (th testOSHelper) RelativeTempDir(t *testing.T, pattern string) (outerDir, dir string) {
	switch th {
	case "windows":
		fullPath, err := os.MkdirTemp("", cmp.Or(pattern, "local-tmp-dir-*"))
		require.NoError(t, err, "creating local temp dir")

		t.Cleanup(func() {
			err := os.RemoveAll(fullPath)
			require.NoError(t, err, "removing local tmp dir")
		})

		pwd, rel := filepath.Split(fullPath)
		t.Chdir(pwd) // t.Parallel() can't be used when using t.Chdir()
		return pwd, rel
	default:
		pwd, err := os.Getwd()
		require.NoError(t, err, "getting PWD")

		fullPath, err := os.MkdirTemp(pwd, cmp.Or(pattern, "local-tmp-dir-*"))
		require.NoError(t, err, "creating local tmp dir")

		t.Cleanup(func() {
			err := os.RemoveAll(fullPath)
			require.NoError(t, err, "removing local tmp dir")
		})

		rel, err := filepath.Rel(pwd, fullPath)
		require.NoError(t, err, "getting local tmp dir's relative path")

		return pwd, rel
	}
}

// setupCachingCredHelpers sets up a (global) git cred helpers
//   - the 1st one uses `git-credential-store` to create a file in the build directory
//   - the 2nd one uses `git-credential-store` with a temporary file
//
// After the tests are done, all original cred helpers are restored
func setupCachingCredHelpers(t *testing.T) {
	gitCredCache, err := os.CreateTemp("", "")
	require.NoError(t, err, "creating temp file for cred cache")
	require.NoError(t, gitCredCache.Close(), "closing the temp file for cred cache")
	t.Cleanup(func() {
		err := os.Remove(gitCredCache.Name())
		require.NoError(t, err, "deleting temp file for cred cache")
	})

	// ignoring error, because unset configs would produce an error too
	orgCredHelper, _ := exec.Command("git", "config", "--global", "--get-all", "credential.helper").Output()
	orgCredHelper = bytes.Trim(orgCredHelper, "\n\r")

	err = exec.Command("git", "config", "--global", "--replace-all", "credential.helper", "").Run()
	require.NoError(t, err, "adding empty cred helper")

	// global credential helper with the cache in the build dir, thus caches are separate per test
	helper := `` +
		`f(){ ` +
		`  bd=$(echo "$CI_BUILDS_DIR" | sed "s/\\\/\//g"); ` +
		`  if [ -z "${bd}" ] || [ ! -d "${bd}" ]; then exit 1; fi; ` +
		`  git credential-store --file="${bd}/git-credentials" "$1" ;` +
		`}; f`
	err = exec.Command("git", "config", "--global", "--add", "credential.helper", "!"+helper).Run()
	require.NoError(t, err, "adding storing cred helper per build")

	// global credential helper which uses the same cache for all tests
	helper = "store --file=" + helpers.ToSlash(gitCredCache.Name())
	err = exec.Command("git", "config", "--global", "--add", "credential.helper", helper).Run()
	require.NoError(t, err, "adding caching cred helper")

	t.Cleanup(func() {
		err := exec.Command("git", "config", "--global", "--unset-all", "credential.helper").Run()
		require.NoError(t, err, "unsetting cred helper")
		for _, helper := range bytes.Split(orgCredHelper, []byte{'\n'}) {
			if len(helper) > 0 {
				err := exec.Command("git", "config", "--global", "--add", "credential.helper", string(helper)).Run()
				require.NoError(t, err, "restoring credential.helper: %s", helper)
			}
		}
	})
}

// setInvalidGitCreds injects invalid creds into git cred helpers
func setInvalidGitCreds(t *testing.T, orgURL url.URL) {
	creds := map[string]string{
		"some-user":       "some-token",
		"gitlab-ci-token": "old_expired_token",
	}

	for username, password := range creds {
		// save the fake / invalid creds
		cmd := exec.Command("git", "credential", "approve")
		cmd.Stdin = strings.NewReader(fmt.Sprintf("protocol=%s\nhost=%s\nusername=%s\npassword=%s\n", orgURL.Scheme, orgURL.Host, username, password))
		err := cmd.Run()
		require.NoError(t, err, "setting up fake creds")

		// ensure the cred helper is set up and returns the fake creds
		cmd = exec.Command("git", "credential", "fill")
		cmd.Stdin = strings.NewReader(fmt.Sprintf("protocol=%s\nhost=%s\nusername=%s\n", orgURL.Scheme, orgURL.Host, username))
		var out []byte
		out, err = cmd.Output()
		require.NoError(t, err, "getting cached git creds")
		require.Contains(t, string(out), fmt.Sprintf("username=%s", username), "wrong user for cached cred")
		require.Contains(t, string(out), fmt.Sprintf("password=%s", password), "wrong password for cached cred")
	}
}

func onlyHost(t *testing.T, remoteURL string) string {
	t.Helper()

	u, err := url.Parse(remoteURL)
	require.NoError(t, err, "parsing URL")

	return url_helpers.OnlySchemeAndHost(u).String()
}

func withMaskedPassword(t *testing.T, orgURL string) string {
	t.Helper()

	pattern := `(//[^:]*:)([^@]+?)(@)`
	re, err := regexp.Compile(pattern)
	require.NoError(t, err, "compiling RE %q", pattern)

	return re.ReplaceAllString(orgURL, "${1}[MASKED]${3}")
}

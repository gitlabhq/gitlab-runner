package shell_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers"
)

func onEachShell(t *testing.T, f func(t *testing.T, shell string)) {
	t.Run("bash", func(t *testing.T) {
		if helpers.SkipIntegrationTests(t, "bash") {
			t.Skip()
		}

		f(t, "bash")
	})

	t.Run("cmd.exe", func(t *testing.T) {
		if helpers.SkipIntegrationTests(t, "cmd.exe") {
			t.Skip()
		}

		f(t, "cmd")
	})

	t.Run("powershell.exe", func(t *testing.T) {
		if helpers.SkipIntegrationTests(t, "powershell.exe") {
			t.Skip()
		}

		f(t, "powershell")
	})
}

func runBuildWithTrace(t *testing.T, build *common.Build, trace *common.Trace) error {
	timeoutTimer := time.AfterFunc(10*time.Second, func() {
		t.Log("Timed out")
		t.FailNow()
	})
	defer timeoutTimer.Stop()

	return build.Run(&common.Config{}, trace)
}

func runBuild(t *testing.T, build *common.Build) error {
	return runBuildWithTrace(t, build, &common.Trace{Writer: os.Stdout})
}

func runBuildReturningOutput(t *testing.T, build *common.Build) (string, error) {
	buf := bytes.NewBuffer(nil)
	err := runBuildWithTrace(t, build, &common.Trace{Writer: buf})
	output := buf.String()
	t.Log(output)

	return output, err
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
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return build, cleanup
}

func TestBuildSuccess(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		err = runBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildAbort(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		longRunningBuild, err := common.GetLongRunningBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, longRunningBuild, shell)
		defer cleanup()

		abortTimer := time.AfterFunc(time.Second, func() {
			t.Log("Interrupt")
			build.SystemInterrupt <- os.Interrupt
		})
		defer abortTimer.Stop()

		err = runBuild(t, build)
		assert.EqualError(t, err, "aborted: interrupt")
	})
}

func TestBuildCancel(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		longRunningBuild, err := common.GetLongRunningBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, longRunningBuild, shell)
		defer cleanup()

		cancelChan := make(chan interface{}, 1)
		cancelTimer := time.AfterFunc(time.Second, func() {
			t.Log("Cancel")
			cancelChan <- true
		})
		defer cancelTimer.Stop()

		err = runBuildWithTrace(t, build, &common.Trace{Writer: os.Stdout, Abort: cancelChan})
		assert.EqualError(t, err, "canceled")
		assert.IsType(t, err, &common.BuildError{})
	})
}

func TestBuildWithIndexLock(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		err = runBuild(t, build)
		assert.NoError(t, err)

		build.JobResponse.AllowGitFetch = true
		ioutil.WriteFile(build.BuildDir+"/.git/index.lock", []byte{}, os.ModeSticky)

		err = runBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildWithShallowLock(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, []common.BuildVariable{
			common.BuildVariable{Key: "GIT_DEPTH", Value: "1"},
			common.BuildVariable{Key: "GIT_STRATEGY", Value: "fetch"}}...)

		err = runBuild(t, build)
		assert.NoError(t, err)

		ioutil.WriteFile(build.BuildDir+"/.git/shallow.lock", []byte{}, os.ModeSticky)

		err = runBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildWithGitStrategyNone(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Variables = append(build.Variables, common.BuildVariable{Key: "GIT_STRATEGY", Value: "none"})

		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "pre-clone-script")
		assert.NotContains(t, out, "Cloning repository")
		assert.NotContains(t, out, "Fetching changes")
		assert.Contains(t, out, "Skipping Git repository setup")
	})
}

func TestBuildWithGitStrategyFetch(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Variables = append(build.Variables, common.BuildVariable{Key: "GIT_STRATEGY", Value: "fetch"})

		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Cloning repository")

		out, err = runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Fetching changes")

		assert.Contains(t, out, "pre-clone-script")
	})
}

func TestBuildWithGitStrategyClone(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Variables = append(build.Variables, common.BuildVariable{Key: "GIT_STRATEGY", Value: "clone"})

		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Cloning repository")

		out, err = runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Cloning repository")

		assert.Contains(t, out, "pre-clone-script")
	})
}

func TestBuildWithGitSubmoduleStrategyNone(t *testing.T) {
	for _, strategy := range []string{"none", ""} {
		t.Run("strategy "+strategy, func(t *testing.T) {
			onEachShell(t, func(t *testing.T, shell string) {
				successfulBuild, err := common.GetSuccessfulBuild()
				assert.NoError(t, err)
				build, cleanup := newBuild(t, successfulBuild, shell)
				defer cleanup()

				build.Variables = append(build.Variables, common.BuildVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "none"})

				out, err := runBuildReturningOutput(t, build)
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
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.BuildVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"})

		out, err := runBuildReturningOutput(t, build)
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
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.BuildVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"})

		out, err := runBuildReturningOutput(t, build)
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

func TestBuildWithGitSubmoduleStrategyInvalid(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.BuildVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "invalid"})

		out, err := runBuildReturningOutput(t, build)
		assert.EqualError(t, err, "unknown GIT_SUBMODULE_STRATEGY")
		assert.NotContains(t, out, "Skipping Git submodules setup")
		assert.NotContains(t, out, "Updating/initializing submodules...")
		assert.NotContains(t, out, "Updating/initializing submodules recursively...")
	})
}

func TestBuildWithGitSubmoduleStrategyRecursiveAndGitStrategyNone(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.BuildVariable{Key: "GIT_STRATEGY", Value: "none"})
		build.Variables = append(build.Variables, common.BuildVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"})

		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Cloning repository")
		assert.NotContains(t, out, "Fetching changes")
		assert.Contains(t, out, "Skipping Git repository setup")
		assert.NotContains(t, out, "Updating/initializing submodules...")
		assert.NotContains(t, out, "Updating/initializing submodules recursively...")
		assert.Contains(t, out, "Skipping Git submodules setup")
	})
}

func TestBuildWithDebugTrace(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		// The default build shouldn't have debug tracing enabled
		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotRegexp(t, `[^$] echo Hello World`, out)

		build.Variables = append(build.Variables, common.BuildVariable{Key: "CI_DEBUG_TRACE", Value: "true"})

		out, err = runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Regexp(t, `[^$] echo Hello World`, out)
	})
}

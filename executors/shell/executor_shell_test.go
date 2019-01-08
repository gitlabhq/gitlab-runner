package shell_test

import (
	"bytes"
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
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/session"
)

const (
	TestTimeout = 10 * time.Second
)

func gitInDir(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	return cmd.Output()
}

func skipOnGit(t *testing.T, constraints string) bool {
	out, err := gitInDir("", "version")
	if err != nil {
		t.Fatal("Can't detect git version", err)
		return true
	}

	gitVersionOut := string(out)
	split := strings.SplitN(gitVersionOut, " ", 3)
	if len(split) < 3 {
		t.Fatal("Can't extract git version from", gitVersionOut)
		return true
	}

	gitVersion, err := version.NewVersion(strings.TrimSpace(split[2]))
	if err != nil {
		t.Fatal("Can't detect git version", err)
		return true
	}

	rules, err := version.NewConstraint(constraints)
	if err != nil {
		t.Fatal("Invalid constraint", err)
		return true
	}

	return rules.Check(gitVersion)
}

func skipOnGit17x(t *testing.T) bool {
	return skipOnGit(t, "< 1.8")
}

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

func runBuildWithOptions(t *testing.T, build *common.Build, config *common.Config, trace *common.Trace) error {
	timeoutTimer := time.AfterFunc(TestTimeout, func() {
		t.Log("Timed out")
		t.FailNow()
	})
	defer timeoutTimer.Stop()

	return build.Run(config, trace)
}

func runBuildWithTrace(t *testing.T, build *common.Build, trace *common.Trace) error {
	return runBuildWithOptions(t, build, &common.Config{}, trace)
}

func runBuild(t *testing.T, build *common.Build) error {
	err := runBuildWithTrace(t, build, &common.Trace{Writer: os.Stdout})
	assert.True(t, build.IsSharedEnv())
	return err
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

		trace := &common.Trace{Writer: os.Stdout}

		cancelTimer := time.AfterFunc(time.Second, func() {
			t.Log("Cancel")
			trace.CancelFunc()
		})
		defer cancelTimer.Stop()

		err = runBuildWithTrace(t, build, trace)
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
		err = ioutil.WriteFile(build.BuildDir+"/.git/index.lock", []byte{}, os.ModeSticky)
		require.NoError(t, err)

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

		build.Variables = append(build.Variables, []common.JobVariable{
			common.JobVariable{Key: "GIT_DEPTH", Value: "1"},
			common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"}}...)

		err = runBuild(t, build)
		assert.NoError(t, err)

		err = ioutil.WriteFile(build.BuildDir+"/.git/shallow.lock", []byte{}, os.ModeSticky)
		require.NoError(t, err)

		err = runBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildWithHeadLock(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		err = runBuild(t, build)
		assert.NoError(t, err)

		build.JobResponse.AllowGitFetch = true
		err = ioutil.WriteFile(build.BuildDir+"/.git/HEAD.lock", []byte{}, os.ModeSticky)
		require.NoError(t, err)

		err = runBuild(t, build)
		assert.NoError(t, err)
	})
}

func TestBuildWithGitLFSHook(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		err = runBuild(t, build)
		assert.NoError(t, err)

		gitLFSPostCheckoutHook := "#!/bin/sh\necho 'running git lfs hook' >&2\nexit 2\n"

		err = os.MkdirAll(build.BuildDir+"/.git/hooks/", 0755)
		require.NoError(t, err)
		err = ioutil.WriteFile(build.BuildDir+"/.git/hooks/post-checkout", []byte(gitLFSPostCheckoutHook), 0777)
		require.NoError(t, err)
		build.JobResponse.AllowGitFetch = true

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
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "none"})

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
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"})

		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Cloning repository")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)

		out, err = runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Fetching changes")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
		assert.Contains(t, out, "pre-clone-script")
	})
}

func TestBuildWithGitStrategyFetchNoCheckout(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"})
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_CHECKOUT", Value: "false"})

		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Cloning repository")
		assert.Contains(t, out, "Skipping Git checkout")

		out, err = runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Fetching changes")
		assert.Contains(t, out, "Skipping Git checkout")
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
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "clone"})

		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Cloning repository")

		out, err = runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Cloning repository")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
		assert.Contains(t, out, "pre-clone-script")
	})
}

func TestBuildWithGitStrategyCloneNoCheckout(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "clone"})
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_CHECKOUT", Value: "false"})

		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Cloning repository")

		out, err = runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Cloning repository")
		assert.Contains(t, out, "Skipping Git checkout")
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

				build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "none"})

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

		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"})

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
	if skipOnGit17x(t) {
		return
	}

	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"})

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

		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "invalid"})

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

		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "none"})
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"})

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

func TestBuildWithGitSubmoduleModified(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"})

		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Updating/initializing submodules...")

		submoduleDir := filepath.Join(build.BuildDir, "gitlab-grack")
		submoduleReadme := filepath.Join(submoduleDir, "README.md")

		// modify submodule and commit
		modifySubmoduleBeforeCommit := "commited change"
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
		out, err = runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotContains(t, out, "Your local changes to the following files would be overwritten by checkout")
		assert.NotContains(t, out, "Please commit your changes or stash them before you switch branches")
		assert.NotContains(t, out, "Aborting")
		assert.Contains(t, out, "Updating/initializing submodules...")
	})
}

func TestBuildWithoutDebugTrace(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		// The default build shouldn't have debug tracing enabled
		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.NotRegexp(t, `[^$] echo Hello World`, out)
	})
}
func TestBuildWithDebugTrace(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetSuccessfulBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Variables = append(build.Variables, common.JobVariable{Key: "CI_DEBUG_TRACE", Value: "true"})

		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Regexp(t, `[^$] echo Hello World`, out)
	})
}

func TestBuildMultilineCommand(t *testing.T) {
	multilineBuild, err := common.GetMultilineBashBuild()
	assert.NoError(t, err)
	build, cleanup := newBuild(t, multilineBuild, "bash")
	defer cleanup()

	// The default build shouldn't have debug tracing enabled
	out, err := runBuildReturningOutput(t, build)
	assert.NoError(t, err)
	assert.NotContains(t, out, "bash")
	assert.Contains(t, out, "Hello World")
	assert.Contains(t, out, "collapsed multi-line command")
}

func TestBuildWithBrokenGitSSLCAInfo(t *testing.T) {
	if skipOnGit17x(t) || skipOnGit(t, ">= 2.10.2") {
		return
	}

	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteBrokenTLSBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.URL = "https://gitlab.com"

		out, err := runBuildReturningOutput(t, build)
		assert.Error(t, err)
		assert.Contains(t, out, "Cloning repository")
		assert.NotContains(t, out, "Updating/initializing submodules")
	})
}

func TestBuildWithGoodGitSSLCAInfo(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteGitLabComTLSBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.URL = "https://gitlab.com"

		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Cloning repository")
		assert.Contains(t, out, "Updating/initializing submodules")
	})
}

// TestBuildWithGitSSLAndStrategyFetch describes issue https://gitlab.com/gitlab-org/gitlab-runner/issues/2991
func TestBuildWithGitSSLAndStrategyFetch(t *testing.T) {
	onEachShell(t, func(t *testing.T, shell string) {
		successfulBuild, err := common.GetRemoteGitLabComTLSBuild()
		assert.NoError(t, err)
		build, cleanup := newBuild(t, successfulBuild, shell)
		defer cleanup()

		build.Runner.PreCloneScript = "echo pre-clone-script"
		build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"})

		out, err := runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Cloning repository")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)

		out, err = runBuildReturningOutput(t, build)
		assert.NoError(t, err)
		assert.Contains(t, out, "Fetching changes")
		assert.Regexp(t, "Checking out [a-f0-9]+ as", out)
		assert.Contains(t, out, "pre-clone-script")
	})
}

func TestInteractiveTerminal(t *testing.T) {
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
			if helpers.SkipIntegrationTests(t, c.app) {
				t.Skip()
			}

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
				err := runBuildWithOptions(
					t,
					build,
					&common.Config{SessionServer: common.SessionServer{SessionTimeout: 2}},
					&common.Trace{Writer: buf},
				)
				require.NoError(t, err)

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
			assert.NoError(t, err)
			assert.Equal(t, c.expectedStatusCode, resp.StatusCode)

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
				t.Log(out)
				assert.Contains(t, out, "Terminal is connected, will time out in 2s...")
				return
			}

			out := <-buildOut
			t.Log(out)
			assert.NotContains(t, out, "Terminal is connected, will time out in 2s...")
		})
	}
}

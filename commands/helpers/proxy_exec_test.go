//go:build !integration

package helpers

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	clihelpers "gitlab.com/gitlab-org/golang-cli-helpers"
)

func newProxyExecTestApp() *cli.App {
	cmd := &ProxyExecCommand{}

	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Commands = append(app.Commands, cli.Command{
		Name:   "proxy-exec",
		Action: cmd.Execute,
		Flags:  clihelpers.GetFlagsFromStruct(cmd),
	})

	return app
}

func TestProxyExec(t *testing.T) {
	dir := t.TempDir()

	cmd := []string{"echo", "foobar"}
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd", "/C", "echo", "foobar"}
	}
	args := append([]string{os.Args[0], "proxy-exec", "--temp-dir", dir}, cmd...)

	app := newProxyExecTestApp()
	buf := new(bytes.Buffer)

	defer captureOutput(buf)()

	require.NoError(t, app.Run(args))
	require.Contains(t, buf.String(), "foobar")
	require.NoFileExists(t, filepath.Join(dir, "gitlab-runner-helper"))
}

func TestProxyExecBootstrap(t *testing.T) {
	dir := t.TempDir()

	cmd := []string{"echo", "bootstrapped"}
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd", "/C", "echo", "bootstrapped"}
	}
	args := append([]string{os.Args[0], "proxy-exec", "--temp-dir", dir, "--bootstrap"}, cmd...)

	app := newProxyExecTestApp()
	buf := new(bytes.Buffer)

	defer captureOutput(buf)()

	require.NoError(t, app.Run(args))
	require.Contains(t, buf.String(), "bootstrapped")
	require.FileExists(t, filepath.Join(dir, "gitlab-runner-helper"))
}

// TestProxyExecBackgroundProcess regression-tests that proxy-exec returns
// promptly when the executed command spawns a backgrounded subprocess that
// inherits stdout/stderr. The `(sleep 30 &)` subshell double-forks sleep so
// it's reparented away from sh, but sleep still inherits sh's stdout/stderr
// file descriptors. With the previous cmd.Stdout = proxy.Stdout() approach,
// os/exec's internal copy goroutine would block on EOF until sleep exited.
func TestProxyExecBackgroundProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on POSIX shell double-fork semantics")
	}

	dir := t.TempDir()
	shellCmd := []string{"sh", "-c", "(sleep 30 &); echo done"}
	args := append([]string{os.Args[0], "proxy-exec", "--temp-dir", dir}, shellCmd...)

	app := newProxyExecTestApp()
	buf := new(bytes.Buffer)
	defer captureOutput(buf)()

	done := make(chan error, 1)
	go func() {
		done <- app.Run(args)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
		require.Contains(t, buf.String(), "done")
	case <-time.After(5 * time.Second):
		t.Fatal("proxy-exec did not return within 5s — likely blocked on inherited stdout/stderr FDs")
	}
}

func captureOutput(w io.Writer) func() {
	stdout = w
	stderr = w
	return func() {
		stdout = os.Stdout
		stderr = os.Stderr
	}
}

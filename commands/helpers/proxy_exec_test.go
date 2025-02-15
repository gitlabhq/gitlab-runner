//go:build !integration

package helpers

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

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

func captureOutput(w io.Writer) func() {
	stdout = w
	stderr = w
	return func() {
		stdout = os.Stdout
		stderr = os.Stderr
	}
}

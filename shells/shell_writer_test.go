package shells

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers"
)

type TestShellWriter interface {
	ShellWriter

	Finish(trace bool) string
	GetTemporaryPath() string
}

func onShell(t *testing.T, name, command, extension string, cmdArgs []string, writer TestShellWriter) {
	const TestPath = "test-path"

	t.Run(name, func(t *testing.T) {
		scriptFile := filepath.Join(writer.GetTemporaryPath(), name+"-test-script."+extension)

		testTmpDir := writer.MkTmpDir(name + "-mkdir-test")
		writer.Cd(testTmpDir)
		writer.MkDir(TestPath)
		writer.MkDir(TestPath)
		script := writer.Finish(false)

		err := ioutil.WriteFile(scriptFile, []byte(script), 0700)
		require.NoError(t, err)

		if helpers.SkipIntegrationTests(t, command) {
			t.Skip()
		}

		cmdArgs = append(cmdArgs, scriptFile)
		cmd := exec.Command(command, cmdArgs...)
		err = cmd.Run()
		assert.NoError(t, err)

		createdPath := filepath.Join(testTmpDir, TestPath)
		_, err = ioutil.ReadDir(createdPath)
		assert.NoError(t, err)
	})
}

func TestMkDir(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "test-shell-script")
	defer os.RemoveAll(tmpDir)
	require.NoError(t, err)

	onShell(t, "bash", "bash", "sh", []string{}, &BashWriter{TemporaryPath: tmpDir})
	onShell(t, "cmd", "cmd.exe", "cmd", []string{"/Q", "/C"}, &CmdWriter{TemporaryPath: tmpDir})
	onShell(t, "powershell", "powershell.exe", "ps1", []string{"-noprofile", "-noninteractive", "-executionpolicy", "Bypass", "-command"}, &PsWriter{TemporaryPath: tmpDir})
}

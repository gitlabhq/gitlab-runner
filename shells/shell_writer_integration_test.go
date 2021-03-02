package shells_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/shells"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

func runShell(t *testing.T, shell, cwd string, writer shells.ShellWriter) {
	var extension string
	var cmdArgs []string

	switch shell {
	case "bash":
		extension = "sh"

	case "cmd":
		extension = "cmd"
		cmdArgs = append(cmdArgs, "/Q", "/C")

	case "powershell", "pwsh":
		extension = "ps1"
		cmdArgs = append(cmdArgs, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command")

	default:
		require.FailNow(t, "unknown shell %q", shell)
	}

	script := writer.Finish(false)
	scriptFile := filepath.Join(cwd, shell+"-test-script."+extension)
	err := ioutil.WriteFile(scriptFile, []byte(script), 0700)
	require.NoError(t, err)
	defer os.Remove(scriptFile)

	cmdArgs = append(cmdArgs, scriptFile)
	cmd := exec.Command(shell, cmdArgs...)
	cmd.Dir = cwd

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "output: %s", string(output))
	assert.Empty(t, string(output))
}

func TestMkDir(t *testing.T) {
	const TestPath = "test-path"

	tmpDir, err := ioutil.TempDir("", "runner-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, writer shells.ShellWriter) {
		testTmpDir := writer.MkTmpDir(shell + "-mkdir-test")
		writer.Cd(testTmpDir)
		writer.MkDir(TestPath)
		writer.MkDir(TestPath)

		runShell(t, shell, tmpDir, writer)

		createdPath := filepath.Join(tmpDir, testTmpDir, TestPath)
		_, err := ioutil.ReadDir(createdPath)
		assert.NoError(t, err)
	})
}

func TestRmFile(t *testing.T) {
	const TestPath = "test-path"

	tmpDir, err := ioutil.TempDir("", "runner-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, writer shells.ShellWriter) {
		tmpFile := path.Join(tmpDir, TestPath)
		err = ioutil.WriteFile(tmpFile, []byte{}, 0600)
		require.NoError(t, err)

		writer.RmFile(TestPath)

		runShell(t, shell, tmpDir, writer)

		_, err = os.Stat(tmpFile)
		require.True(t, os.IsNotExist(err), "tmpFile not deleted")

		// check if the file do not exist
		runShell(t, shell, tmpDir, writer)
	})
}

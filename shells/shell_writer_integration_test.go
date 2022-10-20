//go:build integration

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
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

func runShell(t *testing.T, shell, cwd string, writer shells.ShellWriter) string {
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
	err := os.WriteFile(scriptFile, []byte(script), 0700)
	require.NoError(t, err)
	defer os.Remove(scriptFile)

	cmdArgs = append(cmdArgs, scriptFile)
	cmd := exec.Command(shell, cmdArgs...)
	cmd.Dir = cwd

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "output: %s", string(output))

	return string(output)
}

func TestMkDir(t *testing.T) {
	const TestPath = "test-path"

	tmpDir := t.TempDir()

	shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, writer shells.ShellWriter) {
		testTmpDir := writer.MkTmpDir(shell + "-mkdir-test")
		writer.Cd(testTmpDir)
		writer.MkDir(TestPath)
		writer.MkDir(TestPath)

		assert.Empty(t, runShell(t, shell, tmpDir, writer))

		createdPath := filepath.Join(tmpDir, testTmpDir, TestPath)
		_, err := os.ReadDir(createdPath)
		assert.NoError(t, err)
	})
}

func TestRmFile(t *testing.T) {
	const TestPath = "test-path"

	tmpDir := t.TempDir()

	shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, writer shells.ShellWriter) {
		tmpFile := path.Join(tmpDir, TestPath)
		err := os.WriteFile(tmpFile, []byte{}, 0600)
		require.NoError(t, err)

		writer.RmFile(TestPath)

		assert.Empty(t, runShell(t, shell, tmpDir, writer))

		_, err = os.Stat(tmpFile)
		require.True(t, os.IsNotExist(err), "tmpFile not deleted")

		// check if the file do not exist
		assert.Empty(t, runShell(t, shell, tmpDir, writer))
	})
}

func TestRmFilesRecursive(t *testing.T) {
	const TestPath = "test-path"

	tmpDir := t.TempDir()

	shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, writer shells.ShellWriter) {
		if shell == "cmd" {
			t.Skip("cmd shell is no longer actively developed")
		}

		var tmpFiles []string

		// lockfiles can be in multiple subdirs
		for i := 0; i < 3; i++ {
			tmpSubDir, err := os.MkdirTemp(tmpDir, "subdir")
			require.NoError(t, err)

			tmpFile := path.Join(tmpSubDir, TestPath)
			err = os.WriteFile(tmpFile, []byte{}, 0600)
			require.NoError(t, err)
			tmpFiles = append(tmpFiles, tmpFile)
		}

		writer.RmFilesRecursive(tmpDir, TestPath)

		runShell(t, shell, tmpDir, writer)

		for _, file := range tmpFiles {
			_, err := os.Stat(file)
			require.True(t, os.IsNotExist(err), "tmpFile not deleted")
		}
	})
}

func TestCommandArgumentExpansion(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "runner-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, w shells.ShellWriter) {
		var argumentsNoExpand []string
		var argumentsExpand []string

		switch shell {
		case "bash", "powershell", "pwsh":
			argumentsNoExpand = []string{"$a", "$b", "$c"}
			argumentsExpand = []string{"$d", "$e", "$f"}
		case "cmd":
			argumentsNoExpand = []string{"%a%", "%b%", "%c%"}
			argumentsExpand = []string{"%d%", "%e%", "%f%"}
		default:
			require.FailNow(t, "unknown shell %q", shell)
		}

		testFn := func(t *testing.T, w shells.ShellWriter) {
			w.Variable(common.JobVariable{
				Key:   "a",
				Value: "ac/dc",
			})
			w.Variable(common.JobVariable{
				Key:   "b",
				Value: "beatles",
			})
			w.Variable(common.JobVariable{
				Key:   "c",
				Value: "credence clearwater revival",
			})

			w.Variable(common.JobVariable{
				Key:   "d",
				Value: "d_expanded",
			})
			w.Variable(common.JobVariable{
				Key:   "e",
				Value: "e_expanded",
			})
			w.Variable(common.JobVariable{
				Key:   "f",
				Value: "f_expanded",
			})

			w.Command("echo", argumentsNoExpand...)
			w.CommandArgExpand("echo", argumentsExpand...)

			output := runShell(t, shell, tmpDir, w)

			assert.NotContains(t, output, "ac/dc")
			assert.NotContains(t, output, "beatles")
			assert.NotContains(t, output, "credence clearwater revival")

			assert.Contains(t, output, "d_expanded")
			assert.Contains(t, output, "e_expanded")
			assert.Contains(t, output, "f_expanded")
		}

		if shell == "bash" {
			t.Run("no posix escape", func(t *testing.T) {
				testFn(t, w)
			})
			t.Run("posix escape", func(t *testing.T) {
				build := &common.Build{
					Runner: &common.RunnerConfig{
						RunnerSettings: common.RunnerSettings{
							FeatureFlags: map[string]bool{
								featureflags.PosixlyCorrectEscapes: true,
							},
						},
					},
				}

				testFn(t, shells.NewBashWriter(build, "bash"))
			})
		} else {
			testFn(t, w)
		}
	})
}

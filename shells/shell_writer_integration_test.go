//go:build integration

package shells_test

import (
	"io"
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

func runShell(t *testing.T, shell, cwd string, writer shells.ShellWriter, env []string) string {
	var extension string
	var cmdArgs []string

	switch shell {
	case "bash":
		extension = "sh"

	case "powershell", "pwsh":
		extension = "ps1"
		cmdArgs = append(cmdArgs, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command")

	default:
		require.FailNow(t, "unknown shell %q", shell)
	}

	script := writer.Finish(false)
	// pwsh has issues with `,` in file paths, so we create the script file in a random directory instead of the "test
	// directory", so that we don't fail on test names with `,`.
	scriptFile, err := os.CreateTemp("", shell+"-*-test-script."+extension)
	require.NoError(t, err, "creating temp file")
	_, err = io.WriteString(scriptFile, script)
	require.NoError(t, err, "writing to temp file")
	require.NoError(t, scriptFile.Close(), "closing temp file")
	require.NoError(t, os.Chmod(scriptFile.Name(), 0700), "chmod'ing temp file")
	defer os.Remove(scriptFile.Name())

	cmdArgs = append(cmdArgs, scriptFile.Name())
	cmd := exec.Command(shell, cmdArgs...)
	cmd.Env = env
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

		assert.Empty(t, runShell(t, shell, tmpDir, writer, os.Environ()))

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

		assert.Empty(t, runShell(t, shell, tmpDir, writer, os.Environ()))

		_, err = os.Stat(tmpFile)
		require.True(t, os.IsNotExist(err), "tmpFile not deleted")

		// check if the file do not exist
		assert.Empty(t, runShell(t, shell, tmpDir, writer, os.Environ()))
	})
}

func TestRmFilesRecursive(t *testing.T) {
	const baseName = "test-file"

	testFiles := testFileTree{
		"subdir-1/" + baseName:       "should be deleted",
		"subdir-1/someOtherFile":     "should NOT be deleted",
		"other/subdir-2/" + baseName: "should be deleted",
		"subdir-3/" + baseName:       "should be deleted",
		baseName + "_foo":            "should NOT be deleted",
		baseName:                     "", // is a dir, should not be deleted
	}

	shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, writer shells.ShellWriter) {
		tmpDir := t.TempDir()

		testFiles.Create(t, tmpDir)

		writer.RmFilesRecursive(tmpDir, baseName)
		runShell(t, shell, tmpDir, writer, os.Environ())

		assert.DirExists(t, filepath.Join(tmpDir, "subdir-1"))
		assert.NoFileExists(t, filepath.Join(tmpDir, "subdir-1", baseName))
		assert.FileExists(t, filepath.Join(tmpDir, "subdir-1", "someOtherFile"))

		assert.DirExists(t, filepath.Join(tmpDir, "other", "subdir-2"))
		assert.NoFileExists(t, filepath.Join(tmpDir, "other", "subdir-2"))

		assert.DirExists(t, filepath.Join(tmpDir, "subdir-3"))
		assert.NoFileExists(t, filepath.Join(tmpDir, "subdir-3", baseName))

		assert.FileExists(t, filepath.Join(tmpDir, baseName+"_foo"))

		assert.DirExists(t, filepath.Join(tmpDir, baseName))
	})
}

func TestRmDirsRecursive(t *testing.T) {
	testFiles := testFileTree{
		"some/dir2rm/even/nested/dir2rm/file": "should be deleted incl. ancestor dirs",
		"dir2rm":                              "this is a file and should not be deleted",
		"not/really_dir2rm":                   "",
		"random/dir2rm":                       "",
	}

	shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, writer shells.ShellWriter) {
		tmpDir := t.TempDir()
		testFiles.Create(t, tmpDir)

		writer.RmDirsRecursive(tmpDir, "dir2rm")

		runShell(t, shell, tmpDir, writer, os.Environ())

		assert.DirExists(t, filepath.Join(tmpDir, "some"))
		assert.NoDirExists(t, filepath.Join(tmpDir, "some/dir2rm"))
		assert.FileExists(t, filepath.Join(tmpDir, "dir2rm"))
		assert.DirExists(t, filepath.Join(tmpDir, "not/really_dir2rm"))
		assert.DirExists(t, filepath.Join(tmpDir, "random"))
		assert.NoDirExists(t, filepath.Join(tmpDir, "random/dir2rm"))
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

			output := runShell(t, shell, tmpDir, w, os.Environ())

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

type testFileTree map[string]string

func (tft testFileTree) Create(t *testing.T, baseDir string) {
	for path, content := range tft {
		if content == "" {
			// on empty content, we don't create a file but a leaf directory
			err := os.MkdirAll(filepath.Join(baseDir, path), 0750)
			require.NoError(t, err)
			continue
		}

		err := os.MkdirAll(filepath.Join(baseDir, filepath.Dir(path)), 0750)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(baseDir, path), []byte(content), 0644)
		require.NoError(t, err)
	}
}

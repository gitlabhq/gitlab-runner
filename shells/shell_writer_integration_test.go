//go:build integration

package shells_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/test"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

func runShell(t *testing.T, shell, cwd string, writer shells.ShellWriter, env []string) string {
	output, err := execShell(t, shell, cwd, writer, env)
	require.NoError(t, err, "output: %s", output)
	return output
}

// runShellExpectFailure is the counterpart of runShell that asserts the script terminates with a non-zero exit code.
func runShellExpectFailure(t *testing.T, shell, cwd string, writer shells.ShellWriter, env []string) string {
	output, err := execShell(t, shell, cwd, writer, env)
	require.Error(t, err, "script was expected to exit non-zero, output: %s", output)
	return output
}

func execShell(t *testing.T, shell, cwd string, writer shells.ShellWriter, env []string) (string, error) {
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
	return string(output), err
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

func TestExportRaw(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		value          string
		expectedOutput string
	}{
		{
			name:           "empty value",
			expectedOutput: "env:() | var:()",
		},
		{
			name:           "plain value",
			value:          "some-value",
			expectedOutput: "env:(some-value) | var:(some-value)",
		},
		{
			name:  "ref other var",
			value: filepath.Join("$PWD", "something"),
			expectedOutput: func() string {
				f := test.NormalizePath(filepath.Join(tmpDir, "something"))
				return "env:(" + f + ") | var:(" + f + ")"
			}(),
		},
		{
			name:  "is not escaped",
			value: "'$PWD'",
			expectedOutput: func() string {
				d := test.NormalizePath(tmpDir)
				return "env:('" + d + "') | var:('" + d + "')"
			}(),
		},
	}

	const varName = "someVar"

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, w shells.ShellWriter) {
				t.Parallel()

				w.ExportRaw(varName, tc.value)

				testCmd := fmt.Sprintf(
					`echo "env:(%s) | var:(%s)"`,
					varRef(varName, shell, true),  // -> sh: $someVar, pwsh: $env:someVar
					varRef(varName, shell, false), // -> sh: $someVar, pwsh: $someVar
				)
				w.Line(testCmd)

				out := runShell(t, shell, tmpDir, w, nil)
				assert.Equal(t, tc.expectedOutput, strings.TrimSpace(out))
			})
		})
	}
}

func varRef(name, shell string, exported bool) string {
	if shell == shells.Bash {
		return "$" + name
	}
	if exported {
		name = "env:" + name
	}
	return "$" + name
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
	tmpDir := t.TempDir()

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
			w.Variable(spec.Variable{
				Key:   "a",
				Value: "ac/dc",
			})
			w.Variable(spec.Variable{
				Key:   "b",
				Value: "beatles",
			})
			w.Variable(spec.Variable{
				Key:   "c",
				Value: "credence clearwater revival",
			})

			w.Variable(spec.Variable{
				Key:   "d",
				Value: "d_expanded",
			})
			w.Variable(spec.Variable{
				Key:   "e",
				Value: "e_expanded",
			})
			w.Variable(spec.Variable{
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

func TestCommandWithStdin(t *testing.T) {
	// We need a command which is available across OSs, behaves identical across OSs, consumes input from stdin, and gives
	// us a way to assert if the input made it to the command correctly.
	// "git credential fill" is one such command, but it could be any other command which has above mentioned properties.

	// Password used in the "shell metacharacters" case: single quote, backslash, dollar-prefixed token,
	// backtick, double quote.
	const metaPassword = `p'a\ss"$VAR` + "`x"

	tests := []struct {
		name           string
		stdin          string
		expectedOutput string
		expectFailure  bool
	}{
		{
			name:  "url-form input",
			stdin: "url=https://some-user:some-pass@some-host/repo",
			expectedOutput: `protocol=https
host=some-host
path=repo
username=some-user
password=some-pass
`,
		},
		{
			name:  "embedded newlines",
			stdin: "protocol=https\nhost=some-host\nusername=some-user\npassword=some-pass",
			expectedOutput: `protocol=https
host=some-host
username=some-user
password=some-pass
`,
		},
		{
			name:  "shell metacharacters",
			stdin: "protocol=https\nhost=some-host\nusername=some-user\npassword=" + metaPassword,
			expectedOutput: `protocol=https
host=some-host
username=some-user
password=` + metaPassword + "\n",
		},
		{
			name:          "non-zero child exit propagates",
			stdin:         "not-a-valid-credential-request",
			expectFailure: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, w shells.ShellWriter) {
				tmpDir := t.TempDir()

				w.CommandWithStdin(false, tc.stdin, "git", "credential", "fill")

				if tc.expectFailure {
					runShellExpectFailure(t, shell, tmpDir, w, os.Environ())
					return
				}

				output := runShell(t, shell, tmpDir, w, os.Environ())
				assert.Equal(t, tc.expectedOutput, output)
			})
		})
	}
}

func TestCommandWithStdinBestEffort(t *testing.T) {
	const marker = "the-script-carried-on"

	tests := []struct {
		name      string
		stdin     string
		command   string
		arguments []string
		// asserted on the script's output, to prove the command was really run
		expectedOutput string
	}{
		{
			name:           "successful command",
			stdin:          "url=https://some-user:some-pass@some-host/repo",
			command:        "git",
			arguments:      []string{"credential", "fill"},
			expectedOutput: "password=some-pass",
		},
		{
			name:      "failing command does not fail the script",
			stdin:     "not-a-valid-credential-request",
			command:   "git",
			arguments: []string{"credential", "fill"},
		},
		{
			name:    "command which cannot be started does not fail the script",
			stdin:   "some input",
			command: "definitely-not-an-existing-command",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, w shells.ShellWriter) {
				tmpDir := t.TempDir()

				w.CommandWithStdin(true, tc.stdin, tc.command, tc.arguments...)
				w.Command("echo", marker)

				output := runShell(t, shell, tmpDir, w, os.Environ())
				assert.Contains(t, output, marker, "the script must carry on past the command")
				if tc.expectedOutput != "" {
					assert.Contains(t, output, tc.expectedOutput, "the command must have run")
				}
			})
		})
	}
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

// TestIfFileReadable exercises the emitted readability guard end to end on
// every installed shell: the guarded branch must run for a readable file and
// must be skipped for a missing one. The permission-denied states need
// privileges the test environment cannot assume (chmod 000 is ineffective
// for root, ACL denials need Windows), they are pinned by the writer unit
// tests instead.
func TestIfFileReadable(t *testing.T) {
	const marker = "IF-FILE-READABLE-TAKEN"

	tests := []struct {
		name        string
		createFile  bool
		expectTaken bool
	}{
		{name: "readable file takes the branch", createFile: true, expectTaken: true},
		{name: "missing file skips the branch", createFile: false, expectTaken: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, w shells.ShellWriter) {
				tmpDir := t.TempDir()
				probed := filepath.Join(tmpDir, "probed-file")
				if tc.createFile {
					require.NoError(t, os.WriteFile(probed, []byte("content"), 0600))
				}

				w.IfFileReadable(probed)
				w.Line("echo " + marker)
				w.EndIf()

				output := runShell(t, shell, tmpDir, w, os.Environ())
				if tc.expectTaken {
					assert.Contains(t, output, marker)
				} else {
					assert.NotContains(t, output, marker)
				}
			})
		})
	}
}

// TestIfFileReadable_PermissionDenied_Windows verifies that the readability guard
// generated by IfFileReadable skips a file the build user cannot read because of a
// denied ACL, on the PowerShell shells. This is the case that lets get-sources
// chain includes for the user's global Git config without failing when HOME points
// at an unreadable directory. Read access is removed with a deny ACE, which the
// PowerShell guard's File.OpenRead honours even for an elevated process.
func TestIfFileReadable_PermissionDenied_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only: read access is denied with an icacls deny ACE")
	}

	shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, w shells.ShellWriter) {
		if shell == "bash" {
			t.Skip("the Windows read guard is only exercised through PowerShell")
		}

		readable, unreadable := writeReadableAndUnreadable(t)
		denyReadWindows(t, unreadable)

		assertReadGuardDiscriminates(t, shell, w, readable, unreadable)
	})
}

// TestIfFileReadable_PermissionDenied_Unix is the Unix counterpart. Read access is
// removed with chmod. The test skips when running as root, because root bypasses the
// read permission bit (access(R_OK) always succeeds), so an unreadable file cannot be
// simulated and the guard's false branch would never be reached.
func TestIfFileReadable_PermissionDenied_Unix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only: read access is removed with chmod")
	}
	if os.Getuid() == 0 {
		t.Skip("root bypasses the read permission bit, so an unreadable file cannot be simulated")
	}

	shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, w shells.ShellWriter) {
		readable, unreadable := writeReadableAndUnreadable(t)
		require.NoError(t, os.Chmod(unreadable, 0), "removing read access")
		t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) })

		assertReadGuardDiscriminates(t, shell, w, readable, unreadable)
	})
}

// writeReadableAndUnreadable creates two files in a fresh temp dir. Both start
// readable; the caller makes the second one unreadable with an OS-specific mechanism.
func writeReadableAndUnreadable(t *testing.T) (readable, unreadable string) {
	t.Helper()
	dir := t.TempDir()
	readable = filepath.Join(dir, "readable.gitconfig")
	unreadable = filepath.Join(dir, "unreadable.gitconfig")
	require.NoError(t, os.WriteFile(readable, []byte("[user]\n\tname = readable\n"), 0o600))
	require.NoError(t, os.WriteFile(unreadable, []byte("[user]\n\tname = secret\n"), 0o600))
	return readable, unreadable
}

// denyReadWindows removes read access to path for everyone with a deny ACE, and
// restores full access on cleanup so the temp dir can be torn down. The Everyone SID
// (*S-1-1-0) is used instead of a name to stay independent of the account and the
// system language.
func denyReadWindows(t *testing.T, path string) {
	t.Helper()

	t.Cleanup(func() {
		if out, err := exec.Command("icacls", path, "/reset").CombinedOutput(); err != nil {
			t.Logf("icacls /reset failed during cleanup: %s", out)
		}
	})

	out, err := exec.Command("icacls", path, "/deny", "*S-1-1-0:(R)").CombinedOutput()
	require.NoError(t, err, "icacls /deny failed: %s", out)
}

// assertReadGuardDiscriminates generates a single script that guards an echo behind
// IfFileReadable for both files, then asserts the readable file ran its block and the
// unreadable file did not. Doing both in one run proves the guard discriminates in the
// same shell and environment, rather than passing vacuously.
func assertReadGuardDiscriminates(t *testing.T, shell string, w shells.ShellWriter, readable, unreadable string) {
	t.Helper()
	const readMarker = "read-guard-entered"
	const denyMarker = "deny-guard-entered"

	w.IfFileReadable(readable)
	w.Command("echo", readMarker)
	w.EndIf()

	w.IfFileReadable(unreadable)
	w.Command("echo", denyMarker)
	w.EndIf()

	output := runShell(t, shell, t.TempDir(), w, os.Environ())
	assert.Contains(t, output, readMarker, "the guard must run its block for a readable file")
	assert.NotContains(t, output, denyMarker, "the guard must skip its block for a file that cannot be read")
}

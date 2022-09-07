//go:build !integration

package shells

import (
	"fmt"
	"path"
	"runtime"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type testCase struct {
	in  string
	out string
}

type outputColor struct {
	fn    func(string, ...interface{})
	color string
}

func TestCMD_EchoShellEscapes(t *testing.T) {
	for i, tc := range []testCase{
		{`abcdefghijklmnopqrstuvwxyz`, `abcdefghijklmnopqrstuvwxyz`},
		{`^ & < > |`, `^^ ^& ^< ^> ^|`},
		// FIXME: this currently escapes to ^! when it doesn't need to
		// {`!`, `!`},
		{`( )`, `^( ^)`},
	} {
		writer := &CmdWriter{}
		for j, functionsToTest := range []outputColor{
			{writer.Noticef, "\x1b[32;1m"},
			{writer.Warningf, "\x1b[0;33m"},
			{writer.Errorf, "\x1b[31;1m"},
			{writer.Printf, "\x1b[0;m"},
		} {
			functionsToTest.fn(tc.in)
			expected := fmt.Sprintf("echo %s%s\x1b[0;m\r\n", functionsToTest.color, tc.out)
			assert.Equal(t, expected, writer.String(), "case %d : %d", i, j)
			writer.Reset()
		}
	}
}

func TestCMD_CDShellEscapes(t *testing.T) {
	for i, tc := range []testCase{
		{`c:\`, `c:\`},
		{`c:/`, `c:\`},
		{`c:\Program Files`, `c:\Program Files`},
		{`c:\Program Files (x86)`, `c:\Program Files (x86)`},      // Don't escape the parens
		{`c: | rd Windows\System32`, `c: ^| rd Windows\System32`}, // Escape the |
	} {
		writer := &CmdWriter{}
		writer.Cd(tc.in)
		expected := fmt.Sprintf("cd /D \"%s\"\r\nIF !errorlevel! NEQ 0 exit /b !errorlevel!\r\n\r\n", tc.out)
		assert.Equal(t, expected, writer.String(), "case %d", i)
	}
}

func TestCMD_CommandShellEscapes(t *testing.T) {
	writer := &CmdWriter{}
	writer.Command("foo", "x&(y)")

	assert.Equal(t, "\"foo\" \"x^&(y)\"\r\nIF !errorlevel! NEQ 0 exit /b !errorlevel!\r\n\r\n", writer.String())
}

func TestCMD_CommandShellNoEscapes(t *testing.T) {
	writer := &CmdWriter{}
	writer.CommandArgExpand("foo", "x&(y)")

	assert.Equal(t, "\"foo\" \"x^&(y)\"\r\nIF !errorlevel! NEQ 0 exit /b !errorlevel!\r\n\r\n", writer.String())
}

func TestCMD_CommandEscapeVariable(t *testing.T) {
	writer := &CmdWriter{}
	writer.Variable(common.JobVariable{
		Key:   "A",
		Value: "expanded",
	})
	writer.Command("echo", "%A%")

	assert.Contains(t, writer.String(), `"echo" "^%A^%"`)
}

func TestCMD_IfCmdShellEscapes(t *testing.T) {
	writer := &CmdWriter{}
	writer.IfCmd("foo", "x&(y)")

	assert.Equal(t, "\"foo\" \"x^&(y)\" 2>NUL 1>NUL\r\nIF !errorlevel! EQU 0 (\r\n", writer.String())
}

func TestCMD_DelayedExpanstionFeatureFlag(t *testing.T) {
	cases := map[bool]string{
		true:  "\"foo\"\r\nIF %errorlevel% NEQ 0 exit /b %errorlevel%\r\n\r\n",
		false: "\"foo\"\r\nIF !errorlevel! NEQ 0 exit /b !errorlevel!\r\n\r\n",
	}

	for disableDelayedErrorLevelExpansion, expectedCmd := range cases {
		t.Run(
			"disableDelayedErrorLevelExpansion_"+strconv.FormatBool(disableDelayedErrorLevelExpansion),
			func(t *testing.T) {
				writer := &CmdWriter{}
				writer.disableDelayedErrorLevelExpansion = disableDelayedErrorLevelExpansion
				writer.Command("foo")

				assert.Equal(t, expectedCmd, writer.String())
			})
	}
}

func Test_CmdWriter_isTmpFile(t *testing.T) {
	tmpDir := "/foo/bar"
	bw := CmdWriter{TemporaryPath: tmpDir}

	tests := map[string]struct {
		path string
		want bool
	}{
		"tmp file var":     {path: path.Join(tmpDir, "BAZ"), want: true},
		"not tmp file var": {path: "bla bla bla", want: false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, bw.isTmpFile(tt.path))
		})
	}
}

func Test_CmdWriter_cleanPath(t *testing.T) {
	tests := map[string]struct {
		path, wantLinux, wantWindows string
	}{
		"relative path": {
			path:        "foo/bar/KEY",
			wantLinux:   "%CD%\\foo\\bar\\KEY",
			wantWindows: "%CD%\\foo\\bar\\KEY",
		},
		"absolute path": {
			path:        "/foo/bar/KEY",
			wantLinux:   "\\foo\\bar\\KEY",
			wantWindows: "%CD%\\foo\\bar\\KEY",
		},
		"absolute path with drive": {
			path:        "C:/foo/bar/KEY",
			wantLinux:   "%CD%\\C:\\foo\\bar\\KEY",
			wantWindows: "C:\\foo\\bar\\KEY",
		},
	}

	bw := CmdWriter{TemporaryPath: "foo/bar"}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := bw.cleanPath(tt.path)
			if runtime.GOOS == OSWindows {
				assert.Equal(t, tt.wantWindows, got)
			} else {
				assert.Equal(t, tt.wantLinux, got)
			}
		})
	}
}

// nolint:lll
func Test_CmdWriter_Variable(t *testing.T) {
	tests := map[string]struct {
		variable               common.JobVariable
		writer                 CmdWriter
		wantLinux, wantWindows string
	}{
		"file var, relative path": {
			variable: common.JobVariable{Key: "KEY", Value: "the secret", File: true},
			writer:   CmdWriter{TemporaryPath: "foo/bar"},
			// nolint:lll
			wantLinux:   "md \"foo\\\\bar\" 2>NUL 1>NUL\r\necho the secret > %CD%\\foo\\bar\\KEY\r\nSET KEY=%CD%\\foo\\bar\\KEY\r\n",
			wantWindows: "md \"foo\\\\bar\" 2>NUL 1>NUL\r\necho the secret > %CD%\\foo\\bar\\KEY\r\nSET KEY=%CD%\\foo\\bar\\KEY\r\n",
		},
		"file var, absolute path": {
			variable: common.JobVariable{Key: "KEY", Value: "the secret", File: true},
			writer:   CmdWriter{TemporaryPath: "/foo/bar"},
			// nolint:lll
			wantLinux:   "md \"\\\\foo\\\\bar\" 2>NUL 1>NUL\r\necho the secret > \\foo\\bar\\KEY\r\nSET KEY=\\foo\\bar\\KEY\r\n",
			wantWindows: "md \"\\\\foo\\\\bar\" 2>NUL 1>NUL\r\necho the secret > %CD%\\foo\\bar\\KEY\r\nSET KEY=%CD%\\foo\\bar\\KEY\r\n",
		},
		"file var, absolute path with drive": {
			variable: common.JobVariable{Key: "KEY", Value: "the secret", File: true},
			writer:   CmdWriter{TemporaryPath: "C:/foo/bar"},
			// nolint:lll
			wantLinux:   "md \"C:\\\\foo\\\\bar\" 2>NUL 1>NUL\r\necho the secret > %CD%\\C:\\foo\\bar\\KEY\r\nSET KEY=%CD%\\C:\\foo\\bar\\KEY\r\n",
			wantWindows: "md \"C:\\\\foo\\\\bar\" 2>NUL 1>NUL\r\necho the secret > C:\\foo\\bar\\KEY\r\nSET KEY=C:\\foo\\bar\\KEY\r\n",
		},
		"tmp file var, relative path": {
			variable:    common.JobVariable{Key: "KEY", Value: "foo/bar/KEY2"},
			writer:      CmdWriter{TemporaryPath: "foo/bar"},
			wantLinux:   "SET KEY=%%CD%%\\foo\\bar\\KEY2\r\n",
			wantWindows: "SET KEY=%%CD%%\\foo\\bar\\KEY2\r\n",
		},
		"tmp file var, absolute path": {
			variable:    common.JobVariable{Key: "KEY", Value: "/foo/bar/KEY2"},
			writer:      CmdWriter{TemporaryPath: "/foo/bar"},
			wantLinux:   "SET KEY=\\foo\\bar\\KEY2\r\n",
			wantWindows: "SET KEY=%%CD%%\\foo\\bar\\KEY2\r\n",
		},
		"tmp file var, absolute path with drive": {
			variable:    common.JobVariable{Key: "KEY", Value: "C:/foo/bar/KEY2"},
			writer:      CmdWriter{TemporaryPath: "C:/foo/bar"},
			wantLinux:   "SET KEY=%%CD%%\\C:\\foo\\bar\\KEY2\r\n",
			wantWindows: "SET KEY=C:\\foo\\bar\\KEY2\r\n",
		},
		"regular var": {
			variable:    common.JobVariable{Key: "KEY", Value: "VALUE"},
			writer:      CmdWriter{TemporaryPath: "C:/foo/bar"},
			wantLinux:   "SET KEY=VALUE\r\n",
			wantWindows: "SET KEY=VALUE\r\n",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.writer.Variable(tt.variable)
			if runtime.GOOS == OSWindows {
				assert.Equal(t, tt.wantWindows, tt.writer.String())
			} else {
				assert.Equal(t, tt.wantLinux, tt.writer.String())
			}
		})
	}
}

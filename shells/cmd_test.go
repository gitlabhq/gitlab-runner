package shells

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCase struct {
	in  string
	out string
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
		for j, fn := range []func(string, ...interface{}){
			writer.Notice,
			writer.Warning,
			writer.Error,
			writer.Print,
		} {
			fn(tc.in)
			expected := fmt.Sprintf("echo %s\r\n", tc.out)
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
		expected := fmt.Sprintf("cd /D \"%s\"\r\nIF %%errorlevel%% NEQ 0 exit /b %%errorlevel%%\r\n\r\n", tc.out)
		assert.Equal(t, expected, writer.String(), "case %d", i)
	}
}

func TestCMD_CommandShellEscapes(t *testing.T) {
	writer := &CmdWriter{}
	writer.Command("foo", "x&(y)")

	assert.Equal(t, "\"foo\" \"x^&(y)\"\r\nIF %errorlevel% NEQ 0 exit /b %errorlevel%\r\n\r\n", writer.String())
}

func TestCMD_IfCmdShellEscapes(t *testing.T) {
	writer := &CmdWriter{}
	writer.IfCmd("foo", "x&(y)")

	assert.Equal(t, "\"foo\" \"x^&(y)\" 2>NUL 1>NUL\r\nIF %errorlevel% EQU 0 (\r\n", writer.String())
}

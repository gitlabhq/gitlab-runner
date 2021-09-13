//go:build !integration
// +build !integration

package shells

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
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

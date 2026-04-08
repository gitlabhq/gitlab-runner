//go:build !integration

package internal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandFormatter_FormatLogLine_BashMode_SingleLine(t *testing.T) {
	formatter := NewCommandFormatter(false) // Bash mode
	result := formatter.FormatLogLine("echo hello")

	assert.Contains(t, result, EscapeForAnsiC(ansiGreen), "Expected green color code")
	assert.Contains(t, result, EscapeForAnsiC(ansiReset), "Expected reset color code")
	assert.Contains(t, result, commandPrefix, "Expected command prefix")
	assert.Contains(t, result, "echo hello", "Expected command in output")
	assert.NotContains(t, result, multilineIndicator, "Should not have multiline indicator for single-line command")
}

func TestCommandFormatter_FormatLogLine_BashMode_MultiLine(t *testing.T) {
	formatter := NewCommandFormatter(false) // Bash mode
	multiLineCmd := "line1\nline2\nline3"
	result := formatter.FormatLogLine(multiLineCmd)

	assert.Contains(t, result, "line1", "Expected first line in output")
	assert.False(t, strings.Contains(result, "line2") || strings.Contains(result, "line3"),
		"Should only show first line for multi-line command")
	assert.Contains(t, result, multilineIndicator, "Expected multiline indicator")
	assert.Contains(t, result, EscapeForAnsiC(ansiGreen), "Expected green color code")
}

func TestCommandFormatter_FormatLogLine_PosixMode_SingleLine(t *testing.T) {
	formatter := NewCommandFormatter(true) // POSIX mode
	result := formatter.FormatLogLine("echo hello")

	assert.False(t, strings.Contains(result, EscapeForAnsiC(ansiGreen)) || strings.Contains(result, EscapeForAnsiC(ansiReset)),
		"Should not have color codes in POSIX mode")
	assert.Contains(t, result, commandPrefix, "Expected command prefix")
	assert.Contains(t, result, "echo hello", "Expected command in output")
}

func TestCommandFormatter_FormatLogLine_PosixMode_MultiLine(t *testing.T) {
	formatter := NewCommandFormatter(true) // POSIX mode
	multiLineCmd := "line1\nline2\nline3"
	result := formatter.FormatLogLine(multiLineCmd)

	assert.Contains(t, result, "line1", "Expected first line in output")
	assert.Contains(t, result, multilineIndicator, "Expected multiline indicator")
	assert.NotContains(t, result, EscapeForAnsiC(ansiGreen), "Should not have colors in POSIX mode")
}

func TestCommandFormatter_GetDisplayCommand_SingleLine(t *testing.T) {
	formatter := NewCommandFormatter(false)
	result := formatter.getDisplayCommand("echo hello")

	assert.Equal(t, "echo hello", result, "Expected unchanged single line")
}

func TestCommandFormatter_GetDisplayCommand_MultiLine(t *testing.T) {
	formatter := NewCommandFormatter(false)
	result := formatter.getDisplayCommand("line1\nline2")

	expected := "line1" + multilineIndicator
	assert.Equal(t, expected, result)
}

func TestIsMultiline_True(t *testing.T) {
	assert.True(t, isMultiline("line1\nline2"), "Expected true for multi-line string")
}

func TestIsMultiline_False(t *testing.T) {
	assert.False(t, isMultiline("single line"), "Expected false for single-line string")
}

func TestGetFirstLine_SingleLine(t *testing.T) {
	assert.Equal(t, "single line", getFirstLine("single line"))
}

func TestGetFirstLine_MultiLine(t *testing.T) {
	assert.Equal(t, "line1", getFirstLine("line1\nline2\nline3"))
}

func TestGetFirstLine_Empty(t *testing.T) {
	assert.Equal(t, "", getFirstLine(""))
}

func TestCommandFormatter_ColorCodes(t *testing.T) {
	formatter := NewCommandFormatter(false)
	result := formatter.FormatLogLine("test")

	// Should use echo $'...' format for bash
	assert.True(t, strings.HasPrefix(result, "echo $'"), "Expected bash ANSI-C quoting format")
	assert.True(t, strings.HasSuffix(result, "'"), "Expected closing quote")
}

func TestCommandFormatter_PosixFormat(t *testing.T) {
	formatter := NewCommandFormatter(true)
	result := formatter.FormatLogLine("test")

	assert.True(t, strings.HasPrefix(result, "echo "), "Expected 'echo ' prefix")
	// Should NOT use ANSI-C quoting ($'...')
	assert.NotContains(t, result, "$'", "Should not use ANSI-C quoting in POSIX mode")
}

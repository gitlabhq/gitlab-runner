//go:build !integration

package internal

import (
	"strings"
	"testing"
)

func TestCommandFormatter_FormatLogLine_BashMode_SingleLine(t *testing.T) {
	formatter := NewCommandFormatter(false) // Bash mode
	result := formatter.FormatLogLine("echo hello")

	if !strings.Contains(result, ansiGreen) {
		t.Errorf("Expected green color code")
	}
	if !strings.Contains(result, ansiReset) {
		t.Errorf("Expected reset color code")
	}

	if !strings.Contains(result, commandPrefix) {
		t.Errorf("Expected command prefix")
	}

	if !strings.Contains(result, "echo hello") {
		t.Errorf("Expected command in output")
	}

	if strings.Contains(result, multilineIndicator) {
		t.Errorf("Should not have multiline indicator for single-line command")
	}
}

func TestCommandFormatter_FormatLogLine_BashMode_MultiLine(t *testing.T) {
	formatter := NewCommandFormatter(false) // Bash mode
	multiLineCmd := "line1\nline2\nline3"
	result := formatter.FormatLogLine(multiLineCmd)

	if !strings.Contains(result, "line1") {
		t.Errorf("Expected first line in output")
	}

	if strings.Contains(result, "line2") || strings.Contains(result, "line3") {
		t.Errorf("Should only show first line for multi-line command")
	}

	if !strings.Contains(result, multilineIndicator) {
		t.Errorf("Expected multiline indicator")
	}

	if !strings.Contains(result, ansiGreen) {
		t.Errorf("Expected green color code")
	}
}

func TestCommandFormatter_FormatLogLine_PosixMode_SingleLine(t *testing.T) {
	formatter := NewCommandFormatter(true) // POSIX mode
	result := formatter.FormatLogLine("echo hello")

	if strings.Contains(result, ansiGreen) || strings.Contains(result, ansiReset) {
		t.Errorf("Should not have color codes in POSIX mode")
	}

	if !strings.Contains(result, commandPrefix) {
		t.Errorf("Expected command prefix")
	}

	if !strings.Contains(result, "echo hello") {
		t.Errorf("Expected command in output")
	}
}

func TestCommandFormatter_FormatLogLine_PosixMode_MultiLine(t *testing.T) {
	formatter := NewCommandFormatter(true) // POSIX mode
	multiLineCmd := "line1\nline2\nline3"
	result := formatter.FormatLogLine(multiLineCmd)

	if !strings.Contains(result, "line1") {
		t.Errorf("Expected first line in output")
	}

	if !strings.Contains(result, multilineIndicator) {
		t.Errorf("Expected multiline indicator")
	}

	if strings.Contains(result, ansiGreen) {
		t.Errorf("Should not have colors in POSIX mode")
	}
}

func TestCommandFormatter_GetDisplayCommand_SingleLine(t *testing.T) {
	formatter := NewCommandFormatter(false)
	result := formatter.getDisplayCommand("echo hello")

	if result != "echo hello" {
		t.Errorf("Expected unchanged single line, got: %s", result)
	}
}

func TestCommandFormatter_GetDisplayCommand_MultiLine(t *testing.T) {
	formatter := NewCommandFormatter(false)
	result := formatter.getDisplayCommand("line1\nline2")

	expected := "line1" + multilineIndicator
	if result != expected {
		t.Errorf("Expected %s, got: %s", expected, result)
	}
}

func TestIsMultiline_True(t *testing.T) {
	if !isMultiline("line1\nline2") {
		t.Errorf("Expected true for multi-line string")
	}
}

func TestIsMultiline_False(t *testing.T) {
	if isMultiline("single line") {
		t.Errorf("Expected false for single-line string")
	}
}

func TestGetFirstLine_SingleLine(t *testing.T) {
	result := getFirstLine("single line")
	if result != "single line" {
		t.Errorf("Expected 'single line', got: %s", result)
	}
}

func TestGetFirstLine_MultiLine(t *testing.T) {
	result := getFirstLine("line1\nline2\nline3")
	if result != "line1" {
		t.Errorf("Expected 'line1', got: %s", result)
	}
}

func TestGetFirstLine_Empty(t *testing.T) {
	result := getFirstLine("")
	if result != "" {
		t.Errorf("Expected empty string, got: %s", result)
	}
}

func TestCommandFormatter_ColorCodes(t *testing.T) {
	formatter := NewCommandFormatter(false)
	result := formatter.FormatLogLine("test")

	// Should use echo $'...' format for bash
	if !strings.HasPrefix(result, "echo $'") {
		t.Errorf("Expected bash ANSI-C quoting format")
	}

	if !strings.HasSuffix(result, "'") {
		t.Errorf("Expected closing quote")
	}
}

func TestCommandFormatter_PosixFormat(t *testing.T) {
	formatter := NewCommandFormatter(true)
	result := formatter.FormatLogLine("test")

	if !strings.HasPrefix(result, "echo ") {
		t.Errorf("Expected 'echo ' prefix")
	}

	// Should NOT use ANSI-C quoting ($'...')
	if strings.Contains(result, "$'") {
		t.Errorf("Should not use ANSI-C quoting in POSIX mode")
	}
}

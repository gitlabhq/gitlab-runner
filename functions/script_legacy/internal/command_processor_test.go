//go:build !integration

package internal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandProcessor_ProcessCommand_EmptyCommand(t *testing.T) {
	config := ScriptGeneratorConfig{
		ShellPath:      "/bin/bash",
		DebugTrace:     false,
		CheckForErrors: false,
		PosixEscape:    false,
		TraceSections:  false,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	processor.ProcessCommand(&buf, 0, "")
	result := buf.String()

	assert.Equal(t, "echo\n", result)
}

func TestCommandProcessor_ProcessCommand_SingleLine_NoErrors(t *testing.T) {
	config := ScriptGeneratorConfig{
		ShellPath:      "/bin/bash",
		DebugTrace:     false,
		CheckForErrors: false,
		PosixEscape:    false,
		TraceSections:  false,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	processor.ProcessCommand(&buf, 0, "echo hello")
	result := buf.String()

	assert.Contains(t, result, commandPrefix, "Expected command prefix")
	assert.Contains(t, result, "echo hello", "Expected command")
	assert.NotContains(t, result, "_runner_exit_code", "Should not have exit code check when disabled")
}

func TestCommandProcessor_ProcessCommand_SingleLine_WithErrors(t *testing.T) {
	config := ScriptGeneratorConfig{
		ShellPath:      "/bin/bash",
		DebugTrace:     false,
		CheckForErrors: true,
		PosixEscape:    false,
		TraceSections:  false,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	processor.ProcessCommand(&buf, 0, "echo test")
	result := buf.String()

	assert.Contains(t, result, exitCodeCheck, "Expected exit code check when enabled")
}

func TestCommandProcessor_ProcessCommand_MultiLine_NoTraceSections(t *testing.T) {
	config := ScriptGeneratorConfig{
		ShellPath:      "/bin/bash",
		DebugTrace:     false,
		CheckForErrors: false,
		PosixEscape:    false,
		TraceSections:  false,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	multiLine := "line1\nline2"
	processor.ProcessCommand(&buf, 0, multiLine)
	result := buf.String()

	assert.NotContains(t, result, "section_start", "Should not have section markers when trace_sections disabled")
	assert.Contains(t, result, multilineIndicator, "Expected multiline indicator")
}

func TestCommandProcessor_ProcessCommand_MultiLine_WithTraceSections(t *testing.T) {
	config := ScriptGeneratorConfig{
		ShellPath:      "/bin/bash",
		DebugTrace:     false,
		CheckForErrors: false,
		PosixEscape:    false,
		TraceSections:  true,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	multiLine := "line1\nline2"
	processor.ProcessCommand(&buf, 0, multiLine)
	result := buf.String()

	assert.Contains(t, result, "section_start", "Expected section_start marker")
	assert.Contains(t, result, "section_end", "Expected section_end marker")
}

func TestCommandProcessor_ProcessCommand_PosixMode(t *testing.T) {
	config := ScriptGeneratorConfig{
		ShellPath:      "/bin/bash",
		DebugTrace:     false,
		CheckForErrors: false,
		PosixEscape:    true,
		TraceSections:  false,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	processor.ProcessCommand(&buf, 0, "echo test")
	result := buf.String()

	assert.NotContains(t, result, EscapeForAnsiC(ansiGreen), "Should not have colors in POSIX mode")
}

func TestCommandProcessor_ProcessCommand_BashMode(t *testing.T) {
	config := ScriptGeneratorConfig{
		ShellPath:      "/bin/bash",
		DebugTrace:     false,
		CheckForErrors: false,
		PosixEscape:    false,
		TraceSections:  false,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	processor.ProcessCommand(&buf, 0, "echo test")
	result := buf.String()

	assert.Contains(t, result, EscapeForAnsiC(ansiGreen), "Expected colors in bash mode")
}

func TestCommandProcessor_ShouldUseTraceSection_True(t *testing.T) {
	config := ScriptGeneratorConfig{
		TraceSections: true,
	}
	processor := NewCommandProcessor(config)

	assert.True(t, processor.shouldUseTraceSection("line1\nline2"),
		"Expected true for multi-line with trace_sections enabled")
}

func TestCommandProcessor_ShouldUseTraceSection_False_NotMultiline(t *testing.T) {
	config := ScriptGeneratorConfig{
		TraceSections: true,
	}
	processor := NewCommandProcessor(config)

	assert.False(t, processor.shouldUseTraceSection("echo test"),
		"Expected false for single-line command")
}

func TestCommandProcessor_ShouldUseTraceSection_False_Disabled(t *testing.T) {
	config := ScriptGeneratorConfig{
		TraceSections: false,
	}
	processor := NewCommandProcessor(config)

	assert.False(t, processor.shouldUseTraceSection("line1\nline2"),
		"Expected false when trace_sections disabled")
}

func TestCommandProcessor_WriteNormalCommand_WithErrorCheck(t *testing.T) {
	config := ScriptGeneratorConfig{
		CheckForErrors: true,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	processor.writeNormalCommand(&buf, "echo test")
	result := buf.String()

	assert.Contains(t, result, exitCodeCheck, "Expected exit code check")
}

func TestCommandProcessor_WriteNormalCommand_NoErrorCheck(t *testing.T) {
	config := ScriptGeneratorConfig{
		CheckForErrors: false,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	processor.writeNormalCommand(&buf, "echo test")
	result := buf.String()

	assert.NotContains(t, result, "_runner_exit_code", "Should not have exit code check when disabled")
}

func TestCommandProcessor_AllFlags_Disabled(t *testing.T) {
	config := ScriptGeneratorConfig{
		ShellPath:      "/bin/bash",
		DebugTrace:     false,
		CheckForErrors: false,
		PosixEscape:    false,
		TraceSections:  false,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	processor.ProcessCommand(&buf, 0, "echo test")
	result := buf.String()

	assert.Contains(t, result, "echo test", "Expected command")
	assert.NotContains(t, result, "_runner_exit_code", "Should not have error checking")
	assert.NotContains(t, result, "section_start", "Should not have trace sections")
}

func TestCommandProcessor_AllFlags_Enabled(t *testing.T) {
	config := ScriptGeneratorConfig{
		ShellPath:      "/bin/bash",
		DebugTrace:     true,
		CheckForErrors: true,
		PosixEscape:    true,
		TraceSections:  true,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	processor.ProcessCommand(&buf, 0, "echo test")
	result := buf.String()

	assert.Contains(t, result, exitCodeCheck, "Expected error checking for single line")
	assert.NotContains(t, result, EscapeForAnsiC(ansiGreen), "Should not have colors in POSIX mode")
	assert.NotContains(t, result, "section_start", "Should not have trace sections for single line")

	buf.Reset()
	processor.ProcessCommand(&buf, 1, "line1\nline2")
	result2 := buf.String()

	assert.Contains(t, result2, "section_start", "Expected trace sections for multi-line")
}

func TestCommandProcessor_Integration(t *testing.T) {
	// Test that processor correctly orchestrates formatter and section writer
	config := ScriptGeneratorConfig{
		ShellPath:      "/bin/bash",
		DebugTrace:     false,
		CheckForErrors: true,
		PosixEscape:    false,
		TraceSections:  true,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	processor.ProcessCommand(&buf, 0, "echo start")
	processor.ProcessCommand(&buf, 1, "multi\nline")
	processor.ProcessCommand(&buf, 2, "")
	processor.ProcessCommand(&buf, 3, "echo end")

	result := buf.String()

	assert.Contains(t, result, "echo start", "Expected first command")
	assert.Contains(t, result, "section_start", "Expected trace section for multi-line")

	lines := strings.Split(result, "\n")
	hasEchoOnly := false
	for _, line := range lines {
		if line == "echo" {
			hasEchoOnly = true
			break
		}
	}
	assert.True(t, hasEchoOnly, "Expected 'echo' for empty command")
	assert.Contains(t, result, "echo end", "Expected last command")
}

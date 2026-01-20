//go:build !integration

package internal

import (
	"strings"
	"testing"
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

	if result != "echo\n" {
		t.Errorf("Expected 'echo\\n', got: %s", result)
	}
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

	if !strings.Contains(result, commandPrefix) {
		t.Errorf("Expected command prefix")
	}

	if !strings.Contains(result, "echo hello") {
		t.Errorf("Expected command")
	}

	if strings.Contains(result, "_runner_exit_code") {
		t.Errorf("Should not have exit code check when disabled")
	}
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

	if !strings.Contains(result, exitCodeCheck) {
		t.Errorf("Expected exit code check when enabled")
	}
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

	if strings.Contains(result, "section_start") {
		t.Errorf("Should not have section markers when trace_sections disabled")
	}

	if !strings.Contains(result, multilineIndicator) {
		t.Errorf("Expected multiline indicator")
	}
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

	// Should have trace section markers
	if !strings.Contains(result, "section_start") {
		t.Errorf("Expected section_start marker")
	}
	if !strings.Contains(result, "section_end") {
		t.Errorf("Expected section_end marker")
	}
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

	// Should NOT have ANSI colors
	if strings.Contains(result, ansiGreen) {
		t.Errorf("Should not have colors in POSIX mode")
	}
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

	if !strings.Contains(result, ansiGreen) {
		t.Errorf("Expected colors in bash mode")
	}
}

func TestCommandProcessor_ShouldUseTraceSection_True(t *testing.T) {
	config := ScriptGeneratorConfig{
		TraceSections: true,
	}
	processor := NewCommandProcessor(config)

	multiLine := "line1\nline2"
	if !processor.shouldUseTraceSection(multiLine) {
		t.Errorf("Expected true for multi-line with trace_sections enabled")
	}
}

func TestCommandProcessor_ShouldUseTraceSection_False_NotMultiline(t *testing.T) {
	config := ScriptGeneratorConfig{
		TraceSections: true,
	}
	processor := NewCommandProcessor(config)

	singleLine := "echo test"
	if processor.shouldUseTraceSection(singleLine) {
		t.Errorf("Expected false for single-line command")
	}
}

func TestCommandProcessor_ShouldUseTraceSection_False_Disabled(t *testing.T) {
	config := ScriptGeneratorConfig{
		TraceSections: false,
	}
	processor := NewCommandProcessor(config)

	multiLine := "line1\nline2"
	if processor.shouldUseTraceSection(multiLine) {
		t.Errorf("Expected false when trace_sections disabled")
	}
}

func TestCommandProcessor_WriteNormalCommand_WithErrorCheck(t *testing.T) {
	config := ScriptGeneratorConfig{
		CheckForErrors: true,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	processor.writeNormalCommand(&buf, "echo test")
	result := buf.String()

	if !strings.Contains(result, exitCodeCheck) {
		t.Errorf("Expected exit code check")
	}
}

func TestCommandProcessor_WriteNormalCommand_NoErrorCheck(t *testing.T) {
	config := ScriptGeneratorConfig{
		CheckForErrors: false,
	}
	processor := NewCommandProcessor(config)
	var buf strings.Builder

	processor.writeNormalCommand(&buf, "echo test")
	result := buf.String()

	if strings.Contains(result, "_runner_exit_code") {
		t.Errorf("Should not have exit code check when disabled")
	}
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

	if !strings.Contains(result, "echo test") {
		t.Errorf("Expected command")
	}

	if strings.Contains(result, "_runner_exit_code") {
		t.Errorf("Should not have error checking")
	}
	if strings.Contains(result, "section_start") {
		t.Errorf("Should not have trace sections")
	}
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

	if !strings.Contains(result, exitCodeCheck) {
		t.Errorf("Expected error checking for single line")
	}
	if strings.Contains(result, ansiGreen) {
		t.Errorf("Should not have colors in POSIX mode")
	}
	if strings.Contains(result, "section_start") {
		t.Errorf("Should not have trace sections for single line")
	}

	buf.Reset()
	processor.ProcessCommand(&buf, 1, "line1\nline2")
	result2 := buf.String()

	if !strings.Contains(result2, "section_start") {
		t.Errorf("Expected trace sections for multi-line")
	}
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

	if !strings.Contains(result, "echo start") {
		t.Errorf("Expected first command")
	}

	if !strings.Contains(result, "section_start") {
		t.Errorf("Expected trace section for multi-line")
	}

	lines := strings.Split(result, "\n")
	hasEchoOnly := false
	for _, line := range lines {
		if line == "echo" {
			hasEchoOnly = true
			break
		}
	}
	if !hasEchoOnly {
		t.Errorf("Expected 'echo' for empty command")
	}

	if !strings.Contains(result, "echo end") {
		t.Errorf("Expected last command")
	}
}

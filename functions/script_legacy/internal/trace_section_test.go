//go:build !integration

package internal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTraceSectionWriter_WriteSection_Basic(t *testing.T) {
	writer := NewTraceSectionWriter(false, false)
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo hello")
	result := buf.String()

	assert.Contains(t, result, "section_start:", "Expected section_start marker")
	assert.Contains(t, result, "section_end:", "Expected section_end marker")
	assert.Contains(t, result, "echo hello", "Expected command in output")
}

func TestTraceSectionWriter_WriteSection_WithErrorChecking(t *testing.T) {
	writer := NewTraceSectionWriter(true, false) // Error checking enabled
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo test")
	result := buf.String()

	assert.Contains(t, result, exitCodeCheck, "Expected exit code check when enabled")
}

func TestTraceSectionWriter_WriteSection_WithoutErrorChecking(t *testing.T) {
	writer := NewTraceSectionWriter(false, false) // Error checking disabled
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo test")
	result := buf.String()

	assert.NotContains(t, result, "_runner_exit_code", "Should not have exit code check when disabled")
}

func TestTraceSectionWriter_SectionName_Format(t *testing.T) {
	writer := NewTraceSectionWriter(false, false)
	var buf strings.Builder

	writer.WriteSection(&buf, 5, "echo test")
	result := buf.String()

	assert.Contains(t, result, "section_script_step_5", "Expected section name 'section_script_step_5'")
}

func TestTraceSectionWriter_ContainsTimestamp(t *testing.T) {
	writer := NewTraceSectionWriter(false, false)
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo test")
	result := buf.String()

	assert.Contains(t, result, timestampCommand, "Expected timestamp command")
}

func TestTraceSectionWriter_ContainsSectionMarkers(t *testing.T) {
	writer := NewTraceSectionWriter(false, false)
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo test")
	result := buf.String()

	assert.Contains(t, result, traceSectionOptions, "Expected trace section options")
	assert.Contains(t, result, "\r"+ansiClear, "Expected \\r and ANSI clear sequence")
}

func TestTraceSectionWriter_ContainsCommand(t *testing.T) {
	writer := NewTraceSectionWriter(false, false)
	var buf strings.Builder

	testCmd := "multi\nline\ncommand"
	writer.WriteSection(&buf, 0, testCmd)
	result := buf.String()

	assert.Contains(t, result, "multi", "Expected first line")
	assert.Contains(t, result, "line", "Expected second line")
	assert.Contains(t, result, "command", "Expected third line")
}

func TestTraceSectionWriter_ANSICodes(t *testing.T) {
	writer := NewTraceSectionWriter(false, false)
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo test")
	result := buf.String()

	assert.Contains(t, result, ansiClear, "Expected ANSI clear code")
	assert.Contains(t, result, EscapeForAnsiC(ansiBoldGreen), "Expected ANSI bold green code")
	assert.Contains(t, result, EscapeForAnsiC(ansiResetTrace), "Expected ANSI reset code for trace sections")
}

func TestTraceSectionWriter_commandPrefix(t *testing.T) {
	writer := NewTraceSectionWriter(false, false)
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo test")
	result := buf.String()

	assert.Contains(t, result, commandPrefix, "Expected command prefix in section start")
}

func TestTraceSectionWriter_MultipleIndexes(t *testing.T) {
	writer := NewTraceSectionWriter(false, false)

	tests := []struct {
		index    int
		expected string
	}{
		{0, "section_script_step_0"},
		{1, "section_script_step_1"},
		{99, "section_script_step_99"},
	}

	for _, tt := range tests {
		var buf strings.Builder
		writer.WriteSection(&buf, tt.index, "echo test")
		result := buf.String()

		assert.Contains(t, result, tt.expected, "Expected section name %s for index %d", tt.expected, tt.index)
	}
}

//go:build !integration

package internal

import (
	"strings"
	"testing"
)

func TestTraceSectionWriter_WriteSection_Basic(t *testing.T) {
	writer := NewTraceSectionWriter(false)
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo hello")
	result := buf.String()

	if !strings.Contains(result, "section_start:") {
		t.Errorf("Expected section_start marker")
	}

	if !strings.Contains(result, "section_end:") {
		t.Errorf("Expected section_end marker")
	}

	if !strings.Contains(result, "echo hello") {
		t.Errorf("Expected command in output")
	}
}

func TestTraceSectionWriter_WriteSection_WithErrorChecking(t *testing.T) {
	writer := NewTraceSectionWriter(true) // Error checking enabled
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo test")
	result := buf.String()

	if !strings.Contains(result, exitCodeCheck) {
		t.Errorf("Expected exit code check when enabled")
	}
}

func TestTraceSectionWriter_WriteSection_WithoutErrorChecking(t *testing.T) {
	writer := NewTraceSectionWriter(false) // Error checking disabled
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo test")
	result := buf.String()

	if strings.Contains(result, "_runner_exit_code") {
		t.Errorf("Should not have exit code check when disabled")
	}
}

func TestTraceSectionWriter_SectionName_Format(t *testing.T) {
	writer := NewTraceSectionWriter(false)
	var buf strings.Builder

	writer.WriteSection(&buf, 5, "echo test")
	result := buf.String()

	if !strings.Contains(result, "section_script_step_5") {
		t.Errorf("Expected section name 'section_script_step_5'")
	}
}

func TestTraceSectionWriter_ContainsTimestamp(t *testing.T) {
	writer := NewTraceSectionWriter(false)
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo test")
	result := buf.String()

	if !strings.Contains(result, timestampCommand) {
		t.Errorf("Expected timestamp command")
	}
}

func TestTraceSectionWriter_ContainsSectionMarkers(t *testing.T) {
	writer := NewTraceSectionWriter(false)
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo test")
	result := buf.String()

	if !strings.Contains(result, traceSectionOptions) {
		t.Errorf("Expected trace section options")
	}

	// Should have proper format with carriage return and clear line
	if !strings.Contains(result, "\\r"+ansiClear) {
		t.Errorf("Expected \\r and ANSI clear sequence")
	}
}

func TestTraceSectionWriter_ContainsCommand(t *testing.T) {
	writer := NewTraceSectionWriter(false)
	var buf strings.Builder

	testCmd := "multi\nline\ncommand"
	writer.WriteSection(&buf, 0, testCmd)
	result := buf.String()

	// Should contain the full command (not just first line)
	if !strings.Contains(result, "multi") {
		t.Errorf("Expected first line")
	}
	if !strings.Contains(result, "line") {
		t.Errorf("Expected second line")
	}
	if !strings.Contains(result, "command") {
		t.Errorf("Expected third line")
	}
}

func TestTraceSectionWriter_ANSICodes(t *testing.T) {
	writer := NewTraceSectionWriter(false)
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo test")
	result := buf.String()

	// Should contain ANSI codes for trace sections
	if !strings.Contains(result, ansiClear) {
		t.Errorf("Expected ANSI clear code")
	}
	if !strings.Contains(result, ansiBoldGreen) {
		t.Errorf("Expected ANSI bold green code")
	}
	if !strings.Contains(result, ansiResetTrace) {
		t.Errorf("Expected ANSI reset code for trace sections")
	}
}

func TestTraceSectionWriter_commandPrefix(t *testing.T) {
	writer := NewTraceSectionWriter(false)
	var buf strings.Builder

	writer.WriteSection(&buf, 0, "echo test")
	result := buf.String()

	// Should contain command prefix in section start
	// It's part of the section_start line showing what will execute
	if !strings.Contains(result, commandPrefix) {
		t.Errorf("Expected command prefix in section start")
	}
}

func TestTraceSectionWriter_MultipleIndexes(t *testing.T) {
	writer := NewTraceSectionWriter(false)

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

		if !strings.Contains(result, tt.expected) {
			t.Errorf("Expected section name %s for index %d", tt.expected, tt.index)
		}
	}
}

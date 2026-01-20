package internal

import (
	"fmt"
	"strings"
)

const (
	// ansiClear clears the line (\033[0K or \e[0K)
	ansiClear = "\\033[0K"

	// ansiBoldGreen is bold green text (\033[32;1m or \e[32;1m)
	ansiBoldGreen = "\\033[32;1m"

	// ansiResetTrace resets all attributes (\033[0;m or \e[0;m)
	ansiResetTrace = "\\033[0;m"

	// traceSectionOptions are the default options for trace sections
	// hide_duration=true: Don't show duration in the section header
	// collapsed=true: Section starts collapsed
	traceSectionOptions = "hide_duration=true,collapsed=true"

	// timestampCommand generates a Unix timestamp using awk (same as GitLab Runner)
	timestampCommand = "$(awk 'BEGIN{srand(); print srand()}')"

	// traceSectionNameFormat is the format string for section names
	traceSectionNameFormat = "script_step_%d"
)

// TraceSectionWriter writes GitLab trace section markers.
// Trace sections create collapsible sections in GitLab CI logs.
type TraceSectionWriter struct {
	checkForErrors bool
}

// NewTraceSectionWriter creates a new trace section writer.
func NewTraceSectionWriter(checkForErrors bool) *TraceSectionWriter {
	return &TraceSectionWriter{
		checkForErrors: checkForErrors,
	}
}

// WriteSection writes a command wrapped in trace section markers.
// Format: section_start → command execution → section_end
func (w *TraceSectionWriter) WriteSection(buf *strings.Builder, index int, command string) {
	sectionName := fmt.Sprintf(traceSectionNameFormat, index)

	w.writeSectionStart(buf, sectionName, command)
	w.writeCommand(buf, command)
	w.writeSectionEnd(buf, sectionName)
}

// writeSectionStart writes the section_start marker with command preview.
// Format: section_start:TIMESTAMP:section_NAME[options]\r\e[0K\e[32;1m$ COMMAND\e[0;m
func (w *TraceSectionWriter) writeSectionStart(buf *strings.Builder, sectionName, command string) {
	fmt.Fprintf(buf, "printf '%%b\\n' "+
		"\"section_start:%s:section_%s[%s]\\r%s%s%s%s%s\"\n",
		timestampCommand,
		sectionName,
		traceSectionOptions,
		ansiClear,
		ansiBoldGreen,
		commandPrefix,
		EscapeForAnsiC(command),
		ansiResetTrace)
}

// writeCommand writes the actual command and optional error checking.
func (w *TraceSectionWriter) writeCommand(buf *strings.Builder, command string) {
	buf.WriteString(command)
	buf.WriteString("\n")

	if w.checkForErrors {
		buf.WriteString(exitCodeCheck)
		buf.WriteString("\n")
	}
}

// writeSectionEnd writes the section_end marker.
// Format: section_end:TIMESTAMP:section_NAME\r\e[0K
func (w *TraceSectionWriter) writeSectionEnd(buf *strings.Builder, sectionName string) {
	fmt.Fprintf(buf, "printf '%%b\\n' "+
		"\"section_end:%s:section_%s\\r%s\"\n",
		timestampCommand,
		sectionName,
		ansiClear)
}

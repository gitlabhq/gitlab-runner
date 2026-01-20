package internal

import "strings"

// Error checking used after each command when check_for_errors is enabled.
// Matches GitLab Runner's FF_ENABLE_BASH_EXIT_CODE_CHECK behavior.
const exitCodeCheck = "_runner_exit_code=$?; if [ $_runner_exit_code -ne 0 ]; then exit $_runner_exit_code; fi"

// CommandProcessor processes individual commands and writes them to the script.
// It orchestrates the formatter and section writer based on configuration.
type CommandProcessor struct {
	formatter      *CommandFormatter
	sectionWriter  *TraceSectionWriter
	checkForErrors bool
	traceSections  bool
}

// NewCommandProcessor creates a new command processor with the given configuration.
func NewCommandProcessor(config ScriptGeneratorConfig) *CommandProcessor {
	return &CommandProcessor{
		formatter:      NewCommandFormatter(config.PosixEscape),
		sectionWriter:  NewTraceSectionWriter(config.CheckForErrors),
		checkForErrors: config.CheckForErrors,
		traceSections:  config.TraceSections,
	}
}

// ProcessCommand writes a single command to the buffer.
// Handles empty commands, trace sections, and normal commands.
func (p *CommandProcessor) ProcessCommand(buf *strings.Builder, index int, command string) {
	command = strings.TrimSpace(command)

	if command == "" {
		buf.WriteString("echo\n")
		return
	}

	if p.shouldUseTraceSection(command) {
		p.sectionWriter.WriteSection(buf, index, command)
	} else {
		p.writeNormalCommand(buf, command)
	}
}

func (p *CommandProcessor) shouldUseTraceSection(command string) bool {
	return p.traceSections && isMultiline(command)
}

func (p *CommandProcessor) writeNormalCommand(buf *strings.Builder, command string) {
	logLine := p.formatter.FormatLogLine(command)
	buf.WriteString(logLine)
	buf.WriteString("\n")

	buf.WriteString(command)
	buf.WriteString("\n")

	if p.checkForErrors {
		buf.WriteString(exitCodeCheck)
		buf.WriteString("\n")
	}
}

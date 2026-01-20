package internal

import (
	"fmt"
	"strings"
)

const (
	// ansiGreen is green text (\033[1;32m) used in bash mode
	ansiGreen = "\\033[1;32m"

	// ansiReset resets all attributes (\033[0m) used in bash mode
	ansiReset = "\\033[0m"

	// commandPrefix is shown before each command
	commandPrefix = "$ "

	// multilineIndicator is appended to the first line of collapsed multi-line commands
	multilineIndicator = " # collapsed multi-line command"
)

// CommandFormatter formats commands for logging.
type CommandFormatter struct {
	posixMode bool
}

// NewCommandFormatter creates a new command formatter.
func NewCommandFormatter(posixMode bool) *CommandFormatter {
	return &CommandFormatter{posixMode: posixMode}
}

// FormatLogLine generates the echo statement to log a command.
// Returns different formats based on POSIX mode setting.
func (f *CommandFormatter) FormatLogLine(command string) string {
	if f.posixMode {
		return f.formatPosixLogLine(command)
	}
	return f.formatBashLogLine(command)
}

// formatPosixLogLine creates a POSIX-compatible log line without colors.
func (f *CommandFormatter) formatPosixLogLine(command string) string {
	displayCmd := f.getDisplayCommand(command)
	return fmt.Sprintf("echo %s", EscapeForPosix(commandPrefix+displayCmd))
}

// formatBashLogLine creates a bash log line with ANSI colors.
func (f *CommandFormatter) formatBashLogLine(command string) string {
	displayCmd := f.getDisplayCommand(command)
	return fmt.Sprintf("echo $'%s%s%s%s'",
		ansiGreen,
		commandPrefix,
		EscapeForAnsiC(displayCmd),
		ansiReset)
}

// getDisplayCommand returns the command string to display in logs.
// For multi-line commands, returns first line with indicator.
func (f *CommandFormatter) getDisplayCommand(command string) string {
	if !isMultiline(command) {
		return command
	}

	firstLine := getFirstLine(command)
	return firstLine + multilineIndicator
}

func isMultiline(s string) bool {
	return strings.Contains(s, "\n")
}

func getFirstLine(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

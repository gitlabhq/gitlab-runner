package internal

import (
	"fmt"
	"strings"
)

const (
	ansiGreen = "\033[1;32m"
	ansiReset = "\033[0m"

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
	command = ansiGreen + commandPrefix + f.getDisplayCommand(command) + ansiReset

	if f.posixMode {
		return fmt.Sprintf("echo %s", EscapeForPosix(command))
	}

	return fmt.Sprintf("echo $'%s'", EscapeForAnsiC(command))
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

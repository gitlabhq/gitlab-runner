package internal

import "strings"

const (
	// trapTerm traps SIGTERM to prevent bash from dumping the script in logs on cancellation.
	// When a job is cancelled through the UI, GitLab Runner sends SIGTERM to all PIDs.
	// On Bash version 4+, the process termination dumps the executed script in the job logs.
	// This trap prevents that behavior and ensures clean exit.
	trapTerm = "trap exit 1 TERM"

	// setPipefailCheck conditionally enables pipefail for bash compatibility.
	// This allows the script to run in both bash and POSIX sh.
	setPipefailCheck = "if set -o | grep pipefail > /dev/null; then set -o pipefail; fi"

	// setErrexit enables exit-on-error mode.
	setErrexit = "set -o errexit"

	// setNoclobber disables noclobber to allow scripts to overwrite files with >.
	// Matches GitLab Runner's behavior for compatibility.
	setNoclobber = "set +o noclobber"

	// setXtrace enables command tracing (debug mode).
	setXtrace = "xtrace"
)

// ScriptHeader generates the script header (shebang and set options).
type ScriptHeader struct {
	shellPath  string
	debugTrace bool
}

// NewScriptHeader creates a new script header generator.
func NewScriptHeader(shellPath string, debugTrace bool) *ScriptHeader {
	return &ScriptHeader{
		shellPath:  shellPath,
		debugTrace: debugTrace,
	}
}

// Generate creates the script header with shebang and shell options.
// The header includes:
// - Shebang with detected shell path
// - SIGTERM trap (prevents script dump in logs on cancellation)
// - Conditional pipefail (for bash/sh compatibility)
// - errexit (exit on error)
// - noclobber disabled (allows file overwrites)
// - xtrace (if debug trace enabled)
func (h *ScriptHeader) Generate() string {
	var buf strings.Builder

	buf.WriteString("#!")
	buf.WriteString(h.shellPath)
	buf.WriteString("\n\n")

	buf.WriteString(trapTerm)
	buf.WriteString("\n\n")

	buf.WriteString(setPipefailCheck)
	buf.WriteString("\n")

	buf.WriteString(setErrexit)
	if h.debugTrace {
		buf.WriteString(" -o ")
		buf.WriteString(setXtrace)
	}
	buf.WriteString("\n")

	buf.WriteString(setNoclobber)
	buf.WriteString("\n\n")

	return buf.String()
}

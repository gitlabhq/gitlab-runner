package scriptwriter

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const (
	ShellPwsh       = "pwsh"
	ShellPowershell = "powershell"
	ShellBash       = "bash"
	ShellSh         = "sh"
)

// shellSearchDirs are the directories searched by absolute path when PATH has
// no shell. They match the directories probed by shells.BashDetectShellScript.
var shellSearchDirs = []string{"/usr/local/bin", "/usr/bin", "/bin", "/busybox"}

// Builder constructs shell scripts from a list of command lines,
// wrapping them with error handling, tracing, and optional
// GitLab CI section markers.
type Builder struct {
	stepName string
	shell    string

	DebugTrace     bool
	ExitCodeCheck  bool
	ScriptSections bool
	// UseLegacyBashEval restores the legacy `: | eval ...` pipeline form.
	// When false (the default), eval runs in a trapped subshell with stdin
	// closed via `< /dev/null` to suppress the script-body leak on
	// cancellation (see issue 39005).
	UseLegacyBashEval bool
}

// New creates a Builder for the given step name and shell.
func New(stepName, shell string) *Builder {
	return &Builder{stepName: stepName, shell: shell}
}

// Build renders the script lines into a complete shell script.
func (b *Builder) Build(lines []string) string {
	switch b.shell {
	case ShellPwsh, ShellPowershell:
		return b.buildPwshScript(lines)
	default:
		return b.buildBashScript(lines)
	}
}

// buildBashScript renders lines into a bash script. Each line emits, in order:
// the echoed "$ ..." trace header, the line itself, and - when ExitCodeCheck is
// set - the post-command exit-code check.
//
// Deliberately absent is any restore of the previous command's $? for the user
// command to observe (the echoed header resets it to 0). This used to emit
// "(exit $_runner_exit_code)" before each line, but under "set -o errexit" a
// standalone "(exit N)" with N != 0 aborts the script, which is exactly what
// happens at the top of an else branch - where $? carries the failed
// if-condition's status - and after a trailing "! cmd". User commands therefore
// see $?=0, matching the legacy generators (shells/abstract.go writeCommands and
// functions/script_legacy), which never emitted such a restore either.
// See gitlab-runner#39610.
//
// This differs from buildPwshScript, which does still carry $LASTEXITCODE across
// the header. That is safe there because an assignment, unlike bash's "(exit N)",
// is not a command that can fail and so cannot trip $ErrorActionPreference.
func (b *Builder) buildBashScript(lines []string) string {
	shPath, err := resolveShell(b.shell)
	if err != nil {
		shPath = "/bin/sh"
	}

	checkErr := ""
	if b.ExitCodeCheck {
		checkErr = "\n_runner_exit_code=$?; if [ $_runner_exit_code -ne 0 ]; then exit $_runner_exit_code; fi"
	}

	var body strings.Builder
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			body.WriteString("\n")
			continue
		}

		lineCheck := exitCodeCheckSuffix(line, checkErr)

		firstLine, _, multiline := strings.Cut(line, "\n")
		if multiline && b.ScriptSections {
			sectionName := fmt.Sprintf("%s_%d", b.stepName, i)
			body.WriteString(fmt.Sprintf(
				`printf "section_start:%%s:%s[hide_duration=true,collapsed=true]\r\033[0K" "$(date +%%s)"`,
				sectionName,
			) + "\n")
			body.WriteString("echo " + shellEscape(fmt.Sprintf("\033[32;1m$ %s\033[0m", line)) + "\n")
			body.WriteString(fmt.Sprintf(
				`printf "section_end:%%s:%s\r\033[0K" "$(date +%%s)"`,
				sectionName,
			) + "\n")
			body.WriteString(line + lineCheck + "\n")
			continue
		}

		if multiline {
			body.WriteString("echo " + shellEscape("\033[32;1m$ "+firstLine+" # collapsed multi-line command\033[0m") + "\n")
		} else {
			body.WriteString("echo " + shellEscape("\033[32;1m$ "+line+"\033[0m") + "\n")
		}

		body.WriteString(line + lineCheck + "\n")
	}

	var buf strings.Builder
	buf.WriteString("#!" + shPath + "\n\n")
	buf.WriteString("trap 'exit 1' TERM\n\n")
	if b.DebugTrace {
		buf.WriteString("set -o xtrace\n")
	}
	buf.WriteString("if set -o | grep pipefail > /dev/null; then set -o pipefail; fi; set -o errexit\n")
	buf.WriteString("set +o noclobber\n")
	// Run the user script in an explicit subshell with stdin closed via
	// `< /dev/null`. The TERM trap is re-installed inside this subshell so that
	// SIGTERM causes a clean exit instead of a signal-induced death that would
	// dump the eval body — including expanded secrets — to the job log on
	// cancellation (issue 39005). FF_USE_LEGACY_BASH_EVAL restores the prior
	// `: | eval ...` pipeline form as an escape hatch.
	if b.UseLegacyBashEval {
		buf.WriteString(": | eval " + shellEscape(body.String()) + "\n")
	} else {
		buf.WriteString("(trap 'exit 1' TERM; eval " + shellEscape(body.String()) + ") < /dev/null\n")
	}
	buf.WriteString("exit 0\n")

	return buf.String()
}

// exitCodeCheckSuffix returns the post-command exit-code check for lines that may
// carry it, and "" for lines that must not (see exitCodeCheckApplies). The check
// reads $? as the result of the line it follows, so it is appended per line rather
// than unconditionally. See https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/39634.
func exitCodeCheckSuffix(line, checkErr string) string {
	if !exitCodeCheckApplies(line) {
		return ""
	}
	return checkErr
}

// exitCodeCheckApplies reports whether the post-command exit-code check may be
// appended after line. The check reads $? from the last line of a multi-line
// item, so that line decides. It must not follow a bare "else" (where $? still
// holds the failed if-condition's status), a negated command "! cmd" (where $?
// is non-zero when cmd succeeds), "fi" or "done" (where $? is the status of the
// last body command, which may be such a negation), nor "until ...; do" (where
// $? is the failed condition's status). Failures that "set -o errexit" ignores,
// such as "false && true", must still abort: that is the purpose of the check.
// After "then" and "while ...; do", $? is 0, because the body runs only when
// the condition succeeded.
// See https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/39634.
func exitCodeCheckApplies(line string) bool {
	fields := strings.Fields(line[strings.LastIndexByte(line, '\n')+1:])
	if len(fields) == 0 {
		return true
	}
	switch fields[0] {
	case "!", "fi", "done":
		return false
	case "until":
		return fields[len(fields)-1] != "do"
	case "else":
		return len(fields) > 1 && !strings.HasPrefix(fields[1], "#")
	}
	return true
}

func (b *Builder) buildPwshScript(lines []string) string {
	shPath, err := exec.LookPath(b.shell)
	if err != nil {
		shPath = b.shell
	}

	eol := "\r\n"
	if runtime.GOOS != "windows" {
		eol = "\n"
	}

	checkErr := eol + "if(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }" + eol

	var body strings.Builder
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			body.WriteString(eol)
			continue
		}

		body.WriteString("$_runner_exit_code = $LASTEXITCODE" + eol)

		firstLine, _, multiline := strings.Cut(line, "\n")
		if multiline && b.ScriptSections {
			sectionName := fmt.Sprintf("%s_%d", b.stepName, i)
			body.WriteString(fmt.Sprintf(
				`Write-Host -NoNewline "section_start:$([DateTimeOffset]::Now.ToUnixTimeSeconds()):%s[hide_duration=true,collapsed=true]`+"`r"+`"`,
				sectionName,
			) + eol)
			body.WriteString("echo " + psQuoteVariable(fmt.Sprintf("\033[32;1m$ %s\033[0m", line)) + eol)
			body.WriteString(fmt.Sprintf(
				`Write-Host -NoNewline "section_end:$([DateTimeOffset]::Now.ToUnixTimeSeconds()):%s`+"`r"+`"`,
				sectionName,
			) + eol)
			body.WriteString("$global:LASTEXITCODE = $_runner_exit_code" + eol)
			body.WriteString(line + checkErr)
			continue
		}

		displayLine := line
		if multiline {
			displayLine = firstLine + " # collapsed multi-line command"
		}
		body.WriteString("echo " + psQuoteVariable("\033[32;1m$ "+displayLine+"\033[0m") + eol)

		body.WriteString("$global:LASTEXITCODE = $_runner_exit_code" + eol)
		body.WriteString(line + checkErr)
	}

	var buf strings.Builder

	if runtime.GOOS != "windows" {
		buf.WriteString("#!" + shPath + eol)
	}

	buf.WriteString("& {" + eol + eol)
	if b.DebugTrace {
		buf.WriteString("Set-PSDebug -Trace 2" + eol)
	}
	buf.WriteString(`$ErrorActionPreference = "Stop"` + eol)
	buf.WriteString(body.String() + eol)
	buf.WriteString("}" + eol + eol)

	return buf.String()
}

// --- shell resolution ---

// resolveShell returns the absolute path of the POSIX shell to run. "sh" only
// matches sh. Any other value, including an unknown one, prefers bash and
// falls back to sh. PATH is searched first, then shellSearchDirs.
func resolveShell(name string) (string, error) {
	names := []string{ShellBash, ShellSh}
	if name == ShellSh {
		names = []string{ShellSh}
	}

	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p, nil
		}
	}

	for _, n := range names {
		for _, dir := range shellSearchDirs {
			if p := dir + "/" + n; isExecutableFile(p) {
				return p, nil
			}
		}
	}

	return "", fmt.Errorf("shell not found")
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// --- string escaping ---

var psReplacer = strings.NewReplacer(
	"`", "``",
	"\a", "`a",
	"\b", "`b",
	"\f", "`f",
	"\r", "`r",
	"\n", "`n",
	"\t", "`t",
	"\v", "`v",
	"#", "`#",
	"'", "`'",
	`"`, "`\"",
	"$", "`$",
	"\u201c", "`\u201c",
	"\u201d", "`\u201d",
	"\u201e", "`\u201e",
)

func psQuoteVariable(text string) string {
	return `"` + psReplacer.Replace(text) + `"`
}

func shellEscape(input string) string {
	if input == "" {
		return "''"
	}

	var sb strings.Builder
	sb.Grow(len(input) * 2)

	needsQuoting := false
	for _, c := range []byte(input) {
		switch c {
		case '`':
			sb.WriteString("\\`")
			needsQuoting = true
		case '"':
			sb.WriteString(`\"`)
			needsQuoting = true
		case '\\':
			sb.WriteString(`\\`)
			needsQuoting = true
		case '$':
			sb.WriteString(`\$`)
			needsQuoting = true
		case ' ', '!', '#', '%', '&', '(', ')', '*', '<', '=', '>', '?', '[', '|':
			sb.WriteByte(c)
			needsQuoting = true
		default:
			sb.WriteByte(c)
		}
	}

	if needsQuoting {
		return `"` + sb.String() + `"`
	}

	return sb.String()
}

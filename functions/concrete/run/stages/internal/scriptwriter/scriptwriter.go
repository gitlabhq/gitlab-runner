package scriptwriter

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var bashFallbackPaths = []string{
	"/usr/local/bin/bash",
	"/usr/bin/bash",
	"/bin/bash",
	"/usr/local/bin/sh",
	"/usr/bin/sh",
	"/bin/sh",
	"/busybox/sh",
}

// Builder constructs shell scripts from a list of command lines,
// wrapping them with error handling, tracing, and optional
// GitLab CI section markers.
type Builder struct {
	stepName string
	shell    string

	DebugTrace     bool
	ExitCodeCheck  bool
	ScriptSections bool
}

// New creates a Builder for the given step name and shell.
func New(stepName, shell string) *Builder {
	return &Builder{stepName: stepName, shell: shell}
}

// Build renders the script lines into a complete shell script.
func (b *Builder) Build(lines []string) string {
	switch b.shell {
	case "pwsh", "powershell":
		return b.buildPwshScript(lines)
	default:
		return b.buildBashScript(lines)
	}
}

func (b *Builder) buildBashScript(lines []string) string {
	shPath, err := shellPath(b.shell)
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

		body.WriteString("_runner_exit_code=$?\n")

		nlIdx := strings.Index(line, "\n")
		if nlIdx != -1 && b.ScriptSections {
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
			body.WriteString("(exit $_runner_exit_code)\n")
			body.WriteString(line + checkErr + "\n")
			continue
		}

		if nlIdx != -1 {
			body.WriteString("echo " + shellEscape("\033[32;1m$ "+line[:nlIdx]+" # collapsed multi-line command\033[0m") + "\n")
		} else {
			body.WriteString("echo " + shellEscape("\033[32;1m$ "+line+"\033[0m") + "\n")
		}

		body.WriteString("(exit $_runner_exit_code)\n")
		body.WriteString(line + checkErr + "\n")
	}

	var buf strings.Builder
	buf.WriteString("#!" + shPath + "\n\n")
	buf.WriteString("trap exit 1 TERM\n\n")
	if b.DebugTrace {
		buf.WriteString("set -o xtrace\n")
	}
	buf.WriteString("if set -o | grep pipefail > /dev/null; then set -o pipefail; fi; set -o errexit\n")
	buf.WriteString("set +o noclobber\n")
	buf.WriteString(": | (eval " + shellEscape(body.String()) + ")\n")
	buf.WriteString("exit 0\n")

	return buf.String()
}

func (b *Builder) buildPwshScript(lines []string) string {
	shPath, err := shellPath(b.shell)
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

		nlIdx := strings.Index(line, "\n")
		if nlIdx != -1 && b.ScriptSections {
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
		if nlIdx != -1 {
			displayLine = line[:nlIdx] + " # collapsed multi-line command"
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

func resolveBash() (string, error) {
	for _, name := range []string{"bash", "sh"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}

	for _, p := range bashFallbackPaths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}

	return "", fmt.Errorf("shell not found")
}

func shellPath(name string) (string, error) {
	switch name {
	case "pwsh", "powershell":
		return exec.LookPath(name)
	default:
		return resolveBash()
	}
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

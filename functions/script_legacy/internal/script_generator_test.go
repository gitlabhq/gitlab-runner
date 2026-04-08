//go:build !integration

package internal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateScript_Basic(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, ShellPath: "/bin/bash"})

	commands := []string{"echo hello", "echo world"}
	script := gen.GenerateScript(commands)

	assert.True(t, strings.HasPrefix(script, "#!/bin/bash\n"), "Script should start with bash shebang")
	assert.Contains(t, script, "set -o errexit", "Script should set errexit")
	assert.Contains(t, script, "if set -o | grep pipefail", "Script should conditionally set pipefail for sh compatibility")
	assert.Contains(t, script, "echo hello", "Script should contain 'echo hello' command")
	assert.Contains(t, script, "echo world", "Script should contain 'echo world' command")
	assert.Contains(t, script, EscapeForAnsiC(ansiGreen), "Script should contain ANSI color codes for logging")
}

func TestGenerateScript_EmptyLines(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, ShellPath: "/bin/bash"})

	commands := []string{"echo first", "", "echo last"}
	script := gen.GenerateScript(commands)

	lines := strings.Split(script, "\n")
	foundEmptyEcho := false
	for _, line := range lines {
		if line == "echo" {
			foundEmptyEcho = true
			break
		}
	}

	assert.True(t, foundEmptyEcho, "Script should contain 'echo' for empty command")
}

func TestGenerateScript_BasicBehavior(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, ShellPath: "/bin/bash"})

	commands := []string{"echo test"}
	script := gen.GenerateScript(commands)

	assert.Contains(t, script, "set +o noclobber", "Script should disable noclobber (GitLab Runner compatibility)")
	assert.Contains(t, script, "trap exit 1 TERM", "Script should contain SIGTERM trap")
	assert.NotContains(t, script, "set -o xtrace", "Script should not use xtrace when debug_trace is false")
}

func TestGenerateScript_SecurityFeatures(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, ShellPath: "/bin/bash"})

	commands := []string{"echo \"\x1b[2J\x1b[HCleared!\""}
	script := gen.GenerateScript(commands)

	assert.Contains(t, script, "\\x1b", "Script should hex-escape ANSI escape sequences to prevent terminal manipulation")

	requiredFeatures := []struct {
		feature string
		desc    string
	}{
		{"trap exit 1 TERM", "Prevents script dump on cancellation"},
		{"set +o noclobber", "Allows file overwrites"},
		{"set -o errexit", "Exit on error"},
	}

	for _, rf := range requiredFeatures {
		assert.Contains(t, script, rf.feature, "Missing security feature: %s", rf.desc)
	}
}

func TestGenerateScript_DebugTraceEnabled(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: true, ShellPath: "/bin/bash"})

	commands := []string{"echo test"}
	script := gen.GenerateScript(commands)

	assert.Contains(t, script, "set -o errexit -o xtrace", "Script should include errexit and xtrace when debug_trace is true")
	assert.Contains(t, script, "if set -o | grep pipefail", "Script should conditionally set pipefail")
	assert.Contains(t, script, EscapeForAnsiC(ansiGreen), "Script should still have ANSI color logging when debug_trace is true")
}

func TestGenerateScript_DebugTraceDisabled(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, ShellPath: "/bin/bash"})

	commands := []string{"echo test"}
	script := gen.GenerateScript(commands)

	assert.NotContains(t, script, "-o xtrace", "Script should not include xtrace when debug_trace is false")
	assert.Contains(t, script, "set -o errexit", "Script should set errexit")
	assert.Contains(t, script, "if set -o | grep pipefail", "Script should conditionally set pipefail for sh compatibility")
}

func TestGenerateScript_MultilineCommand(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, ShellPath: "/bin/bash"})

	multilineCmd := "echo line1\necho line2\necho line3"
	commands := []string{multilineCmd}
	script := gen.GenerateScript(commands)

	assert.Contains(t, script, multilineCmd, "Script should contain full multiline command")
	assert.Contains(t, script, "collapsed multi-line command", "Script should indicate collapsed multi-line command in log")
	assert.Contains(t, script, "echo line1", "Script log should show first line of multi-line command")
}

func TestGenerateScript_CheckForErrors_Disabled(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, CheckForErrors: false})

	commands := []string{"echo hello", "echo world"}
	script := gen.GenerateScript(commands)

	assert.NotContains(t, script, "_runner_exit_code", "Script should not contain exit code checks when check_for_errors is false")
}

func TestGenerateScript_CheckForErrors_Enabled(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, CheckForErrors: true})

	commands := []string{"echo hello", "echo world"}
	script := gen.GenerateScript(commands)

	assert.Contains(t, script, "_runner_exit_code=$?; if [ $_runner_exit_code -ne 0 ]; then exit $_runner_exit_code; fi",
		"Script should contain exit code checks when check_for_errors is true")

	count := strings.Count(script, "_runner_exit_code=$?")
	assert.Equal(t, 2, count, "Expected 2 exit code checks for 2 commands")
}

func TestGenerateScript_CheckForErrors_WithDebugTrace(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: true, CheckForErrors: true})

	commands := []string{"echo test"}
	script := gen.GenerateScript(commands)

	assert.Contains(t, script, "set -o errexit -o xtrace", "Script should include xtrace when debug_trace is true")
	assert.Contains(t, script, "_runner_exit_code=$?", "Script should contain exit code checks when check_for_errors is true")
}

func TestGenerateScript_CheckForErrors_EmptyCommand(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, CheckForErrors: true})

	commands := []string{"echo first", "", "echo last"}
	script := gen.GenerateScript(commands)

	lines := strings.Split(script, "\n")
	foundEcho := false
	foundExitCheckAfterEcho := false

	for i, line := range lines {
		if line == "echo" {
			foundEcho = true
			if i+1 < len(lines) && strings.Contains(lines[i+1], "_runner_exit_code") {
				foundExitCheckAfterEcho = true
			}
		}
	}

	assert.True(t, foundEcho, "Script should contain plain 'echo' for empty command")
	assert.False(t, foundExitCheckAfterEcho, "Empty commands should not have exit code checks")
	assert.Contains(t, script, "_runner_exit_code", "Non-empty commands should have exit code checks")
}

func TestGenerateScript_PosixEscape_Disabled(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, PosixEscape: false})

	commands := []string{"echo hello"}
	script := gen.GenerateScript(commands)

	assert.Contains(t, script, EscapeForAnsiC(ansiGreen), "Script should use ANSI-C quoting with colors when posix_escape is false")
	assert.Contains(t, script, EscapeForAnsiC(ansiReset), "Script should include color reset code when posix_escape is false")
}

func TestGenerateScript_PosixEscape_Enabled(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, PosixEscape: true})

	commands := []string{"echo hello"}
	script := gen.GenerateScript(commands)

	assert.NotContains(t, script, "$'\\033", "Script should not use ANSI-C quoting when posix_escape is true")
	assert.NotContains(t, script, "\\033", "Script should not include ANSI color codes when posix_escape is true")
	assert.Contains(t, script, "echo", "Script should contain echo statement")
	assert.Contains(t, script, "hello", "Script should contain the command")
}

func TestGenerateScript_PosixEscape_SpecialChars(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, PosixEscape: true})

	commands := []string{`echo "test" $VAR`}
	script := gen.GenerateScript(commands)

	assert.Contains(t, script, `\"`, "Script should escape double quotes in POSIX mode")
	assert.Contains(t, script, `\$`, "Script should escape dollar signs in POSIX mode")
}

func TestGenerateScript_PosixEscape_Multiline(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, PosixEscape: true})

	multilineCmd := "echo line1\necho line2"
	commands := []string{multilineCmd}
	script := gen.GenerateScript(commands)

	assert.Contains(t, script, "echo line1", "Script should contain first line of command")
	assert.Contains(t, script, "collapsed multi-line command", "Script should indicate collapsed multi-line command")
	assert.NotContains(t, script, "\\033", "Script should not contain ANSI codes in POSIX mode")
}

func TestGenerateScript_ShellShebang(t *testing.T) {
	tests := []struct {
		name      string
		shellPath string
		expected  string
	}{
		{
			name:      "bash shebang",
			shellPath: "/bin/bash",
			expected:  "#!/bin/bash\n",
		},
		{
			name:      "sh shebang",
			shellPath: "/bin/sh",
			expected:  "#!/bin/sh\n",
		},
		{
			name:      "custom path",
			shellPath: "/usr/local/bin/bash",
			expected:  "#!/usr/local/bin/bash\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewScriptGenerator(ScriptGeneratorConfig{ShellPath: tt.shellPath})
			commands := []string{"echo test"}
			script := gen.GenerateScript(commands)

			assert.True(t, strings.HasPrefix(script, tt.expected),
				"Expected shebang %q, but script starts with: %q", tt.expected, script[:min(len(script), 50)])
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestGenerateScript_TraceSections_Multiline(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{
		DebugTrace:    false,
		TraceSections: true,
		ShellPath:     "/bin/bash",
	})

	multilineCmd := "echo line1\necho line2\necho line3"
	commands := []string{multilineCmd}
	script := gen.GenerateScript(commands)

	assert.Contains(t, script, "section_start:", "Script should contain section_start marker with trace_sections enabled")
	assert.Contains(t, script, "section_end:", "Script should contain section_end marker with trace_sections enabled")
	assert.Contains(t, script, "section_script_step_0", "Script should contain section name with trace_sections enabled")
	assert.Contains(t, script, "[hide_duration=true,collapsed=true]", "Script should contain section options with trace_sections enabled")
	assert.Contains(t, script, "printf", "Script should use printf for section markers")
	assert.Contains(t, script, "awk 'BEGIN{srand(); print srand()}'", "Script should use awk for timestamp generation")
	assert.NotContains(t, script, "collapsed multi-line command", "Script should not show collapsed message when trace_sections enabled")
}

func TestGenerateScript_TraceSections_Disabled_Multiline(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{
		DebugTrace:    false,
		TraceSections: false,
		ShellPath:     "/bin/bash",
	})

	multilineCmd := "echo line1\necho line2\necho line3"
	commands := []string{multilineCmd}
	script := gen.GenerateScript(commands)

	assert.NotContains(t, script, "section_start:", "Script should not contain section_start marker when trace_sections disabled")
	assert.NotContains(t, script, "section_end:", "Script should not contain section_end marker when trace_sections disabled")
	assert.Contains(t, script, "collapsed multi-line command", "Script should show collapsed message when trace_sections disabled")
}

func TestGenerateScript_TraceSections_SingleLine(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{
		DebugTrace:    false,
		TraceSections: true,
		ShellPath:     "/bin/bash",
	})

	commands := []string{"echo hello"}
	script := gen.GenerateScript(commands)

	assert.NotContains(t, script, "section_start:", "Script should not contain section markers for single-line commands")
	assert.NotContains(t, script, "section_end:", "Script should not contain section markers for single-line commands")
	assert.Contains(t, script, "echo hello", "Script should contain the command")
}

func TestGenerateScript_TraceSections_MultipleCommands(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{
		DebugTrace:    false,
		TraceSections: true,
		ShellPath:     "/bin/bash",
	})

	commands := []string{
		"echo single",
		"echo multi1\necho multi2",
		"echo another",
		"echo multi3\necho multi4",
	}
	script := gen.GenerateScript(commands)

	assert.Contains(t, script, "section_script_step_1", "Script should have section for second command (index 1)")
	assert.Contains(t, script, "section_script_step_3", "Script should have section for fourth command (index 3)")
	assert.NotContains(t, script, "section_script_step_0", "Script should not have section for first command (single-line)")
	assert.NotContains(t, script, "section_script_step_2", "Script should not have section for third command (single-line)")
	assert.Contains(t, script, "echo single", "Script should contain first command")
	assert.Contains(t, script, "echo another", "Script should contain third command")
}

func TestGenerateScript_GitLabEnvFile_NotSet(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{ShellPath: "/bin/bash"})

	script := gen.GenerateScript([]string{"echo hello"})

	assert.NotContains(t, script, "GITLAB_ENV", "Script should not reference GITLAB_ENV when GitLabEnvFile is not set")
}

func TestGenerateScript_GitLabEnvFile_Set(t *testing.T) {
	envFile := "/builds/project.tmp/gitlab_runner_env"
	gen := NewScriptGenerator(ScriptGeneratorConfig{
		ShellPath:     "/bin/bash",
		GitLabEnvFile: envFile,
	})

	script := gen.GenerateScript([]string{"echo hello"})

	assert.Contains(t, script, `export GITLAB_ENV="`+envFile+`"`,
		"Script should export GITLAB_ENV=%q", envFile)
	assert.Contains(t, script, `while read -r line; do export "$line" || true; done`,
		"Script should source the GITLAB_ENV file using a read loop")

	preamblePos := strings.Index(script, "GITLAB_ENV")
	cmdPos := strings.Index(script, "echo hello")
	assert.Less(t, preamblePos, cmdPos, "GITLAB_ENV preamble should appear before user commands in the script")
}

func TestGenerateScript_GitLabEnvFile_PreambleAfterHeader(t *testing.T) {
	envFile := "/tmp/gitlab_runner_env"
	gen := NewScriptGenerator(ScriptGeneratorConfig{
		ShellPath:     "/bin/bash",
		GitLabEnvFile: envFile,
	})

	script := gen.GenerateScript([]string{"echo test"})

	headerPos := strings.Index(script, "set -o errexit")
	preamblePos := strings.Index(script, "export GITLAB_ENV")
	assert.Less(t, headerPos, preamblePos, "Shell header options should appear before the GITLAB_ENV preamble")
}

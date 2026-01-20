//go:build !integration

package internal

import (
	"strings"
	"testing"
)

func TestGenerateScript_Basic(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, ShellPath: "/bin/bash"})

	commands := []string{"echo hello", "echo world"}
	script := gen.GenerateScript(commands)

	if !strings.HasPrefix(script, "#!/bin/bash\n") {
		t.Errorf("Script should start with bash shebang")
	}

	if !strings.Contains(script, "set -o errexit") {
		t.Errorf("Script should set errexit")
	}
	if !strings.Contains(script, "if set -o | grep pipefail") {
		t.Errorf("Script should conditionally set pipefail for sh compatibility")
	}

	if !strings.Contains(script, "echo hello") {
		t.Errorf("Script should contain 'echo hello' command")
	}
	if !strings.Contains(script, "echo world") {
		t.Errorf("Script should contain 'echo world' command")
	}

	if !strings.Contains(script, "\\033[1;32m") {
		t.Errorf("Script should contain ANSI color codes for logging")
	}
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

	if !foundEmptyEcho {
		t.Errorf("Script should contain 'echo' for empty command")
	}
}

func TestGenerateScript_BasicBehavior(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, ShellPath: "/bin/bash"})

	commands := []string{"echo test"}
	script := gen.GenerateScript(commands)

	if !strings.Contains(script, "set +o noclobber") {
		t.Errorf("Script should disable noclobber (GitLab Runner compatibility)")
	}

	if !strings.Contains(script, "trap exit 1 TERM") {
		t.Errorf("Script should contain SIGTERM trap")
	}

	if strings.Contains(script, "set -o xtrace") {
		t.Errorf("Script should not use xtrace when debug_trace is false")
	}
}

func TestGenerateScript_SecurityFeatures(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, ShellPath: "/bin/bash"})

	commands := []string{"echo \"\x1b[2J\x1b[HCleared!\""}
	script := gen.GenerateScript(commands)

	if !strings.Contains(script, "\\x1b") {
		t.Errorf("Script should hex-escape ANSI escape sequences to prevent terminal manipulation")
	}

	requiredFeatures := []string{
		"trap exit 1 TERM", // Prevents script dump on cancellation
		"set +o noclobber", // Allows file overwrites
		"set -o errexit",   // Exit on error
	}

	for _, feature := range requiredFeatures {
		if !strings.Contains(script, feature) {
			t.Errorf("Missing security feature: %s", feature)
		}
	}
}

func TestGenerateScript_DebugTraceEnabled(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: true, ShellPath: "/bin/bash"})

	commands := []string{"echo test"}
	script := gen.GenerateScript(commands)

	if !strings.Contains(script, "set -o errexit -o xtrace") {
		t.Errorf("Script should include errexit and xtrace when debug_trace is true")
	}

	if !strings.Contains(script, "if set -o | grep pipefail") {
		t.Errorf("Script should conditionally set pipefail")
	}

	if !strings.Contains(script, "\\033[1;32m") {
		t.Errorf("Script should still have ANSI color logging when debug_trace is true")
	}
}

func TestGenerateScript_DebugTraceDisabled(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, ShellPath: "/bin/bash"})

	commands := []string{"echo test"}
	script := gen.GenerateScript(commands)

	if strings.Contains(script, "-o xtrace") {
		t.Errorf("Script should not include xtrace when debug_trace is false")
	}

	if !strings.Contains(script, "set -o errexit") {
		t.Errorf("Script should set errexit")
	}
	if !strings.Contains(script, "if set -o | grep pipefail") {
		t.Errorf("Script should conditionally set pipefail for sh compatibility")
	}
}

func TestGenerateScript_MultilineCommand(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, ShellPath: "/bin/bash"})

	multilineCmd := "echo line1\necho line2\necho line3"
	commands := []string{multilineCmd}
	script := gen.GenerateScript(commands)

	if !strings.Contains(script, multilineCmd) {
		t.Errorf("Script should contain full multiline command")
	}

	if !strings.Contains(script, "collapsed multi-line command") {
		t.Errorf("Script should indicate collapsed multi-line command in log")
	}

	if !strings.Contains(script, "echo line1") {
		t.Errorf("Script log should show first line of multi-line command")
	}
}

func TestGenerateScript_CheckForErrors_Disabled(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, CheckForErrors: false})

	commands := []string{"echo hello", "echo world"}
	script := gen.GenerateScript(commands)

	if strings.Contains(script, "_runner_exit_code") {
		t.Errorf("Script should not contain exit code checks when check_for_errors is false")
	}
}

func TestGenerateScript_CheckForErrors_Enabled(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, CheckForErrors: true})

	commands := []string{"echo hello", "echo world"}
	script := gen.GenerateScript(commands)

	if !strings.Contains(script, "_runner_exit_code=$?; if [ $_runner_exit_code -ne 0 ]; then exit $_runner_exit_code; fi") {
		t.Errorf("Script should contain exit code checks when check_for_errors is true")
	}

	// Count how many times the check appears (should be 2 for 2 commands)
	count := strings.Count(script, "_runner_exit_code=$?")
	if count != 2 {
		t.Errorf("Expected 2 exit code checks for 2 commands, got %d", count)
	}
}

func TestGenerateScript_CheckForErrors_WithDebugTrace(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: true, CheckForErrors: true})

	commands := []string{"echo test"}
	script := gen.GenerateScript(commands)

	if !strings.Contains(script, "set -o errexit -o xtrace") {
		t.Errorf("Script should include xtrace when debug_trace is true")
	}

	if !strings.Contains(script, "_runner_exit_code=$?") {
		t.Errorf("Script should contain exit code checks when check_for_errors is true")
	}
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
			// Check if next line is NOT the exit code check
			if i+1 < len(lines) && strings.Contains(lines[i+1], "_runner_exit_code") {
				foundExitCheckAfterEcho = true
			}
		}
	}

	if !foundEcho {
		t.Errorf("Script should contain plain 'echo' for empty command")
	}

	if foundExitCheckAfterEcho {
		t.Errorf("Empty commands should not have exit code checks")
	}

	if !strings.Contains(script, "_runner_exit_code") {
		t.Errorf("Non-empty commands should have exit code checks")
	}
}

func TestGenerateScript_PosixEscape_Disabled(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, PosixEscape: false})

	commands := []string{"echo hello"}
	script := gen.GenerateScript(commands)

	if !strings.Contains(script, "echo $'\\033[1;32m$") {
		t.Errorf("Script should use ANSI-C quoting with colors when posix_escape is false")
	}

	if !strings.Contains(script, "\\033[0m'") {
		t.Errorf("Script should include color reset code when posix_escape is false")
	}
}

func TestGenerateScript_PosixEscape_Enabled(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, PosixEscape: true})

	commands := []string{"echo hello"}
	script := gen.GenerateScript(commands)

	if strings.Contains(script, "$'\\033") {
		t.Errorf("Script should not use ANSI-C quoting when posix_escape is true")
	}

	if strings.Contains(script, "\\033") {
		t.Errorf("Script should not include ANSI color codes when posix_escape is true")
	}

	if !strings.Contains(script, "echo") {
		t.Errorf("Script should contain echo statement")
	}

	if !strings.Contains(script, "hello") {
		t.Errorf("Script should contain the command")
	}
}

func TestGenerateScript_PosixEscape_SpecialChars(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, PosixEscape: true})

	commands := []string{`echo "test" $VAR`}
	script := gen.GenerateScript(commands)

	if !strings.Contains(script, `\"`) {
		t.Errorf("Script should escape double quotes in POSIX mode")
	}

	if !strings.Contains(script, `\$`) {
		t.Errorf("Script should escape dollar signs in POSIX mode")
	}
}

func TestGenerateScript_PosixEscape_Multiline(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{DebugTrace: false, PosixEscape: true})

	multilineCmd := "echo line1\necho line2"
	commands := []string{multilineCmd}
	script := gen.GenerateScript(commands)

	if !strings.Contains(script, "echo line1") {
		t.Errorf("Script should contain first line of command")
	}

	if !strings.Contains(script, "collapsed multi-line command") {
		t.Errorf("Script should indicate collapsed multi-line command")
	}

	if strings.Contains(script, "\\033") {
		t.Errorf("Script should not contain ANSI codes in POSIX mode")
	}
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

			if !strings.HasPrefix(script, tt.expected) {
				t.Errorf("Expected shebang %q, but script starts with: %q",
					tt.expected, script[:min(len(script), 50)])
			}
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

	if !strings.Contains(script, "section_start:") {
		t.Errorf("Script should contain section_start marker with trace_sections enabled")
	}

	if !strings.Contains(script, "section_end:") {
		t.Errorf("Script should contain section_end marker with trace_sections enabled")
	}

	if !strings.Contains(script, "section_script_step_0") {
		t.Errorf("Script should contain section name with trace_sections enabled")
	}

	if !strings.Contains(script, "[hide_duration=true,collapsed=true]") {
		t.Errorf("Script should contain section options with trace_sections enabled")
	}

	if !strings.Contains(script, "printf") {
		t.Errorf("Script should use printf for section markers")
	}

	if !strings.Contains(script, "awk 'BEGIN{srand(); print srand()}'") {
		t.Errorf("Script should use awk for timestamp generation")
	}

	if strings.Contains(script, "collapsed multi-line command") {
		t.Errorf("Script should not show collapsed message when trace_sections enabled")
	}
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

	if strings.Contains(script, "section_start:") {
		t.Errorf("Script should not contain section_start marker when trace_sections disabled")
	}

	if strings.Contains(script, "section_end:") {
		t.Errorf("Script should not contain section_end marker when trace_sections disabled")
	}

	if !strings.Contains(script, "collapsed multi-line command") {
		t.Errorf("Script should show collapsed message when trace_sections disabled")
	}
}

func TestGenerateScript_TraceSections_SingleLine(t *testing.T) {
	gen := NewScriptGenerator(ScriptGeneratorConfig{
		DebugTrace:    false,
		TraceSections: true,
		ShellPath:     "/bin/bash",
	})

	commands := []string{"echo hello"}
	script := gen.GenerateScript(commands)

	if strings.Contains(script, "section_start:") {
		t.Errorf("Script should not contain section markers for single-line commands")
	}

	if strings.Contains(script, "section_end:") {
		t.Errorf("Script should not contain section markers for single-line commands")
	}

	if !strings.Contains(script, "echo hello") {
		t.Errorf("Script should contain the command")
	}
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

	if !strings.Contains(script, "section_script_step_1") {
		t.Errorf("Script should have section for second command (index 1)")
	}

	if !strings.Contains(script, "section_script_step_3") {
		t.Errorf("Script should have section for fourth command (index 3)")
	}

	if strings.Contains(script, "section_script_step_0") {
		t.Errorf("Script should not have section for first command (single-line)")
	}

	if strings.Contains(script, "section_script_step_2") {
		t.Errorf("Script should not have section for third command (single-line)")
	}

	if !strings.Contains(script, "echo single") {
		t.Errorf("Script should contain first command")
	}
	if !strings.Contains(script, "echo another") {
		t.Errorf("Script should contain third command")
	}
}

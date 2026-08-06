//go:build !integration

package scriptwriter

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireShell skips the test if the given shell is not available on the system.
func requireShell(t *testing.T, shell string) {
	t.Helper()
	switch shell {
	case ShellBash, ShellSh:
		if _, err := resolveBash(); err != nil {
			t.Skipf("skipping: no POSIX shell available")
		}
	case ShellPwsh, ShellPowershell:
		if _, err := exec.LookPath(shell); err != nil {
			t.Skipf("skipping: %s not available", shell)
		}
	}
}

func newBuilder(shell string, opts ...func(*Builder)) *Builder {
	b := New("test_step", shell)
	for _, o := range opts {
		o(b)
	}
	return b
}

func withExitCodeCheck(b *Builder)     { b.ExitCodeCheck = true }
func withDebugTrace(b *Builder)        { b.DebugTrace = true }
func withScriptSections(b *Builder)    { b.ScriptSections = true }
func withUseLegacyBashEval(b *Builder) { b.UseLegacyBashEval = true }

func TestBashScript(t *testing.T) {
	requireShell(t, ShellBash)

	tests := map[string]struct {
		lines  []string
		opts   []func(*Builder)
		assert func(t *testing.T, script string)
	}{
		"structure": {
			lines: []string{"echo hello"},
			assert: func(t *testing.T, s string) {
				assert.True(t, strings.HasPrefix(s, "#!"))
				for _, want := range []string{"set -o errexit", "set +o noclobber", "trap 'exit 1' TERM", "eval", "exit 0"} {
					assert.Contains(t, s, want)
				}
			},
		},
		"exit code check enabled": {
			lines: []string{"echo a"},
			opts:  []func(*Builder){withExitCodeCheck},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "then exit")
			},
		},
		"exit code check disabled": {
			lines: []string{"echo a"},
			assert: func(t *testing.T, s string) {
				assert.NotContains(t, s, "then exit")
			},
		},
		"debug trace enabled": {
			lines: []string{"echo a"},
			opts:  []func(*Builder){withDebugTrace},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "set -o xtrace")
			},
		},
		"debug trace disabled": {
			lines: []string{"echo a"},
			assert: func(t *testing.T, s string) {
				assert.NotContains(t, s, "set -o xtrace")
			},
		},
		"default wraps eval in a trapped subshell with stdin from /dev/null": {
			lines: []string{"echo a"},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "(trap 'exit 1' TERM; eval ")
				assert.Contains(t, s, ") < /dev/null")
				assert.NotRegexp(t, `:\s*\|\s*eval `, s,
					"default must not use the legacy pipeline form")
			},
		},
		"FF_USE_LEGACY_BASH_EVAL uses the bare eval pipeline": {
			lines: []string{"echo a"},
			opts:  []func(*Builder){withUseLegacyBashEval},
			assert: func(t *testing.T, s string) {
				assert.Regexp(t, `:\s*\|\s*eval `, s)
				assert.NotContains(t, s, "< /dev/null")
			},
		},
		"echoes commands": {
			lines: []string{"echo hello"},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "$ echo hello")
			},
		},
		"multiline collapsed": {
			lines: []string{"echo first\necho second"},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "$ echo first # collapsed multi-line command")
			},
		},
		"multiline with script sections": {
			lines: []string{"echo first\necho second"},
			opts:  []func(*Builder){withScriptSections},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "section_start:")
				assert.Contains(t, s, "test_step_0")
				assert.Contains(t, s, "hide_duration=true,collapsed=true")
				assert.Contains(t, s, "section_end:")
				assert.NotContains(t, s, "# collapsed multi-line command")
			},
		},
		"single line skips sections": {
			lines: []string{"echo hello"},
			opts:  []func(*Builder){withScriptSections},
			assert: func(t *testing.T, s string) {
				assert.NotContains(t, s, "section_start:")
				assert.Contains(t, s, "$ echo hello")
			},
		},
		"section index increments": {
			lines: []string{"echo a\necho b", "echo c\necho d"},
			opts:  []func(*Builder){withScriptSections},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "test_step_0")
				assert.Contains(t, s, "test_step_1")
			},
		},
		"sections with exit code check": {
			lines: []string{"echo a\necho b"},
			opts:  []func(*Builder){withScriptSections, withExitCodeCheck},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "section_start:")
				assert.Contains(t, s, "then exit")
			},
		},
		"empty lines with sections enabled": {
			lines: []string{"", "echo a\necho b"},
			opts:  []func(*Builder){withScriptSections},
			assert: func(t *testing.T, s string) {
				assert.Equal(t, 1, strings.Count(s, "section_start:"))
				assert.Contains(t, s, "test_step_1")
			},
		},
		// Regression guard for gitlab-runner#39610: the bash generator must NOT
		// emit a standalone "(exit $_runner_exit_code)" restore before user
		// commands. Such a restore aborts under "set -o errexit" at the top of
		// an else branch, where $? holds the failed if-condition's status.
		//
		// The body is shell-escaped, so `$` reaches the script as `\$` and a
		// literal "(exit $_runner_exit_code)" never appears. Match on "(exit "
		// so the guard actually fails if the restore is reintroduced.
		"does not emit errexit-tripping exit-code restore": {
			lines: []string{"echo hello"},
			assert: func(t *testing.T, s string) {
				assert.NotContains(t, s, "(exit ")
			},
		},
		"does not emit errexit-tripping exit-code restore with sections": {
			lines: []string{"echo first\necho second"},
			opts:  []func(*Builder){withScriptSections},
			assert: func(t *testing.T, s string) {
				assert.NotContains(t, s, "(exit ")
			},
		},
		// The post-command check is a different mechanism and must survive: it
		// reads $? after the user command rather than re-raising it before.
		"exit code check still captures the status after the command": {
			lines: []string{"echo hello"},
			opts:  []func(*Builder){withExitCodeCheck},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, `_runner_exit_code=\$?; if [ \$_runner_exit_code -ne 0 ]; then exit \$_runner_exit_code; fi`)
			},
		},
		"empty lines": {
			lines: []string{"", "echo hello"},
			assert: func(t *testing.T, s string) {
				assert.NotContains(t, s, "$ \n")
				assert.Contains(t, s, "$ echo hello")
			},
		},
		"all empty lines": {
			lines: []string{"", "", ""},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "eval")
				assert.Contains(t, s, "exit 0")
				assert.NotContains(t, s, "$ \n")
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			b := newBuilder(ShellBash, tc.opts...)
			tc.assert(t, b.Build(tc.lines))
		})
	}
}

// runBash writes script to a temp file, executes it with bash, and returns the
// combined output and the process exit code.
func runBash(t *testing.T, script string) (string, int) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "script.sh")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o700))

	// resolveBash, not a hardcoded "bash": requireShell gates on resolveBash,
	// which falls back to sh, so a host without bash in PATH would admit the
	// test and then fail to exec.
	shell, err := resolveBash()
	require.NoError(t, err)

	out, err := exec.Command(shell, path).CombinedOutput()
	if err == nil {
		return string(out), 0
	}

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("executing generated script: %v", err)
	return "", -1
}

// TestBashScript_Execute runs generated scripts through a real shell.
//
// Regression coverage for
// https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/39610.
//
// When a conditional was authored as separate script lines, the generator used
// to emit a per-line "(exit $_runner_exit_code)" status-restore. It captured the
// failed condition's exit status and re-raised it as a standalone "(exit 1)",
// which under "set -o errexit" aborted the whole script before the branch body
// ran. The restore is gone; these cases keep it gone.
//
// This was the concrete-only path: the legacy shells/bash.go generator never
// emitted such a restore, so the same scripts succeeded there (see the
// FF_CONCRETE off-vs-on integration test in executors/shell).
//
// The negative cases guard the other direction: removing the restore must not
// weaken failure propagation.
func TestBashScript_Execute(t *testing.T) {
	requireShell(t, ShellBash)

	// elseLines and negationLines are the two shapes that tripped the removed
	// restore. Both are re-run with ExitCodeCheck below, where they still fail.
	elseLines := []string{
		// TAKE_THEN is unset, so the else branch is taken.
		`if [ "$TAKE_THEN" = "true" ]; then`,
		`echo THEN_RAN`,
		`else`,
		`echo ELSE_RAN`,
		`fi`,
		`echo AFTER_FI`,
	}
	negationLines := []string{
		`! true`,
		`echo AFTER_NEGATION`,
	}

	tests := map[string]struct {
		lines    []string
		opts     []func(*Builder)
		wantCode int
		contains []string
		excludes []string
		skip     string
	}{
		"else branch runs to completion": {
			lines:    elseLines,
			contains: []string{"ELSE_RAN", "AFTER_FI"},
			excludes: []string{"THEN_RAN"},
		},
		// The post-command exit-code check reads $? at the top of the else branch
		// for exactly the same reason the removed restore did, so it re-raises the
		// failed if-condition's status and aborts before the else body runs. That
		// is a pre-existing defect shared with the legacy generators
		// (shells/bash.go CheckForErrors, driven per script line by
		// shells/abstract.go writeCommands, and functions/script_legacy), not
		// something this change introduced, and FF_ENABLE_BASH_EXIT_CODE_CHECK is
		// off by default. Recorded here so the gap is visible rather than merely
		// uncovered; un-skip these when gitlab-runner#39634 is fixed.
		"else branch runs to completion with exit code check": {
			lines:    elseLines,
			opts:     []func(*Builder){withExitCodeCheck},
			contains: []string{"ELSE_RAN", "AFTER_FI"},
			excludes: []string{"THEN_RAN"},
			skip:     "known gap gitlab-runner#39634: FF_ENABLE_BASH_EXIT_CODE_CHECK re-raises the failed if-condition's status at the top of the else branch",
		},
		"elif branch runs to completion": {
			lines: []string{
				`if [ a = b ]; then`,
				`echo FIRST_RAN`,
				`elif [ c = c ]; then`,
				`echo SECOND_RAN`,
				`fi`,
				`echo AFTER_FI`,
			},
			contains: []string{"SECOND_RAN", "AFTER_FI"},
			excludes: []string{"FIRST_RAN"},
		},
		// A trailing "! cmd" leaves $? non-zero for the same reason an else
		// branch does, and aborted identically before the fix.
		"negated command does not abort the script": {
			lines:    negationLines,
			contains: []string{"AFTER_NEGATION"},
		},
		"negated command does not abort the script with exit code check": {
			lines:    negationLines,
			opts:     []func(*Builder){withExitCodeCheck},
			contains: []string{"AFTER_NEGATION"},
			skip:     "known gap gitlab-runner#39634: FF_ENABLE_BASH_EXIT_CODE_CHECK re-raises the negated command's non-zero status",
		},
		// errexit must still propagate a plain failing command...
		"failing command still aborts the script": {
			lines:    []string{`false`, `echo NEVER_RAN`},
			wantCode: 1,
			excludes: []string{"NEVER_RAN"},
		},
		// ...and an explicit exit must keep its own status rather than becoming 0.
		"explicit exit preserves its status": {
			lines:    []string{`exit 7`, `echo NEVER_RAN`},
			wantCode: 7,
			excludes: []string{"NEVER_RAN"},
		},
		"failing command with exit code check still aborts the script": {
			lines:    []string{`false`, `echo NEVER_RAN`},
			opts:     []func(*Builder){withExitCodeCheck},
			wantCode: 1,
			excludes: []string{"NEVER_RAN"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.skip != "" {
				t.Skip(tc.skip)
			}

			script := newBuilder(ShellBash, tc.opts...).Build(tc.lines)
			out, code := runBash(t, script)

			assert.Equalf(t, tc.wantCode, code,
				"unexpected exit code\n--- script ---\n%s\n--- output ---\n%s", script, out)
			for _, want := range tc.contains {
				assert.Contains(t, out, want, "script:\n%s", script)
			}
			// Safe as a plain substring check: the echoed "$ <cmd>" header for
			// each excluded command sits inside the branch that is not taken,
			// so it is not printed either.
			for _, notWant := range tc.excludes {
				assert.NotContains(t, out, notWant, "script:\n%s", script)
			}
		})
	}
}

func TestPwshScript(t *testing.T) {
	tests := map[string]struct {
		shell  string
		lines  []string
		opts   []func(*Builder)
		assert func(t *testing.T, script string)
	}{
		"structure": {
			shell: ShellPwsh,
			lines: []string{"echo hello"},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, `$ErrorActionPreference = "Stop"`)
				assert.Contains(t, s, "& {")
			},
		},
		"error check": {
			shell: ShellPwsh,
			lines: []string{"echo a"},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "if(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }")
			},
		},
		"debug trace enabled": {
			shell: ShellPwsh,
			lines: []string{"echo a"},
			opts:  []func(*Builder){withDebugTrace},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "Set-PSDebug -Trace 2")
			},
		},
		"debug trace disabled": {
			shell: ShellPwsh,
			lines: []string{"echo a"},
			assert: func(t *testing.T, s string) {
				assert.NotContains(t, s, "Set-PSDebug -Trace 2")
			},
		},
		"echoes commands": {
			shell: ShellPwsh,
			lines: []string{"echo hello"},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "$ echo hello")
			},
		},
		"multiline collapsed": {
			shell: ShellPwsh,
			lines: []string{"echo first\necho second"},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "collapsed multi-line command")
				assert.Contains(t, s, "echo first")
			},
		},
		"multiline with script sections": {
			shell: ShellPwsh,
			lines: []string{"echo first\necho second"},
			opts:  []func(*Builder){withScriptSections},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "section_start:")
				assert.Contains(t, s, "test_step_0")
				assert.Contains(t, s, "section_end:")
				assert.NotContains(t, s, "collapsed multi-line command")
			},
		},
		"preserves exit code": {
			shell: ShellPwsh,
			lines: []string{"echo hello"},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "$_runner_exit_code = $LASTEXITCODE")
				assert.Contains(t, s, "$global:LASTEXITCODE = $_runner_exit_code")
			},
		},
		"shebang non-windows": {
			shell: ShellPwsh,
			lines: []string{"echo hello"},
			assert: func(t *testing.T, s string) {
				if runtime.GOOS == "windows" {
					assert.False(t, strings.HasPrefix(s, "#!"))
				} else {
					assert.True(t, strings.HasPrefix(s, "#!"))
					assert.Contains(t, s, ShellPwsh)
				}
			},
		},
		"line breaks non-windows": {
			shell: ShellPwsh,
			lines: []string{"echo hello"},
			assert: func(t *testing.T, s string) {
				if runtime.GOOS != "windows" {
					for line := range strings.SplitSeq(s, "\n") {
						assert.False(t, strings.HasSuffix(line, "\r"), "unexpected \\r in line: %q", line)
					}
				}
			},
		},
		"line breaks windows": {
			shell: ShellPowershell,
			lines: []string{"echo hello"},
			assert: func(t *testing.T, s string) {
				if runtime.GOOS == "windows" {
					assert.Contains(t, s, "\r\n")
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			requireShell(t, tc.shell)
			b := newBuilder(tc.shell, tc.opts...)
			tc.assert(t, b.Build(tc.lines))
		})
	}
}

func TestBuild_Dispatch(t *testing.T) {
	tests := map[string]struct {
		shell    string
		wantEval bool
		wantPwsh bool
	}{
		ShellBash:       {shell: ShellBash, wantEval: true},
		ShellSh:         {shell: ShellSh, wantEval: true},
		ShellPwsh:       {shell: ShellPwsh, wantPwsh: true},
		ShellPowershell: {shell: ShellPowershell, wantPwsh: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			requireShell(t, tc.shell)
			s := New("test", tc.shell).Build([]string{"echo hello"})
			assert.Equal(t, tc.wantEval, strings.Contains(s, "eval"))
			assert.Equal(t, tc.wantPwsh, strings.Contains(s, "$ErrorActionPreference"))
		})
	}
}

func TestResolveBash(t *testing.T) {
	requireShell(t, ShellBash)

	p, err := resolveBash()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(p, "/") || strings.Contains(p, ":\\"),
		"expected absolute path, got %s", p)
}

func TestShellPath(t *testing.T) {
	for _, shell := range []string{ShellBash, ShellSh} {
		t.Run(shell, func(t *testing.T) {
			requireShell(t, shell)
			p, err := shellPath(shell)
			require.NoError(t, err)
			assert.NotEmpty(t, p)
		})
	}
}

func TestShellEscape(t *testing.T) {
	tests := map[string]struct {
		input, expected string
	}{
		"empty":               {"", "''"},
		"safe":                {"hello", "hello"},
		"spaces":              {"hello world", `"hello world"`},
		"dollar":              {"$HOME", `"\$HOME"`},
		"backtick":            {"foo`bar", `"foo\` + "`" + `bar"`},
		"double quote":        {`say "hi"`, `"say \"hi\""`},
		"backslash":           {`path\to`, `"path\\to"`},
		"special chars":       {"a & b | c", `"a & b | c"`},
		"parentheses":         {"x&(y)", `"x&(y)"`},
		"slashes":             {"foo/bar/baz", "foo/bar/baz"},
		"dots":                {"file.txt", "file.txt"},
		"hyphens underscores": {"my-file_name", "my-file_name"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, shellEscape(tc.input))
		})
	}
}

func TestPsQuoteVariable(t *testing.T) {
	tests := map[string]struct {
		input, expected string
	}{
		"plain":        {"hello", `"hello"`},
		"dollar":       {"$foo", "\"`$foo\""},
		"backtick":     {"foo`bar", "\"foo``bar\""},
		"double quote": {`say "hello"`, "\"say `\"hello`\"\""},
		"single quote": {"it's", "\"it`'s\""},
		"newline":      {"line1\nline2", "\"line1`nline2\""},
		"tab":          {"col1\tcol2", "\"col1`tcol2\""},
		"smart quotes": {"\u201cleft\u201d \u201elow\u201c", "\"`\u201cleft`\u201d `\u201elow`\u201c\""},
		"hash":         {"# comment", "\"`# comment\""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, psQuoteVariable(tc.input))
		})
	}
}

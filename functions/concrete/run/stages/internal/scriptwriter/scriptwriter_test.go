//go:build !integration

package scriptwriter

import (
	"os/exec"
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

func withExitCodeCheck(b *Builder)  { b.ExitCodeCheck = true }
func withDebugTrace(b *Builder)     { b.DebugTrace = true }
func withScriptSections(b *Builder) { b.ScriptSections = true }

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
				for _, want := range []string{"set -o errexit", "set +o noclobber", "trap exit 1 TERM", "eval", "exit 0"} {
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
		"preserves exit code": {
			lines: []string{"echo hello"},
			assert: func(t *testing.T, s string) {
				assert.Contains(t, s, "_runner_exit_code=")
				assert.Contains(t, s, "(exit ")
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
					for _, line := range strings.Split(s, "\n") {
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

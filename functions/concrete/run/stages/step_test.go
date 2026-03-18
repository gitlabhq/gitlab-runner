//go:build !integration

package stages

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
)

func stdout(e *env.Env) string {
	return e.Stdout.(*bytes.Buffer).String()
}

func testShells(t *testing.T) []string {
	t.Helper()

	candidates := []string{"bash", "pwsh", "powershell"}

	var shells []string
	for _, s := range candidates {
		if _, err := exec.LookPath(s); err == nil {
			shells = append(shells, s)
		}
	}

	if len(shells) == 0 {
		t.Skip("no supported shell found on PATH")
	}

	return shells
}

func TestStep_ScriptOutput(t *testing.T) {
	tests := map[string]struct {
		lines          []string
		envVars        map[string]string
		gitLabEnv      map[string]string
		shellCmd       map[string][]string // per-shell overrides for lines
		contains       []string
		notContains    []string
		scriptSections bool
	}{
		"executes multiple lines": {
			lines:    []string{"echo hello", "echo world"},
			contains: []string{"hello", "world"},
		},
		"echoes commands with dollar prefix": {
			lines:    []string{"echo actual_output"},
			contains: []string{"$ echo actual_output", "actual_output"},
		},
		"collapses multi-line commands": {
			lines:    []string{"echo first\necho second"},
			contains: []string{"collapsed multi-line command", "first", "second"},
		},
		"multiline with script sections": {
			lines:          []string{"echo first\necho second"},
			scriptSections: true,
			contains:       []string{"section_start:", "section_end:", "first", "second"},
			notContains:    []string{"collapsed multi-line command", "$(date +%s)", "$([DateTimeOffset]"},
		},
		"passes environment variables": {
			envVars: map[string]string{"MY_VAR": "hello_from_env"},
			shellCmd: map[string][]string{
				"bash":       {"echo $MY_VAR"},
				"pwsh":       {"echo $env:MY_VAR"},
				"powershell": {"echo $env:MY_VAR"},
			},
			contains: []string{"hello_from_env"},
		},
		"GitLabEnv overrides base env": {
			envVars:   map[string]string{"MY_VAR": "base_value"},
			gitLabEnv: map[string]string{"MY_VAR": "gitlab_value"},
			shellCmd: map[string][]string{
				"bash":       {"echo $MY_VAR"},
				"pwsh":       {"echo $env:MY_VAR"},
				"powershell": {"echo $env:MY_VAR"},
			},
			contains:    []string{"gitlab_value"},
			notContains: []string{"base_value"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			for _, shell := range testShells(t) {
				t.Run(shell, func(t *testing.T) {
					e := newTestEnv(t, shell)

					for k, v := range tc.envVars {
						e.Env[k] = v
					}
					for k, v := range tc.gitLabEnv {
						e.GitLabEnv[k] = v
					}

					lines := tc.lines
					if perShell, ok := tc.shellCmd[shell]; ok {
						lines = perShell
					}

					step := Step{
						Step:           "test",
						Script:         lines,
						OnSuccess:      true,
						OnFailure:      true,
						ScriptSections: tc.scriptSections,
					}
					err := step.Run(t.Context(), e)
					require.NoError(t, err)

					output := stdout(e)
					for _, s := range tc.contains {
						assert.Contains(t, output, s)
					}
					for _, s := range tc.notContains {
						assert.NotContains(t, output, s)
					}
				})
			}
		})
	}
}

func TestStep_ErrorBehavior(t *testing.T) {
	tests := map[string]struct {
		lines             []string
		allowFailure      bool
		bashExitCodeCheck bool
		skipPwsh          bool
		expectError       bool
		contains          []string
		notContains       []string
	}{
		"stops on error": {
			lines:       []string{"exit 1", "echo should_not_appear"},
			expectError: true,
			notContains: []string{"should_not_appear"},
		},
		"allow failure suppresses error": {
			lines:        []string{"exit 1"},
			allowFailure: true,
			expectError:  false,
		},
		"empty script is a no-op": {
			lines:       []string{},
			expectError: false,
		},
		"preserves exit code with BashExitCodeCheck": {
			lines:             []string{"(exit 42) || true", "echo exit_was_$?"},
			bashExitCodeCheck: true,
			skipPwsh:          true,
			expectError:       false,
			contains:          []string{"exit_was_0"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			for _, shell := range testShells(t) {
				if tc.skipPwsh && (shell == "pwsh" || shell == "powershell") {
					continue
				}

				t.Run(shell, func(t *testing.T) {
					e := newTestEnv(t, shell)

					step := Step{
						Step:              "test",
						Script:            tc.lines,
						AllowFailure:      tc.allowFailure,
						BashExitCodeCheck: tc.bashExitCodeCheck,
						OnSuccess:         true,
						OnFailure:         true,
					}

					err := step.Run(t.Context(), e)

					if tc.expectError {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
					}

					output := stdout(e)
					for _, s := range tc.contains {
						assert.Contains(t, output, s)
					}
					for _, s := range tc.notContains {
						assert.NotContains(t, output, s)
					}
				})
			}
		})
	}
}

func TestStep_ConditionalExecution(t *testing.T) {
	tests := map[string]struct {
		onSuccess  bool
		onFailure  bool
		jobSuccess bool
		expectRun  bool
	}{
		"OnSuccess only, job succeeded": {
			onSuccess:  true,
			onFailure:  false,
			jobSuccess: true,
			expectRun:  true,
		},
		"OnSuccess only, job failed": {
			onSuccess:  true,
			onFailure:  false,
			jobSuccess: false,
			expectRun:  false,
		},
		"OnFailure only, job failed": {
			onSuccess:  false,
			onFailure:  true,
			jobSuccess: false,
			expectRun:  true,
		},
		"OnFailure only, job succeeded": {
			onSuccess:  false,
			onFailure:  true,
			jobSuccess: true,
			expectRun:  false,
		},
		"always (both), job succeeded": {
			onSuccess:  true,
			onFailure:  true,
			jobSuccess: true,
			expectRun:  true,
		},
		"always (both), job failed": {
			onSuccess:  true,
			onFailure:  true,
			jobSuccess: false,
			expectRun:  true,
		},
		"never (neither), job succeeded": {
			onSuccess:  false,
			onFailure:  false,
			jobSuccess: true,
			expectRun:  false,
		},
		"never (neither), job failed": {
			onSuccess:  false,
			onFailure:  false,
			jobSuccess: false,
			expectRun:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			for _, shell := range testShells(t) {
				t.Run(shell, func(t *testing.T) {
					e := newTestEnv(t, shell)

					if tc.jobSuccess {
						e.SetStatus(env.Success)
					} else {
						e.SetStatus(env.Failed)
					}
					step := Step{
						Step:      "test",
						Script:    []string{"echo step_executed"},
						OnSuccess: tc.onSuccess,
						OnFailure: tc.onFailure,
					}

					err := step.Run(t.Context(), e)
					require.NoError(t, err)

					output := stdout(e)
					if tc.expectRun {
						assert.Contains(t, output, "step_executed")
					} else {
						assert.NotContains(t, output, "step_executed")
					}
				})
			}
		})
	}
}

func TestStep_FailureThenCleanup(t *testing.T) {
	for _, shell := range testShells(t) {
		t.Run(shell, func(t *testing.T) {
			e := newTestEnv(t, shell)

			// First step fails
			build := Step{
				Step:      "build",
				Script:    []string{"echo build_ran", "exit 1"},
				OnSuccess: true,
			}
			err := build.Run(t.Context(), e)
			require.Error(t, err)
			assert.Contains(t, stdout(e), "build_ran")

			e.SetStatus(env.Failed)

			// Cleanup step runs on failure
			cleanup := Step{
				Step:      "cleanup",
				Script:    []string{"echo cleanup_ran"},
				OnSuccess: false,
				OnFailure: true,
			}
			err = cleanup.Run(t.Context(), e)
			require.NoError(t, err)
			assert.Contains(t, stdout(e), "cleanup_ran")
		})
	}
}

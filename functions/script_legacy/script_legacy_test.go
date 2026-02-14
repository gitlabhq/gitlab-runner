//go:build !integration

package script_legacy_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	script_legacy "gitlab.com/gitlab-org/gitlab-runner/functions/script_legacy"
	"gitlab.com/gitlab-org/gitlab-runner/functions/script_legacy/internal"
	"gitlab.com/gitlab-org/step-runner/pkg/testutil"
	"gitlab.com/gitlab-org/step-runner/proto"
)

func TestScriptLegacyIntegration(t *testing.T) {
	if runtime.GOOS == internal.WindowsOS {
		t.Skip("script_legacy is not supported on Windows")
	}

	t.Run("basic script execution with array syntax", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: hello_script_legacy
    step: builtin://script_legacy
    inputs:
      script:
        - echo "Hello from script_legacy"
        - echo "Second command"
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, `Running step name=hello_script_legacy`)
		require.Contains(t, logs, "Hello from script_legacy")
		require.Contains(t, logs, "Second command")
	})

	t.Run("environment variables are accessible", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_env
    step: builtin://script_legacy
    inputs:
      script:
        - 'echo "Custom: $CUSTOM_VAR"'
        - 'echo "Project: $PROJECT_NAME"'
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			WithEnvKeyVal("CUSTOM_VAR", "test_value").
			WithEnvKeyVal("PROJECT_NAME", "my-project").
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "Custom: test_value")
		require.Contains(t, logs, "Project: my-project")
	})

	t.Run("shell state persists across commands", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_persistence
    step: builtin://script_legacy
    inputs:
      script:
        - export MY_VAR=hello
        - 'echo "Value: $MY_VAR"'
        - cd /tmp
        - pwd
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "Value: hello")
		// Check for /tmp (may be /private/tmp on macOS)
		assert.True(t, strings.Contains(logs, "/tmp"), "logs should contain /tmp path")
	})

	t.Run("debug_trace defaults to false", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_no_trace
    step: builtin://script_legacy
    inputs:
      script:
        - echo "test command"
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "test command")
		// Should NOT have xtrace output (no '+' prefix from bash)
		require.NotContains(t, logs, "+ echo")
	})

	t.Run("debug_trace can be enabled", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_with_trace
    step: builtin://script_legacy
    inputs:
      script:
        - echo "traced command"
      debug_trace: true
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "traced command")
		// Should have xtrace output ('+' prefix from bash)
		require.Contains(t, logs, "+ echo")
	})

	t.Run("error in script fails the step", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_error
    step: builtin://script_legacy
    inputs:
      script:
        - echo "before error"
        - exit 1
        - echo "after error"
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.Error(t, err)
		assert.Equal(t, proto.StepResult_failure, res.Status)
		require.Contains(t, logs, "before error")
		// Should NOT reach "after error" because errexit is enabled
		require.NotContains(t, logs, "after error")
	})

	t.Run("empty script array fails", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_empty
    step: builtin://script_legacy
    inputs:
      script: []
`
		res, _, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.Error(t, err)
		assert.Equal(t, proto.StepResult_failure, res.Status)
		require.Contains(t, err.Error(), "empty")
	})

	t.Run("multi-line commands work correctly", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_multiline
    step: builtin://script_legacy
    inputs:
      script:
        - |
          if [ -n "$USER" ]; then
            echo "User is set"
          fi
        - echo "done"
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			WithEnvKeyVal("USER", "testuser").
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "User is set")
		require.Contains(t, logs, "done")
	})

	t.Run("can use expressions in inputs", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_expressions
    step: builtin://script_legacy
    inputs:
      script:
        - echo "${{ env.BASE_MSG }} from expressions"
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			WithEnvKeyVal("BASE_MSG", "Hello").
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "Hello from expressions")
	})

	t.Run("works with other steps in sequence", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: step1
    step: builtin://script_legacy
    inputs:
      script:
        - echo "First step"
  - name: step2
    step: builtin://script_legacy
    inputs:
      script:
        - echo "Second step"
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, `Running step name=step1`)
		require.Contains(t, logs, "First step")
		require.Contains(t, logs, `Running step name=step2`)
		require.Contains(t, logs, "Second step")
		assert.Len(t, res.SubStepResults, 2)
	})

	t.Run("special characters in env vars are preserved", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_special_chars
    step: builtin://script_legacy
    inputs:
      script:
        - 'echo "Quotes: $VAR_WITH_QUOTES"'
        - 'echo "Spaces: $VAR_WITH_SPACES"'
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			WithEnvKeyVal("VAR_WITH_QUOTES", `value with "quotes"`).
			WithEnvKeyVal("VAR_WITH_SPACES", "hello world").
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, `value with "quotes"`)
		require.Contains(t, logs, "hello world")
	})

	t.Run("check_for_errors defaults to false", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_no_check
    step: builtin://script_legacy
    inputs:
      script:
        - echo "before"
        - true
        - echo "after"
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "before")
		require.Contains(t, logs, "after")
		// Should not contain explicit exit code check pattern
		require.NotContains(t, logs, "_runner_exit_code")
	})

	t.Run("check_for_errors enabled catches failures", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_with_check
    step: builtin://script_legacy
    inputs:
      script:
        - echo "before false"
        - /bin/false
        - echo "after false - should not appear"
      check_for_errors: true
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.Error(t, err)
		assert.Equal(t, proto.StepResult_failure, res.Status)
		require.Contains(t, logs, "before false")
		require.NotContains(t, logs, "after false - should not appear")
	})

	t.Run("check_for_errors with successful commands", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_check_success
    step: builtin://script_legacy
    inputs:
      script:
        - echo "command 1"
        - echo "command 2"
        - echo "command 3"
      check_for_errors: true
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "command 1")
		require.Contains(t, logs, "command 2")
		require.Contains(t, logs, "command 3")
	})

	t.Run("posix_escape defaults to false (ANSI-C mode)", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_ansi_mode
    step: builtin://script_legacy
    inputs:
      script:
        - echo "test command"
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		// ANSI-C mode includes color codes (though we can't easily assert on the exact codes)
		require.Contains(t, logs, "test command")
	})

	t.Run("posix_escape enabled uses POSIX mode", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_posix_mode
    step: builtin://script_legacy
    inputs:
      script:
        - echo "test command"
      posix_escape: true
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "test command")
		// In POSIX mode, no ANSI color codes should be present
		// We verify the command still executes correctly
	})

	t.Run("posix_escape with special characters", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_posix_special
    step: builtin://script_legacy
    inputs:
      script:
        - 'echo "quote: test"'
        - 'echo "dollar: $HOME"'
      posix_escape: true
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "quote:")
		require.Contains(t, logs, "dollar:")
	})

	t.Run("trace_sections defaults to false", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_no_sections
    step: builtin://script_legacy
    inputs:
      script:
        - |
          echo "line 1"
          echo "line 2"
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "line 1")
		require.Contains(t, logs, "line 2")
		// Should show "collapsed multi-line command" indicator
		require.Contains(t, logs, "collapsed multi-line command")
		// Should NOT contain trace section markers
		require.NotContains(t, logs, "section_start:")
		require.NotContains(t, logs, "section_end:")
	})

	t.Run("trace_sections enabled creates section markers", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_with_sections
    step: builtin://script_legacy
    inputs:
      script:
        - |
          echo "multi-line"
          echo "command"
      trace_sections: true
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "multi-line")
		require.Contains(t, logs, "command")
		// Should contain GitLab trace section markers
		require.Contains(t, logs, "section_start:")
		require.Contains(t, logs, "section_end:")
		require.Contains(t, logs, "section_script_step_0")
		// Should NOT show "collapsed multi-line command" when sections enabled
		require.NotContains(t, logs, "collapsed multi-line command")
	})

	t.Run("trace_sections only affects multi-line commands", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_sections_selective
    step: builtin://script_legacy
    inputs:
      script:
        - echo "single line"
        - |
          echo "multi"
          echo "line"
        - echo "another single"
      trace_sections: true
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "single line")
		require.Contains(t, logs, "multi")
		require.Contains(t, logs, "line")
		require.Contains(t, logs, "another single")
		// Section markers should only appear for the multi-line command (index 1)
		require.Contains(t, logs, "section_script_step_1")
		// Should NOT have sections for single-line commands (indexes 0 and 2)
		require.NotContains(t, logs, "section_script_step_0")
		require.NotContains(t, logs, "section_script_step_2")
	})

	t.Run("combined flags: debug_trace and check_for_errors", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_combined_1
    step: builtin://script_legacy
    inputs:
      script:
        - echo "test1"
        - echo "test2"
      debug_trace: true
      check_for_errors: true
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		// debug_trace should show xtrace output
		require.Contains(t, logs, "+ echo")
		// Both commands should execute
		require.Contains(t, logs, "test1")
		require.Contains(t, logs, "test2")
	})

	t.Run("combined flags: posix_escape and trace_sections", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_combined_2
    step: builtin://script_legacy
    inputs:
      script:
        - |
          echo "line1"
          echo "line2"
      posix_escape: true
      trace_sections: true
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "line1")
		require.Contains(t, logs, "line2")
		// trace_sections should create section markers
		require.Contains(t, logs, "section_start:")
		require.Contains(t, logs, "section_end:")
	})

	t.Run("combined flags: debug_trace and posix_escape", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_combined_3
    step: builtin://script_legacy
    inputs:
      script:
        - echo "test output"
      debug_trace: true
      posix_escape: true
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "test output")
		// debug_trace should show xtrace output
		require.Contains(t, logs, "+ echo")
		// posix_escape should use simple format (no ANSI color codes in command log)
		// The $ prompt should be visible
		require.Contains(t, logs, "$ echo")
	})

	t.Run("all flags enabled together", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_all_flags
    step: builtin://script_legacy
    inputs:
      script:
        - echo "single command"
        - |
          echo "multi1"
          echo "multi2"
      debug_trace: true
      check_for_errors: true
      posix_escape: true
      trace_sections: true
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.NoError(t, err)
		assert.Equal(t, proto.StepResult_success, res.Status)
		require.Contains(t, logs, "single command")
		require.Contains(t, logs, "multi1")
		require.Contains(t, logs, "multi2")
		// debug_trace
		require.Contains(t, logs, "+ echo")
		// trace_sections for multi-line command
		require.Contains(t, logs, "section_script_step_1")
	})

	t.Run("trace_sections with command failure", func(t *testing.T) {
		stepYml := `
spec:
---
run:
  - name: test_sections_failure
    step: builtin://script_legacy
    inputs:
      script:
        - |
          echo "before failure"
          exit 42
          echo "after failure - should not execute"
      trace_sections: true
`
		res, logs, err := testutil.StepRunner(t).
			RegisterStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run).
			Run(stepYml)
		require.Error(t, err)
		assert.Equal(t, proto.StepResult_failure, res.Status)
		require.Contains(t, logs, "before failure")
		// exit 42 should cause errexit to trigger
		require.NotContains(t, logs, "after failure - should not execute")
		// Section markers should still be present
		require.Contains(t, logs, "section_start:")
	})
}

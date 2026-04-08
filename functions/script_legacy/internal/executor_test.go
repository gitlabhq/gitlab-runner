//go:build !integration

package internal

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute_BasicScript(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	require.NoError(t, err, "DetectShell() failed")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	executor := NewExecutor(ExecutorConfig{
		Stdout:    stdout,
		Stderr:    stderr,
		Env:       []string{},
		WorkDir:   ".",
		ShellPath: shellPath,
	})

	script := `#!/usr/bin/env bash
echo "Hello World"
`

	err = executor.Execute(t.Context(), script)
	require.NoError(t, err, "Execute() failed")

	assert.Contains(t, stdout.String(), "Hello World", "Expected 'Hello World' in output")
}

func TestExecute_WithShCompatibility(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	require.NoError(t, err, "DetectShell() failed")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	executor := NewExecutor(ExecutorConfig{
		Stdout:    stdout,
		Stderr:    stderr,
		Env:       []string{},
		WorkDir:   ".",
		ShellPath: shellPath,
	})

	// Script with conditional pipefail (sh-compatible)
	script := `#!/usr/bin/env bash
if set -o | grep pipefail > /dev/null; then set -o pipefail; fi
set -o errexit
echo "test output"
`

	err = executor.Execute(t.Context(), script)
	require.NoError(t, err, "Execute() failed with sh-compatible script")

	assert.Contains(t, stdout.String(), "test output", "Expected 'test output' in output")
}

func TestExecute_ExitOnError(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	require.NoError(t, err, "DetectShell() failed")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	executor := NewExecutor(ExecutorConfig{
		Stdout:    stdout,
		Stderr:    stderr,
		Env:       []string{},
		WorkDir:   ".",
		ShellPath: shellPath,
	})

	script := `#!/usr/bin/env bash
set -o errexit
false
echo "should not reach here"
`

	err = executor.Execute(t.Context(), script)
	assert.Error(t, err, "Expected error from failed command")

	assert.NotContains(t, stdout.String(), "should not reach here", "Script continued after error when errexit was set")
}

func TestExecute_WithEnvironment(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	require.NoError(t, err, "DetectShell() failed")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := []string{"TEST_VAR=test_value"}
	executor := NewExecutor(ExecutorConfig{
		Stdout:    stdout,
		Stderr:    stderr,
		Env:       env,
		WorkDir:   ".",
		ShellPath: shellPath,
	})

	script := `#!/usr/bin/env bash
echo $TEST_VAR
`

	err = executor.Execute(t.Context(), script)
	require.NoError(t, err, "Execute() failed")

	assert.Contains(t, stdout.String(), "test_value", "Expected environment variable in output")
}

func TestExecute_CheckForErrorsCatchesFailure(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	require.NoError(t, err, "DetectShell() failed")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	executor := NewExecutor(ExecutorConfig{
		Stdout:    stdout,
		Stderr:    stderr,
		Env:       []string{},
		WorkDir:   ".",
		ShellPath: shellPath,
	})

	script := `#!/usr/bin/env bash
set -o errexit
echo "before false"
false
_runner_exit_code=$?; if [ $_runner_exit_code -ne 0 ]; then exit $_runner_exit_code; fi
echo "after false - should not reach here"
`

	err = executor.Execute(t.Context(), script)
	assert.Error(t, err, "Expected error from failed command with exit code check")

	assert.Contains(t, stdout.String(), "before false", "Should have reached first echo")
	assert.NotContains(t, stdout.String(), "after false", "Should not have reached echo after failed command")
}

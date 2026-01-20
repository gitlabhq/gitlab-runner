//go:build !integration

package internal

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

func TestExecute_BasicScript(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	if err != nil {
		t.Fatalf("DetectShell() failed: %v", err)
	}

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
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Hello World") {
		t.Errorf("Expected 'Hello World' in output, got: %s", output)
	}
}

func TestExecute_WithShCompatibility(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	if err != nil {
		t.Fatalf("DetectShell() failed: %v", err)
	}

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
	if err != nil {
		t.Fatalf("Execute() failed with sh-compatible script: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "test output") {
		t.Errorf("Expected 'test output' in output, got: %s", output)
	}
}

func TestExecute_ExitOnError(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	if err != nil {
		t.Fatalf("DetectShell() failed: %v", err)
	}

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
	if err == nil {
		t.Error("Expected error from failed command, got nil")
	}

	output := stdout.String()
	if strings.Contains(output, "should not reach here") {
		t.Error("Script continued after error when errexit was set")
	}
}

func TestExecute_WithEnvironment(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	if err != nil {
		t.Fatalf("DetectShell() failed: %v", err)
	}

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
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "test_value") {
		t.Errorf("Expected environment variable in output, got: %s", output)
	}
}

func TestExecute_CheckForErrorsCatchesFailure(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	if err != nil {
		t.Fatalf("DetectShell() failed: %v", err)
	}

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
	if err == nil {
		t.Error("Expected error from failed command with exit code check, got nil")
	}

	output := stdout.String()
	if !strings.Contains(output, "before false") {
		t.Error("Should have reached first echo")
	}

	if strings.Contains(output, "after false") {
		t.Error("Should not have reached echo after failed command")
	}
}

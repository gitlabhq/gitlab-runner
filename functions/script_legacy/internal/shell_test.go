//go:build !integration

package internal

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectShell_FindsShell(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	require.NoError(t, err, "DetectShell() failed")

	// Verify the shell is actually executable
	_, err = exec.LookPath(shellPath)
	assert.NoError(t, err, "Detected shell %s is not executable", shellPath)

	t.Logf("Detected shell: %s", shellPath)
}

func TestDetectShell_PrefersBashOverSh(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	require.NoError(t, err, "DetectShell() failed")

	// If both bash and sh are available, should prefer bash
	bashPath, bashErr := exec.LookPath("bash")
	shPath, shErr := exec.LookPath("sh")

	if bashErr == nil && shErr == nil {
		assert.True(t, strings.Contains(shellPath, "bash"),
			"Expected bash path when both bash and sh available, got %s", shellPath)
	}

	assert.NotEmpty(t, shellPath, "DetectShell() returned empty path")

	t.Logf("Detected shell at %s (bash: %s, sh: %s)", shellPath, bashPath, shPath)
}

func TestDetectShell_Windows(t *testing.T) {
	if runtime.GOOS != WindowsOS {
		t.Skip("This test only runs on Windows")
	}

	_, err := DetectShell()
	assert.Error(t, err, "Expected error on Windows")
	assert.Contains(t, err.Error(), "not supported on Windows", "Expected Windows error message")
}

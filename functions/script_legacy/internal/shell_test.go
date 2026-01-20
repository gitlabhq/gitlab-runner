//go:build !integration

package internal

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestDetectShell_FindsShell(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	if err != nil {
		t.Fatalf("DetectShell() failed: %v", err)
	}

	// Verify the shell is actually executable
	if _, err := exec.LookPath(shellPath); err != nil {
		t.Errorf("Detected shell %s is not executable: %v", shellPath, err)
	}

	t.Logf("Detected shell: %s", shellPath)
}

func TestDetectShell_PrefersBashOverSh(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("scriptv2 is not supported on Windows")
	}

	shellPath, err := DetectShell()
	if err != nil {
		t.Fatalf("DetectShell() failed: %v", err)
	}

	// If both bash and sh are available, should prefer bash
	bashPath, bashErr := exec.LookPath("bash")
	shPath, shErr := exec.LookPath("sh")

	if bashErr == nil && shErr == nil {
		// Both available - should choose bash
		if !strings.Contains(shellPath, "bash") {
			t.Errorf("Expected bash path when both bash and sh available, got %s", shellPath)
		}
	}

	// At minimum, should find something
	if shellPath == "" {
		t.Error("DetectShell() returned empty path")
	}

	t.Logf("Detected shell at %s (bash: %s, sh: %s)", shellPath, bashPath, shPath)
}

func TestDetectShell_Windows(t *testing.T) {
	if runtime.GOOS != WindowsOS {
		t.Skip("This test only runs on Windows")
	}

	_, err := DetectShell()
	if err == nil {
		t.Error("Expected error on Windows, got nil")
	}

	if !strings.Contains(err.Error(), "not supported on Windows") {
		t.Errorf("Expected Windows error message, got: %v", err)
	}
}

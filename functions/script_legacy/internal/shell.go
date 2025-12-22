package internal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

const WindowsOS = "windows"

// DetectShell finds bash or sh by trying multiple common paths.
// Returns the full path to the detected shell.
// This handles containers where bash might not be available (e.g., Alpine).
// Falls back to sh variants if bash not found.
func DetectShell() (shellPath string, err error) {
	if runtime.GOOS == WindowsOS {
		return "", fmt.Errorf("script steps are not supported on Windows")
	}

	candidates := []string{
		"bash",
		"/bin/bash",
		"/usr/bin/bash",
		"/usr/local/bin/bash",
		"sh",
		"/bin/sh",
		"/usr/bin/sh",
		"/usr/local/bin/sh",
		"/busybox/sh",
	}

	for _, shell := range candidates {
		path, lookupErr := exec.LookPath(shell)
		if lookupErr == nil && isExecutable(path) {
			return path, nil
		}
	}

	return "", fmt.Errorf("no shell found in any of: %v", candidates)
}

// isExecutable checks if a file exists, is a regular file, and is executable.
func isExecutable(file string) bool {
	info, err := os.Stat(file)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}

	if err != nil {
		return false
	}

	return info.Mode().IsRegular() && info.Mode()&0o111 != 0
}

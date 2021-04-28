package helpers

import (
	"os/exec"
	"testing"
)

func SkipIntegrationTests(t *testing.T, app ...string) {
	if testing.Short() {
		t.Skip("Skipping long tests")
	}

	if ok, err := ExecuteCommandSucceeded(app...); !ok {
		t.Skip(app[0], "failed", err)
	}
}

// ExecuteCommandSucceeded tests whether a particular command execution successfully
// completes. If it does not, it returns the error produced.
func ExecuteCommandSucceeded(app ...string) (bool, error) {
	if len(app) > 0 {
		cmd := exec.Command(app[0], app[1:]...)
		err := cmd.Run()
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

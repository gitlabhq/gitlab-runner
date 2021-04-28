package helpers

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"
)

func ShouldSkipIntegrationTests(app ...string) error {
	if testing.Short() {
		return errors.New("skipping long tests")
	}

	if ok, err := ExecuteCommandSucceeded(app...); !ok {
		return fmt.Errorf("%s failed: %v", app[0], err)
	}

	return nil
}

func SkipIntegrationTests(t *testing.T, app ...string) {
	err := ShouldSkipIntegrationTests(app...)
	if err != nil {
		t.Skip(err.Error())
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

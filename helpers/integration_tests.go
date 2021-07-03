package helpers

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func SkipIntegrationTests(t *testing.T, cmd ...string) {
	if testing.Short() {
		t.Skip("Skipping long tests")
	}

	if len(cmd) == 0 {
		return
	}

	executable, err := exec.LookPath(cmd[0])
	if err != nil {
		t.Skip(cmd[0], "doesn't exist", err)
	}

	if err := executeCommandSucceeded(executable, cmd[1:]); err != nil {
		assert.FailNow(t, "failed integration test command", "%s failed with error: %v", executable, err)
	}
}

// executeCommandSucceeded tests whether a particular command execution successfully
// completes. If it does not, it returns the error produced.
func executeCommandSucceeded(executable string, args []string) error {
	cmd := exec.Command(executable, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s - %w", string(out), err)
	}

	return nil
}

//go:build !integration

package docker

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func guardMachineOperationTest(t *testing.T, name string, callback func(t *testing.T)) {
	tempHomeDir := t.TempDir()

	machineDir := path.Join(tempHomeDir, ".docker", "machine")
	err := os.MkdirAll(machineDir, 0755)
	require.NoError(t, err)

	t.Setenv("MACHINE_STORAGE_PATH", machineDir)
	t.Run(name, callback)
}

func TestList(t *testing.T) {
	guardMachineOperationTest(t, "no machines", func(t *testing.T) {
		err := os.MkdirAll(getMachineDir(), 0755)
		require.NoError(t, err)

		mc := NewMachineCommand()
		hostNames, err := mc.List()
		assert.Empty(t, hostNames)
		assert.NoError(t, err)
	})

	guardMachineOperationTest(t, "one machine", func(t *testing.T) {
		err := os.MkdirAll(getMachineDir(), 0755)
		require.NoError(t, err)

		machineDir := path.Join(getMachineDir(), "machine-1")
		err = os.MkdirAll(machineDir, 0755)
		require.NoError(t, err)

		mc := NewMachineCommand()
		hostNames, err := mc.List()
		assert.Contains(t, hostNames, "machine-1")
		assert.Len(t, hostNames, 1)
		assert.NoError(t, err)
	})

	guardMachineOperationTest(t, "machines directory doesn't exist", func(t *testing.T) {
		mc := NewMachineCommand()
		hostNames, err := mc.List()
		assert.Empty(t, hostNames)
		assert.NoError(t, err)
	})

	guardMachineOperationTest(t, "machines directory is invalid", func(t *testing.T) {
		err := os.MkdirAll(getBaseDir(), 0755)
		require.NoError(t, err)

		err = os.WriteFile(getMachineDir(), []byte{}, 0o600)
		require.NoError(t, err)

		mc := NewMachineCommand()
		hostNames, err := mc.List()
		assert.Empty(t, hostNames)
		assert.Error(t, err)
	})
}

func mockDockerMachineExecutable(t *testing.T) func() {
	tempDir := t.TempDir()

	dmExecutable := filepath.Join(tempDir, "docker-machine")
	if runtime.GOOS == "windows" {
		dmExecutable += ".exe"
	}

	err := os.WriteFile(dmExecutable, []byte{}, 0o777)
	require.NoError(t, err)

	currentDockerMachineExecutable := dockerMachineExecutable
	dockerMachineExecutable = dmExecutable

	return func() {
		dockerMachineExecutable = currentDockerMachineExecutable
	}
}

var dockerMachineCommandArgs = []string{"version", "--help"}

func getDockerMachineCommandExpectedArgs(token string) []string {
	if token == "" {
		token = "no-report"
	}

	return []string{dockerMachineExecutable, fmt.Sprintf("--bugsnag-api-token=%s", token), "version", "--help"}
}

var dockerMachineCommandTests = map[string]struct {
	tokenEnvValue string
	expectedArgs  func() []string
}{
	"MACHINE_BUGSNAG_API_TOKEN is defined by the user": {
		tokenEnvValue: "some-other-token",
		expectedArgs:  func() []string { return getDockerMachineCommandExpectedArgs("some-other-token") },
	},
	"MACHINE_BUGSNAG_API_TOKEN is not defined by the user": {
		tokenEnvValue: "",
		expectedArgs:  func() []string { return getDockerMachineCommandExpectedArgs("") },
	},
}

func TestNewDockerMachineCommand(t *testing.T) {
	for tn, tc := range dockerMachineCommandTests {
		t.Run(tn, func(t *testing.T) {
			err := os.Setenv("MACHINE_BUGSNAG_API_TOKEN", tc.tokenEnvValue)
			require.NoError(t, err)

			ctx, ctxCancelFn := context.WithTimeout(t.Context(), 1*time.Hour)
			defer ctxCancelFn()

			cmd := newDockerMachineCommand(ctx, dockerMachineCommandArgs...)

			assert.Equal(t, tc.expectedArgs(), cmd.Args)
			assert.NotEmpty(t, cmd.Env)
		})
	}
}

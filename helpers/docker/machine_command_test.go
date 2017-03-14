package docker_helpers

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/docker/machine/commands/mcndirs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func guardMachineOperationTest(t *testing.T, name string, callback func(t *testing.T)) {
	tempHomeDir, err := ioutil.TempDir("", "docker-machine-list")
	require.NoError(t, err)

	machineDir := path.Join(tempHomeDir, ".docker", "machine")
	err = os.MkdirAll(machineDir, 0755)
	require.NoError(t, err)

	mcndirs.BaseDir = machineDir
	defer func() {
		mcndirs.BaseDir = ""
		os.RemoveAll(tempHomeDir)
	}()

	t.Run(name, callback)
}

func TestList(t *testing.T) {
	guardMachineOperationTest(t, "no machines", func(t *testing.T) {
		err := os.MkdirAll(mcndirs.GetMachineDir(), 0755)
		require.NoError(t, err)

		mc := NewMachineCommand()
		hostNames, err := mc.List()
		assert.Empty(t, hostNames)
		assert.NoError(t, err)
	})

	guardMachineOperationTest(t, "one machine", func(t *testing.T) {
		err := os.MkdirAll(mcndirs.GetMachineDir(), 0755)
		require.NoError(t, err)

		machineDir := path.Join(mcndirs.GetMachineDir(), "machine-1")
		err = os.MkdirAll(machineDir, 0755)
		require.NoError(t, err)

		mc := NewMachineCommand()
		hostNames, err := mc.List()
		assert.Contains(t, hostNames, "machine-1")
		assert.Len(t, hostNames, 1)
		assert.NoError(t, err)
	})

	guardMachineOperationTest(t, "machines directory doesn't exists", func(t *testing.T) {
		mc := NewMachineCommand()
		hostNames, err := mc.List()
		assert.Empty(t, hostNames)
		assert.NoError(t, err)
	})

	guardMachineOperationTest(t, "machines directory is invalid", func(t *testing.T) {
		err := os.MkdirAll(mcndirs.GetBaseDir(), 0755)
		require.NoError(t, err)

		err = ioutil.WriteFile(mcndirs.GetMachineDir(), []byte{}, 0600)
		require.NoError(t, err)

		mc := NewMachineCommand()
		hostNames, err := mc.List()
		assert.Empty(t, hostNames)
		assert.Error(t, err)
	})
}

package machine

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMachineNewName(t *testing.T) {
	a := newMachineName("machine-template-%s")
	b := newMachineName("machine-template-%s")
	assert.NotEqual(t, a, b)
}

func TestMachineFilter(t *testing.T) {
	filter := "machine-template-%s"
	machines := []string{
		"test-machine",
		"machine-template-10",
	}
	filtered := filterMachineList(machines, filter)

	assert.NotContains(t, filtered, machines[0])
	assert.Contains(t, filtered, machines[1])
}

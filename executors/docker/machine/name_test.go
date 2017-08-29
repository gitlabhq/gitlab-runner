package machine

import (
	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"testing"
)

func TestMachineNewName(t *testing.T) {
	config := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Machine: &common.DockerMachine{
				MachineName: "test-machine-%s",
			},
		},
	}
	a := newMachineName(config)
	b := newMachineName(config)
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

//go:build !integration

package machine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	dns_test "gitlab.com/gitlab-org/gitlab-runner/helpers/dns/test"
)

func TestNewMachineName(t *testing.T) {
	testCases := map[string]struct {
		token string
	}{
		"DNS-1123 compatible token": {
			token: "token-of",
		},
		"non DNS-1123 compatible token": {
			token: "ToK3_?OF",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			config := &common.RunnerConfig{
				RunnerCredentials: common.RunnerCredentials{
					Token: testCase.token,
				},
				RunnerSettings: common.RunnerSettings{
					Machine: &common.DockerMachine{
						MachineName: "test-machine-%s",
					},
				},
			}

			name := newMachineName(config)
			dns_test.AssertRFC1123Compatibility(t, name)
		})
	}
}

func TestNewMachineNameIsUnique(t *testing.T) {
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

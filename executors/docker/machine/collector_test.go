//go:build !integration

package machine

import (
	"testing"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestIfMachineProviderExposesCollectInterface(t *testing.T) {
	var provider common.ExecutorProvider = &machineProvider{}
	collector, ok := provider.(prometheus.Collector)
	assert.True(t, ok)
	assert.NotNil(t, collector)
}

func TestMachineProviderDeadInterval(t *testing.T) {
	provider := newMachineProvider("docker_machines", "docker")
	assert.Equal(t, 0, provider.collectDetails().Idle)

	details := provider.machineDetails("test", false)
	assert.Equal(t, 1, provider.collectDetails().Idle)

	details.LastSeen = time.Now().Add(-machineDeadInterval)
	assert.Equal(t, 0, provider.collectDetails().Idle)
}

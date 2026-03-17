//go:build !integration

package machine

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	docker_executor "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
)

func TestIfMachineProviderExposesCollectInterface(t *testing.T) {
	var provider common.ExecutorProvider = &machineProvider{}
	collector, ok := provider.(prometheus.Collector)
	assert.True(t, ok)
	assert.NotNil(t, collector)
}

func TestMachineProviderDeadInterval(t *testing.T) {
	provider := newMachineProvider(docker_executor.NewProvider())
	assert.Equal(t, 0, provider.collectDetails().Idle)

	details := provider.machineDetails("test", false)
	assert.Equal(t, 1, provider.collectDetails().Idle)

	details.LastSeen = time.Now().Add(-(machineDeadInterval * time.Second))
	assert.Equal(t, 0, provider.collectDetails().Idle)
}

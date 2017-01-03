package machine

import (
	"testing"
	"time"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestIfMachineProviderExposesCollectInterface(t *testing.T) {
	var provider common.ExecutorProvider
	provider = &machineProvider{}
	collector, ok := provider.(prometheus.Collector)
	assert.True(t, ok)
	assert.NotNil(t, collector)
}

func TestMachineProviderDescribeAndCollect(t *testing.T) {
	provider := &machineProvider{}

	descCh := make(chan *prometheus.Desc, 10)
	provider.Describe(descCh)
	assert.Len(t, descCh, 2)

	metricCh := make(chan prometheus.Metric, 50)
	provider.Collect(metricCh)
	assert.Len(t, metricCh, 8)
}

func TestMachineProviderDeadInterval(t *testing.T) {
	provider := &machineProvider{
		details: make(machinesDetails),
	}
	assert.Equal(t, 0, provider.collectDetails().Idle)

	details := provider.machineDetails("test", false)
	assert.Equal(t, 1, provider.collectDetails().Idle)

	details.LastSeen = time.Now().Add(-machineDeadInterval)
	assert.Equal(t, 0, provider.collectDetails().Idle)
}

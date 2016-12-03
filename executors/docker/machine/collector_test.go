package machine

import (
	"testing"

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

func TestMachineProviderDescribe(t *testing.T) {
	ch := make(chan *prometheus.Desc, 10)
	provider := &machineProvider{}
	provider.Describe(ch)
	assert.Len(t, ch, 2)
}

func TestMachineProviderCollect(t *testing.T) {
	ch := make(chan prometheus.Metric, 50)
	provider := &machineProvider{}
	provider.Collect(ch)
	assert.Len(t, ch, 8)
}

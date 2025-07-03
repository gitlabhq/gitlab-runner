//go:build !integration

package prometheus

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	prometheus_go "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestFailuresCollector_Collect_GroupingReasons(t *testing.T) {
	ch := make(chan prometheus.Metric, 50)

	rc := common.RunnerConfig{
		Name: "qwerty123",
		// RunnerSettings: common.RunnerSettings{},
		RunnerCredentials: common.RunnerCredentials{
			Token: "abcd1234",
		},
	}

	fc := NewFailuresCollector()
	fc.RecordFailure(common.ScriptFailure, rc)
	fc.RecordFailure(common.RunnerSystemFailure, rc)

	fc.Collect(ch)
	assert.Len(t, ch, 2)
}

func TestFailuresCollector_Collect_MetricsValues(t *testing.T) {
	ch := make(chan prometheus.Metric, 50)

	rc := common.RunnerConfig{
		Name: "qwerty123",
		// RunnerSettings: common.RunnerSettings{},
		RunnerCredentials: common.RunnerCredentials{
			Token: "a1b2c3d4",
		},
	}

	fc := NewFailuresCollector()
	fc.RecordFailure(common.ScriptFailure, rc)
	fc.RecordFailure(common.ScriptFailure, rc)

	fc.Collect(ch)

	metric := &prometheus_go.Metric{}
	m := <-ch
	_ = m.Write(metric)

	labels := make(map[string]string)
	for _, labelPair := range metric.Label {
		labels[*labelPair.Name] = *labelPair.Value
	}

	assert.Equal(t, float64(2), *metric.Counter.Value)
	assert.Equal(t, string(common.ScriptFailure), labels["failure_reason"])
	assert.Equal(t, "a1b2c3d4", labels["runner"])
	assert.Equal(t, "qwerty123", labels["runner_name"])
}

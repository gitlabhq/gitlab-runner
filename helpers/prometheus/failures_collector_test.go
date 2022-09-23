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

	fc := NewFailuresCollector()
	fc.RecordFailure(common.ScriptFailure, "a1b2c3d4")
	fc.RecordFailure(common.RunnerSystemFailure, "e5f67890")

	fc.Collect(ch)
	assert.Len(t, ch, 2)
}

func TestFailuresCollector_Collect_MetricsValues(t *testing.T) {
	ch := make(chan prometheus.Metric, 50)

	fc := NewFailuresCollector()
	fc.RecordFailure(common.ScriptFailure, "a1b2c3d4")
	fc.RecordFailure(common.ScriptFailure, "a1b2c3d4")

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
}

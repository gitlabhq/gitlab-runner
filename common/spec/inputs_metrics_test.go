//go:build !integration

package spec

import (
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	_ prometheus.Collector = (*JobInputsMetricsCollector)(nil)
)

func TestJobInputsInterpolationMetrics(t *testing.T) {
	testMetrics := NewJobInputsMetricsCollector()

	t.Run("tracks successful interpolation when output differs", func(t *testing.T) {
		inputs := prepareTestInputs(t, `[
			{
				"key": "name",
				"value": {
					"type": "string",
					"content": "John",
					"sensitive": false
				}
			}
		]`)
		inputs.SetMetricsCollector(testMetrics)

		beforeCount := getCounterValue(t, testMetrics.interpolations)

		result, err := inputs.Expand("Hello ${{ job.inputs.name }}")
		require.NoError(t, err)
		assert.Equal(t, "Hello John", result)

		afterCount := getCounterValue(t, testMetrics.interpolations)
		assert.Equal(t, beforeCount+1, afterCount, "should increment interpolations counter")
	})

	t.Run("does not track when output is same as input", func(t *testing.T) {
		inputs := prepareTestInputs(t, `[
			{
				"key": "name",
				"value": {
					"type": "string",
					"content": "John",
					"sensitive": false
				}
			}
		]`)
		inputs.SetMetricsCollector(testMetrics)

		beforeCount := getCounterValue(t, testMetrics.interpolations)

		result, err := inputs.Expand("Hello World")
		require.NoError(t, err)
		assert.Equal(t, "Hello World", result)

		afterCount := getCounterValue(t, testMetrics.interpolations)
		assert.Equal(t, beforeCount, afterCount, "should not increment interpolations counter when no expression is used")
	})

	t.Run("tracks errors", func(t *testing.T) {
		tests := []struct {
			inputs string
			typ    string
			text   string
		}{
			{
				inputs: `
					[
						{
							"key": "name",
							"value": {
								"type": "string",
								"content": "John",
								"sensitive": false
							}
						}
					]
				`,
				typ:  interpolationErrorTypeParse,
				text: "Hello ${{ job.inputs.name + }}",
			},
			{
				inputs: `
					[
						{
							"key": "name",
							"value": {
								"type": "string",
								"content": "John",
								"sensitive": false
							}
						}
					]
				`,
				typ:  interpolationErrorTypeEvaluation,
				text: "Hello ${{ job.inputs.nonexistent }}",
			},
			{
				inputs: `
					[
						{
							"key": "name",
							"value": {
								"type": "string",
								"content": "John",
								"sensitive": true
							}
						}
					]
				`,
				typ:  interpolationErrorTypeSensitiveUnsupported,
				text: "Hello ${{ job.inputs.name }}",
			},
		}

		for _, tt := range tests {
			t.Run(fmt.Sprintf("error type %s", tt.typ), func(t *testing.T) {
				inputs := prepareTestInputs(t, tt.inputs)
				inputs.SetMetricsCollector(testMetrics)

				beforeCount := getCounterValueWithLabel(t, testMetrics.interpolationFailures, tt.typ)

				_, err := inputs.Expand(tt.text)
				require.Error(t, err)

				afterCount := getCounterValueWithLabel(t, testMetrics.interpolationFailures, tt.typ)
				assert.Equal(t, beforeCount+1, afterCount)
			})
		}
	})
}

func prepareTestInputs(t *testing.T, jsonData string) *Inputs {
	t.Helper()

	var inputs Inputs
	err := inputs.UnmarshalJSON([]byte(jsonData))
	require.NoError(t, err)
	return &inputs
}

func getCounterValue(t *testing.T, counter prometheus.Counter) float64 {
	t.Helper()

	metric := &dto.Metric{}
	err := counter.Write(metric)
	require.NoError(t, err)
	return metric.Counter.GetValue()
}

func getCounterValueWithLabel(t *testing.T, counterVec *prometheus.CounterVec, labelValue string) float64 {
	t.Helper()

	counter := counterVec.WithLabelValues(labelValue)
	return getCounterValue(t, counter)
}

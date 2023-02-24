//go:build !integration

package network

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	prometheus_go "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRequestsCollector_Collect(t *testing.T) {
	var metrics []prometheus.Metric

	ch := make(chan prometheus.Metric)

	wg := new(sync.WaitGroup)
	wg.Add(1)

	go func() {
		defer wg.Done()

		for metric := range ch {
			metrics = append(metrics, metric)
		}
	}()

	c := newAPIRequestCollectorWithBuckets([]float64{0.1, 1, 10})

	// data for one metric entry
	assert.NoError(t, c.observe("runner1", "system1", apiEndpointUpdateJob, http.StatusOK, 0.05))
	assert.NoError(t, c.observe("runner1", "system1", apiEndpointUpdateJob, http.StatusOK, 0.05))
	assert.NoError(t, c.observe("runner1", "system1", apiEndpointUpdateJob, http.StatusOK, 0.5))

	// data for one metric entry
	assert.NoError(t, c.observe("runner1", "system1", apiEndpointUpdateJob, http.StatusNotFound, 1.5))
	assert.NoError(t, c.observe("runner1", "system1", apiEndpointUpdateJob, http.StatusNotFound, 15))

	// data for one metric entry
	assert.NoError(t, c.observe("runner1", "system1", apiEndpointRequestJob, http.StatusOK, 0.05))
	assert.NoError(t, c.observe("runner1", "system1", apiEndpointRequestJob, http.StatusOK, 1.5))

	// data for one metric entry
	assert.NoError(t, c.observe("runner2", "system1", apiEndpointRequestJob, http.StatusOK, 0.05))
	assert.NoError(t, c.observe("runner2", "system1", apiEndpointRequestJob, http.StatusOK, 0.05))
	assert.NoError(t, c.observe("runner2", "system1", apiEndpointRequestJob, http.StatusOK, 1.5))

	c.Collect(ch)
	close(ch)

	wg.Wait()

	require.Len(t, metrics, 7)

	assertStatusMetrics(t, metrics)
	assertDurationMetrics(t, metrics)
}

func assertStatusMetrics(t *testing.T, list []prometheus.Metric) {
	rx, err := regexp.Compile("fqName: \"gitlab_runner_api_request_statuses_total\"")
	require.NoError(t, err)

	metrics := make(map[string]float64)
	for _, m := range list {
		desc := m.Desc()
		require.NotNil(t, desc)

		if !rx.MatchString(desc.String()) {
			continue
		}

		var d prometheus_go.Metric

		err := m.Write(&d)
		require.NoError(t, err)

		var labels []string
		for _, label := range d.Label {
			require.NotNil(t, label)
			labels = append(labels, fmt.Sprintf("%s-%s", label.GetName(), label.GetValue()))
		}
		sort.Strings(labels)

		counter := d.GetCounter()
		require.NotNil(t, counter)

		metrics[strings.Join(labels, "-")] = d.GetCounter().GetValue()
	}

	expected := map[string]float64{
		"endpoint-update_job-runner-runner1-status-200-system_id-system1":  3,
		"endpoint-update_job-runner-runner1-status-404-system_id-system1":  2,
		"endpoint-request_job-runner-runner1-status-200-system_id-system1": 2,
		"endpoint-request_job-runner-runner2-status-200-system_id-system1": 3,
	}

	assert.Equal(t, expected, metrics)
}

func assertDurationMetrics(t *testing.T, list []prometheus.Metric) {
	rx, err := regexp.Compile("fqName: \"gitlab_runner_api_request_duration_seconds\"")
	require.NoError(t, err)

	type hMetric struct {
		count   uint64
		sum     float64
		buckets map[float64]uint64
	}

	metrics := make(map[string]hMetric)
	for _, m := range list {
		desc := m.Desc()
		require.NotNil(t, desc)

		if !rx.MatchString(desc.String()) {
			continue
		}

		var d prometheus_go.Metric

		err := m.Write(&d)
		require.NoError(t, err)

		var labels []string
		for _, label := range d.Label {
			require.NotNil(t, label)
			labels = append(labels, fmt.Sprintf("%s-%s", label.GetName(), label.GetValue()))
		}
		sort.Strings(labels)

		histogram := d.GetHistogram()
		require.NotNil(t, histogram)

		hm := hMetric{
			count:   histogram.GetSampleCount(),
			sum:     histogram.GetSampleSum(),
			buckets: make(map[float64]uint64),
		}

		for _, bucket := range histogram.GetBucket() {
			if bucket != nil {
				hm.buckets[bucket.GetUpperBound()] = bucket.GetCumulativeCount()
			}
		}
		metrics[strings.Join(labels, "-")] = hm
	}

	expected := map[string]hMetric{
		"endpoint-update_job-runner-runner1-system_id-system1": {
			count: 5,
			sum:   17.1,
			buckets: map[float64]uint64{
				0.1: 2,
				1:   3,
				10:  4,
			},
		},
		"endpoint-request_job-runner-runner1-system_id-system1": {
			count: 2,
			sum:   1.55,
			buckets: map[float64]uint64{
				0.1: 1,
				1:   1,
				10:  2,
			},
		},
		"endpoint-request_job-runner-runner2-system_id-system1": {
			count: 3,
			sum:   1.6,
			buckets: map[float64]uint64{
				0.1: 2,
				1:   2,
				10:  3,
			},
		},
	}

	assert.Equal(t, expected, metrics)
}

func TestAPIRequestsCollector_Describe(t *testing.T) {
	var descriptions []*prometheus.Desc

	ch := make(chan *prometheus.Desc)

	wg := new(sync.WaitGroup)
	wg.Add(1)

	go func() {
		defer wg.Done()

		for desc := range ch {
			descriptions = append(descriptions, desc)
		}
	}()

	c := NewAPIRequestsCollector()
	c.Describe(ch)
	close(ch)

	wg.Wait()

	require.Len(t, descriptions, 2)
}

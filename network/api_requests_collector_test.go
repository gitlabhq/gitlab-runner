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
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRequestsCollector_Collect(t *testing.T) {
	var metrics []prometheus.Metric

	ch := make(chan prometheus.Metric)

	wg := new(sync.WaitGroup)
	wg.Go(func() {
		for metric := range ch {
			metrics = append(metrics, metric)
		}
	})

	c := newAPIRequestCollectorWithBuckets([]float64{0.1, 1, 10})

	runner1UpdateJob := apiRequestLabels{runnerID: "runner1", runnerName: "one", systemID: "system1", endpoint: apiEndpointUpdateJob}
	runner1RequestJob := apiRequestLabels{runnerID: "runner1", runnerName: "one", systemID: "system1", endpoint: apiEndpointRequestJob}
	runner2RequestJob := apiRequestLabels{runnerID: "runner2", runnerName: "two", systemID: "system1", endpoint: apiEndpointRequestJob}

	// data for one metric entry
	assert.NoError(t, c.observe(runner1UpdateJob, http.StatusOK, http.MethodPost, 0.05))
	assert.NoError(t, c.observe(runner1UpdateJob, http.StatusOK, http.MethodPost, 0.05))
	assert.NoError(t, c.observe(runner1UpdateJob, http.StatusOK, http.MethodPost, 0.5))

	// data for one metric entry
	assert.NoError(t, c.observe(runner1UpdateJob, http.StatusNotFound, http.MethodPost, 1.5))
	assert.NoError(t, c.observe(runner1UpdateJob, http.StatusNotFound, http.MethodPost, 15))

	// data for one metric entry
	assert.NoError(t, c.observe(runner1RequestJob, http.StatusOK, http.MethodPost, 0.05))
	assert.NoError(t, c.observe(runner1RequestJob, http.StatusOK, http.MethodPost, 1.5))

	// data for one metric entry
	assert.NoError(t, c.observe(runner2RequestJob, http.StatusOK, http.MethodPost, 0.05))
	assert.NoError(t, c.observe(runner2RequestJob, http.StatusOK, http.MethodPost, 0.05))
	assert.NoError(t, c.observe(runner2RequestJob, http.StatusOK, http.MethodPost, 1.5))

	c.AddAttempt(logrus.StandardLogger(), runner1RequestJob, "429", http.MethodPost)
	c.AddAttempt(logrus.StandardLogger(), runner1RequestJob, "429", http.MethodPost)
	c.AddAttempt(logrus.StandardLogger(), runner1RequestJob, "204", http.MethodPost)
	c.AddAttempt(logrus.StandardLogger(), runner1RequestJob, attemptStatusError, http.MethodPost)

	// data for retry counter
	c.AddRetries(logrus.StandardLogger(), "test-path", http.MethodGet, 1)
	c.AddRetries(logrus.StandardLogger(), "test-path", http.MethodGet, 1)
	c.AddRetries(logrus.StandardLogger(), "test-path", http.MethodPost, 1)
	c.AddRetries(logrus.StandardLogger(), "test-path", http.MethodPost, 1)

	c.Collect(ch)
	close(ch)

	wg.Wait()

	require.Len(t, metrics, 13)

	assertStatusMetrics(t, metrics)
	assertAttemptsMetrics(t, metrics)
	assertDurationMetrics(t, metrics)
	assertRetriesMetrics(t, metrics)
}

func collectCounterValues(t *testing.T, list []prometheus.Metric, name string) map[string]float64 {
	rx, err := regexp.Compile(fmt.Sprintf("fqName: %q", name))
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

		metrics[strings.Join(labels, "-")] = counter.GetValue()
	}

	return metrics
}

func assertAttemptsMetrics(t *testing.T, list []prometheus.Metric) {
	expected := map[string]float64{
		"endpoint-request_job-method-post-runner-runner1-runner_name-one-status-429-system_id-system1":   2,
		"endpoint-request_job-method-post-runner-runner1-runner_name-one-status-204-system_id-system1":   1,
		"endpoint-request_job-method-post-runner-runner1-runner_name-one-status-error-system_id-system1": 1,
	}

	assert.Equal(t, expected, collectCounterValues(t, list, "gitlab_runner_api_request_attempts_total"))
}

func assertStatusMetrics(t *testing.T, list []prometheus.Metric) {
	expected := map[string]float64{
		"endpoint-update_job-method-post-runner-runner1-runner_name-one-status-200-system_id-system1":  3,
		"endpoint-update_job-method-post-runner-runner1-runner_name-one-status-404-system_id-system1":  2,
		"endpoint-request_job-method-post-runner-runner1-runner_name-one-status-200-system_id-system1": 2,
		"endpoint-request_job-method-post-runner-runner2-runner_name-two-status-200-system_id-system1": 3,
	}

	assert.Equal(t, expected, collectCounterValues(t, list, "gitlab_runner_api_request_statuses_total"))
}

func assertRetriesMetrics(t *testing.T, list []prometheus.Metric) {
	expected := map[string]float64{
		"method-get-path-test-path":  2,
		"method-post-path-test-path": 2,
	}

	assert.Equal(t, expected, collectCounterValues(t, list, "gitlab_runner_api_request_retries_total"))
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
		"endpoint-request_job-method-post-runner-runner1-runner_name-one-status_class-2xx-system_id-system1": {
			count: 2,
			sum:   1.55,
			buckets: map[float64]uint64{
				0.1: 1,
				1:   1,
				10:  2,
			},
		},
		"endpoint-update_job-method-post-runner-runner1-runner_name-one-status_class-2xx-system_id-system1": {
			count: 3,
			sum:   0.6,
			buckets: map[float64]uint64{
				0.1: 2,
				1:   3,
				10:  3,
			},
		},
		"endpoint-update_job-method-post-runner-runner1-runner_name-one-status_class-4xx-system_id-system1": {
			count: 2,
			sum:   16.5,
			buckets: map[float64]uint64{
				0.1: 0,
				1:   0,
				10:  1,
			},
		},
		"endpoint-request_job-method-post-runner-runner2-runner_name-two-status_class-2xx-system_id-system1": {
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
	wg.Go(func() {
		for desc := range ch {
			descriptions = append(descriptions, desc)
		}
	})

	c := NewAPIRequestsCollector()
	c.Describe(ch)
	close(ch)

	wg.Wait()

	require.Len(t, descriptions, 4)
}

func TestStatusClass(t *testing.T) {
	testCases := []struct {
		status   int
		expected string
	}{
		{
			status:   0,
			expected: "unknown",
		},
		{
			status:   99,
			expected: "unknown",
		},
		{
			status:   100,
			expected: "1xx",
		},
		{
			status:   150,
			expected: "1xx",
		},
		{
			status:   200,
			expected: "2xx",
		},
		{
			status:   250,
			expected: "2xx",
		},
		{
			status:   300,
			expected: "3xx",
		},
		{
			status:   350,
			expected: "3xx",
		},
		{
			status:   400,
			expected: "4xx",
		},
		{
			status:   450,
			expected: "4xx",
		},
		{
			status:   500,
			expected: "5xx",
		},
		{
			status:   550,
			expected: "5xx",
		},
		{
			status:   599,
			expected: "5xx",
		},
		{
			status:   600,
			expected: "unknown",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("status %d mapped to %s", tc.status, tc.expected), func(t *testing.T) {
			result := statusClass(tc.status)
			assert.Equal(t, tc.expected, result)
		})
	}
}

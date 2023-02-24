package network

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type apiEndpoint string

const (
	apiEndpointResetToken apiEndpoint = "reset_token"
	apiEndpointRequestJob apiEndpoint = "request_job"
	apiEndpointUpdateJob  apiEndpoint = "update_job"
	apiEndpointPatchTrace apiEndpoint = "patch_trace"
)

var (
	_ prometheus.Collector = new(APIRequestsCollector)

	requestDurationBuckets = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}
)

type APIRequestsCollector struct {
	lock sync.RWMutex

	statuses  *prometheus.CounterVec
	durations *prometheus.HistogramVec
}

func NewAPIRequestsCollector() *APIRequestsCollector {
	return newAPIRequestCollectorWithBuckets(requestDurationBuckets)
}

func newAPIRequestCollectorWithBuckets(buckets []float64) *APIRequestsCollector {
	return &APIRequestsCollector{
		statuses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_api_request_statuses_total",
				Help: "The total number of api requests, partitioned by runner, system_id, endpoint and status.",
			},
			[]string{"runner", "system_id", "endpoint", "status"},
		),
		durations: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gitlab_runner_api_request_duration_seconds",
				Help:    "Latency histogram of API requests made by GitLab Runner",
				Buckets: buckets,
			},
			[]string{"runner", "system_id", "endpoint"},
		),
	}
}

func (rc *APIRequestsCollector) Observe(
	logger logrus.FieldLogger,
	runnerID string,
	systemID string,
	endpoint apiEndpoint,
	fn func() int,
) {
	requestStart := time.Now()
	status := fn()

	if status == clientError {
		return
	}

	err := rc.observe(
		runnerID,
		systemID,
		endpoint,
		status,
		time.Since(requestStart).Seconds(),
	)
	if err != nil {
		logger.WithError(err).Warning("Updating apiRequestsCollector")
	}
}

func (rc *APIRequestsCollector) observe(
	runnerID string,
	systemID string,
	endpoint apiEndpoint,
	status int,
	duration float64,
) error {
	rc.lock.Lock()
	defer rc.lock.Unlock()

	statusCounter, err := rc.statuses.GetMetricWith(prometheus.Labels{
		"runner":    runnerID,
		"system_id": systemID,
		"endpoint":  string(endpoint),
		"status":    strconv.Itoa(status),
	})
	if err != nil {
		return fmt.Errorf("requesting status counter: %w", err)
	}
	statusCounter.Inc()

	durationHist, err := rc.durations.GetMetricWith(prometheus.Labels{
		"runner":    runnerID,
		"system_id": systemID,
		"endpoint":  string(endpoint),
	})
	if err != nil {
		return fmt.Errorf("requesting durations histogram: %w", err)
	}

	durationHist.Observe(duration)

	return nil
}

// Describe implements prometheus.Collector.
func (rc *APIRequestsCollector) Describe(ch chan<- *prometheus.Desc) {
	rc.statuses.Describe(ch)
	rc.durations.Describe(ch)
}

// Collect implements prometheus.Collector.
func (rc *APIRequestsCollector) Collect(ch chan<- prometheus.Metric) {
	rc.lock.RLock()
	defer rc.lock.RUnlock()

	rc.statuses.Collect(ch)
	rc.durations.Collect(ch)
}

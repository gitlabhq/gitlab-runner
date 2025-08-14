package network

import (
	"fmt"
	"strconv"
	"strings"
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
	retries   *prometheus.CounterVec
}

func NewAPIRequestsCollector() *APIRequestsCollector {
	return newAPIRequestCollectorWithBuckets(requestDurationBuckets)
}

func newAPIRequestCollectorWithBuckets(buckets []float64) *APIRequestsCollector {
	return &APIRequestsCollector{
		statuses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_api_request_statuses_total",
				Help: "The total number of API requests made by GitLab Runner, partitioned by runner, system_id, endpoint, status and method.",
			},
			[]string{"runner", "system_id", "endpoint", "status", "method"},
		),
		durations: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gitlab_runner_api_request_duration_seconds",
				Help:    "Latency histogram of API requests made by GitLab Runner, partitioned by runner, system_id, endpoint, status_class and method.",
				Buckets: buckets,
			},
			[]string{"runner", "system_id", "endpoint", "status_class", "method"},
		),
		retries: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_api_request_retries_total",
				Help: "The total number of retries made by GitLab Runner in process of completing a request, partitioned by path and method.",
			},
			[]string{"path", "method"},
		),
	}
}

func (rc *APIRequestsCollector) Observe(
	logger logrus.FieldLogger,
	runnerID string,
	systemID string,
	endpoint apiEndpoint,
	fn func() (int, string),
) {
	requestStart := time.Now()
	status, method := fn()

	if status == clientError {
		return
	}

	err := rc.observe(
		runnerID,
		systemID,
		endpoint,
		status,
		method,
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
	method string,
	duration float64,
) error {
	rc.lock.Lock()
	defer rc.lock.Unlock()

	ep := string(endpoint)
	st := strconv.Itoa(status)
	md := strings.ToLower(method)

	statusCounter, err := rc.statuses.GetMetricWith(prometheus.Labels{
		"runner":    runnerID,
		"system_id": systemID,
		"endpoint":  ep,
		"status":    st,
		"method":    md,
	})
	if err != nil {
		return fmt.Errorf("requesting status counter: %w", err)
	}
	statusCounter.Inc()

	durationHist, err := rc.durations.GetMetricWith(prometheus.Labels{
		"runner":       runnerID,
		"system_id":    systemID,
		"endpoint":     ep,
		"status_class": statusClass(status),
		"method":       md,
	})
	if err != nil {
		return fmt.Errorf("requesting durations histogram: %w", err)
	}

	durationHist.Observe(duration)

	return nil
}

// AddRetries adds to the retries counter with the given path
// and method the passed in value.
func (rc *APIRequestsCollector) AddRetries(logger logrus.FieldLogger, path string, method string, val float64) {
	rc.lock.Lock()
	defer rc.lock.Unlock()

	retriesCounter, err := rc.retries.GetMetricWith(prometheus.Labels{
		"path":   path,
		"method": strings.ToLower(method),
	})
	if err != nil {
		logger.WithError(err).Warning("Updating apiRequestsCollector")
		return
	}
	retriesCounter.Add(val)
}

// Describe implements prometheus.Collector.
func (rc *APIRequestsCollector) Describe(ch chan<- *prometheus.Desc) {
	rc.statuses.Describe(ch)
	rc.durations.Describe(ch)
	rc.retries.Describe(ch)
}

// Collect implements prometheus.Collector.
func (rc *APIRequestsCollector) Collect(ch chan<- prometheus.Metric) {
	rc.lock.RLock()
	defer rc.lock.RUnlock()

	rc.statuses.Collect(ch)
	rc.durations.Collect(ch)
	rc.retries.Collect(ch)
}

func statusClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	case status >= 200:
		return "2xx"
	case status >= 100:
		return "1xx"
	default:
		return "unknown"
	}
}

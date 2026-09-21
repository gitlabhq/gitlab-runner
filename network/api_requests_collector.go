package network

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type apiEndpoint string

const (
	apiEndpointResetToken apiEndpoint = "reset_token"
	apiEndpointRequestJob apiEndpoint = "request_job"
	apiEndpointUpdateJob  apiEndpoint = "update_job"
	apiEndpointPatchTrace apiEndpoint = "patch_trace"
	apiEndpointDiscovery  apiEndpoint = "discovery"
)

var (
	_ prometheus.Collector = new(APIRequestsCollector)

	requestDurationBuckets = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}
)

type apiRequestLabelsKey struct{}

type apiRequestLabels struct {
	runnerID   string
	runnerName string
	systemID   string
	endpoint   apiEndpoint
}

func runnerRequestLabels(config common.RunnerConfig, endpoint apiEndpoint) apiRequestLabels {
	return apiRequestLabels{
		runnerID:   config.RunnerCredentials.ShortDescription(),
		runnerName: config.Name,
		systemID:   config.SystemID,
		endpoint:   endpoint,
	}
}

func withAPIRequestLabels(ctx context.Context, labels apiRequestLabels) context.Context {
	return context.WithValue(ctx, apiRequestLabelsKey{}, labels)
}

// Requests made outside a runner context get the normalized path as endpoint.
func apiRequestLabelsFromContext(ctx context.Context, path string) apiRequestLabels {
	if l, ok := ctx.Value(apiRequestLabelsKey{}).(apiRequestLabels); ok {
		return l
	}
	return apiRequestLabels{endpoint: apiEndpoint(normalizedURI(path))}
}

type APIRequestsCollector struct {
	lock sync.RWMutex

	statuses  *prometheus.CounterVec
	attempts  *prometheus.CounterVec
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
				Help: "The total number of API requests made by GitLab Runner, partitioned by runner, runner_name, system_id, endpoint, status and method.",
			},
			[]string{"runner", "runner_name", "system_id", "endpoint", "status", "method"},
		),
		attempts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_api_request_attempts_total",
				Help: "The total number of HTTP attempts made by GitLab Runner while completing API requests, including retried attempts, partitioned by runner, runner_name, system_id, endpoint, status and method. Attempts that got no HTTP response have status=\"error\".",
			},
			[]string{"runner", "runner_name", "system_id", "endpoint", "status", "method"},
		),
		durations: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gitlab_runner_api_request_duration_seconds",
				Help:    "Latency histogram of API requests made by GitLab Runner, partitioned by runner, runner_name, system_id, endpoint, status_class and method.",
				Buckets: buckets,
			},
			[]string{"runner", "runner_name", "system_id", "endpoint", "status_class", "method"},
		),
		retries: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_api_request_retries_total",
				Help: "The total number of HTTP attempts (including the first one, not only retries) made by GitLab Runner in process of completing a request, partitioned by path and method. Prefer gitlab_runner_api_request_attempts_total, which also has the status.",
			},
			[]string{"path", "method"},
		),
	}
}

func (rc *APIRequestsCollector) Observe(
	logger logrus.FieldLogger,
	labels apiRequestLabels,
	fn func() (int, string),
) {
	requestStart := time.Now()
	status, method := fn()

	if status == clientError {
		return
	}

	err := rc.observe(labels, status, method, time.Since(requestStart).Seconds())
	if err != nil {
		logger.WithError(err).Warning("Updating apiRequestsCollector")
	}
}

func (rc *APIRequestsCollector) observe(labels apiRequestLabels, status int, method string, duration float64) error {
	rc.lock.Lock()
	defer rc.lock.Unlock()

	statusCounter, err := rc.statuses.GetMetricWith(labels.prometheusLabels(strconv.Itoa(status), method, "status"))
	if err != nil {
		return fmt.Errorf("requesting status counter: %w", err)
	}
	statusCounter.Inc()

	durationHist, err := rc.durations.GetMetricWith(labels.prometheusLabels(statusClass(status), method, "status_class"))
	if err != nil {
		return fmt.Errorf("requesting durations histogram: %w", err)
	}

	durationHist.Observe(duration)

	return nil
}

func (rc *APIRequestsCollector) AddAttempt(logger logrus.FieldLogger, labels apiRequestLabels, status string, method string) {
	rc.lock.Lock()
	defer rc.lock.Unlock()

	counter, err := rc.attempts.GetMetricWith(labels.prometheusLabels(status, method, "status"))
	if err != nil {
		logger.WithError(err).Warning("Updating apiRequestsCollector")
		return
	}
	counter.Inc()
}

func (l apiRequestLabels) prometheusLabels(status, method, statusLabel string) prometheus.Labels {
	return prometheus.Labels{
		"runner":      l.runnerID,
		"runner_name": l.runnerName,
		"system_id":   l.systemID,
		"endpoint":    string(l.endpoint),
		statusLabel:   status,
		"method":      strings.ToLower(method),
	}
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
	rc.attempts.Describe(ch)
	rc.durations.Describe(ch)
	rc.retries.Describe(ch)
}

// Collect implements prometheus.Collector.
func (rc *APIRequestsCollector) Collect(ch chan<- prometheus.Metric) {
	rc.lock.RLock()
	defer rc.lock.RUnlock()

	rc.statuses.Collect(ch)
	rc.attempts.Collect(ch)
	rc.durations.Collect(ch)
	rc.retries.Collect(ch)
}

func statusClass(status int) string {
	switch {
	case status >= 600:
		return "unknown"
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

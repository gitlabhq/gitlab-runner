package router

import "github.com/prometheus/client_golang/prometheus"

const (
	metricsNamespace = "gitlab_runner"
	metricsSubsystem = "job_router"

	// discovery cache event results, used as the "result" label value.
	cacheHit  = "hit"
	cacheMiss = "miss"
)

// fallbackReason records why a job request fell back from the router to direct
// GitLab polling. It is the value of the "reason" label on the fallbacks_total
// counter; the named type keeps the label bounded to the constants below rather
// than arbitrary strings, so the metric cannot grow unbounded cardinality.
type fallbackReason string

const (
	fallbackNone           fallbackReason = ""                // no fallback: the router handled the request
	fallbackNoDiscovery    fallbackReason = "no_discovery"    // discovery returned no router
	fallbackBreakerOpen    fallbackReason = "breaker_open"    // circuit breaker was open, router skipped
	fallbackDialFailed     fallbackReason = "dial_failed"     // could not dial the router
	fallbackBreakerTripped fallbackReason = "breaker_tripped" // router unreachable, breaker tripped this request
	fallbackRouterDisabled fallbackReason = "router_disabled" // router answered Unimplemented (deliberately disabled)
)

// getJobResult records the outcome of a router GetJob RPC. It is the value of the
// "result" label on the get_job_duration_seconds histogram, and mirrors the values
// GitLab Relay (KAS) records on its own job_router_get_job_duration_seconds so the runner-side and
// server-side histograms can be broken down the same way.
type getJobResult string

const (
	getJobResultJob   getJobResult = "job"    // the router returned a job
	getJobResultNoJob getJobResult = "no_job" // the router returned no job, the usual long-poll outcome
	getJobResultError getJobResult = "error"  // the GetJob RPC failed
)

// clientMetrics holds the Prometheus metrics for the router Client's own
// behaviour (the circuit breaker owns its metrics separately). It implements
// prometheus.Collector so the Client can delegate to it.
type clientMetrics struct {
	discoveryCacheEvents *prometheus.CounterVec
	fallbacks            *prometheus.CounterVec
	getJobDuration       *prometheus.HistogramVec
}

func newClientMetrics() *clientMetrics {
	return &clientMetrics{
		discoveryCacheEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "discovery_cache_events_total",
			Help:      "Total number of job router discovery cache lookups, partitioned by result (hit or miss).",
		}, []string{"result"}),
		fallbacks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "fallbacks_total",
			Help:      "Total number of job requests that fell back from the job router to direct GitLab polling, partitioned by reason.",
		}, []string{"reason"}),
		// runner/system_id attribute the latency to the runner entry that polled, and
		// result matches the GitLab Relay (KAS)-side histogram so the two break down the same way.
		// Without result, no-job long-polls - which wait out GitLab's CI polling window
		// - swamp the latency tail of every other outcome. Cardinality stays bounded by
		// the [[runners]] entries on a host: 39 series each, being 3 results x
		// (11 buckets + _sum + _count).
		getJobDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "get_job_duration_seconds",
			Help:      "Latency histogram of runner-side job router GetJob requests, partitioned by runner, system_id and result (job, no_job or error).",
			// Covers the latency range of a router GetJob request, mirroring the
			// buckets used for direct GitLab API requests (see the network package)
			// so the two are comparable when diagnosing degradation. The range
			// extends past the gRPC deadline so tail latencies are not all collapsed
			// into +Inf.
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
		}, []string{"runner", "system_id", "result"}),
	}
}

// recordCacheEvent records a discovery cache hit or miss.
func (m *clientMetrics) recordCacheEvent(result string) {
	m.discoveryCacheEvents.WithLabelValues(result).Inc()
}

// recordFallback records a fallback to direct GitLab polling with its reason.
func (m *clientMetrics) recordFallback(reason fallbackReason) {
	m.fallbacks.WithLabelValues(string(reason)).Inc()
}

// observeGetJob records the duration and outcome of a router GetJob request for
// the runner that issued it.
func (m *clientMetrics) observeGetJob(runnerID, systemID string, result getJobResult, seconds float64) {
	m.getJobDuration.WithLabelValues(runnerID, systemID, string(result)).Observe(seconds)
}

// Describe implements prometheus.Collector.
func (m *clientMetrics) Describe(ch chan<- *prometheus.Desc) {
	m.discoveryCacheEvents.Describe(ch)
	m.fallbacks.Describe(ch)
	m.getJobDuration.Describe(ch)
}

// Collect implements prometheus.Collector.
func (m *clientMetrics) Collect(ch chan<- prometheus.Metric) {
	m.discoveryCacheEvents.Collect(ch)
	m.fallbacks.Collect(ch)
	m.getJobDuration.Collect(ch)
}

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

// clientMetrics holds the Prometheus metrics for the router Client's own
// behaviour (the circuit breaker owns its metrics separately). It implements
// prometheus.Collector so the Client can delegate to it.
type clientMetrics struct {
	discoveryCacheEvents *prometheus.CounterVec
	fallbacks            *prometheus.CounterVec
	getJobDuration       prometheus.Histogram
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
		getJobDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "get_job_duration_seconds",
			Help:      "Latency histogram of runner-side job router GetJob requests, regardless of outcome.",
			// Covers the latency range of a router GetJob request, mirroring the
			// buckets used for direct GitLab API requests (see the network package)
			// so the two are comparable when diagnosing degradation. The range
			// extends past the gRPC deadline so tail latencies are not all collapsed
			// into +Inf.
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
		}),
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

// observeGetJob records the duration of a router GetJob request.
func (m *clientMetrics) observeGetJob(seconds float64) {
	m.getJobDuration.Observe(seconds)
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

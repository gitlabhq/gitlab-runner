package spec

import "github.com/prometheus/client_golang/prometheus"

const (
	// Error type labels for interpolation failures
	interpolationErrorTypeParse                = "parse"
	interpolationErrorTypeEvaluation           = "evaluation"
	interpolationErrorTypeSensitiveUnsupported = "sensitive_unsupported"
)

type JobInputsMetricsCollector struct {
	interpolations        prometheus.Counter
	interpolationFailures *prometheus.CounterVec
}

func NewJobInputsMetricsCollector() *JobInputsMetricsCollector {
	return &JobInputsMetricsCollector{
		interpolations: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gitlab_runner_job_inputs_interpolations_total",
			Help: "Total number of job input interpolations where expressions were actually used (output differs from input)",
		}),
		interpolationFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_job_inputs_interpolation_failures_total",
				Help: "Total number of failed job input interpolations, labeled by error type",
			},
			[]string{"error_type"},
		),
	}
}

// Describe implements prometheus.Collector.
func (c *JobInputsMetricsCollector) Describe(descs chan<- *prometheus.Desc) {
	c.interpolations.Describe(descs)
	c.interpolationFailures.Describe(descs)
}

// Collect implements prometheus.Collector.
func (c *JobInputsMetricsCollector) Collect(metrics chan<- prometheus.Metric) {
	c.interpolations.Collect(metrics)
	c.interpolationFailures.Collect(metrics)
}

// recordSuccess increments the successful interpolations counter
func (c *JobInputsMetricsCollector) recordSuccess() {
	if c == nil {
		return
	}

	c.interpolations.Inc()
}

// recordParseError increments the parse error counter
func (c *JobInputsMetricsCollector) recordParseError() {
	if c == nil {
		return
	}

	c.interpolationFailures.WithLabelValues(interpolationErrorTypeParse).Inc()
}

// recordEvalError increments the evaluation error counter
func (c *JobInputsMetricsCollector) recordEvalError() {
	if c == nil {
		return
	}

	c.interpolationFailures.WithLabelValues(interpolationErrorTypeEvaluation).Inc()
}

// recordSensitiveUnsupportedError increments the sensitive input error counter
func (c *JobInputsMetricsCollector) recordSensitiveUnsupportedError() {
	if c == nil {
		return
	}

	c.interpolationFailures.WithLabelValues(interpolationErrorTypeSensitiveUnsupported).Inc()
}

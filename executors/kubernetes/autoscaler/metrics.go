package autoscaler

import "github.com/prometheus/client_golang/prometheus"

// Standard label names, consistent with other runner metrics.
var metricLabels = []string{"runner", "runner_name", "system_id"}

// Metrics for the Kubernetes pause pod autoscaler.
type Metrics struct {
	desiredPods     *prometheus.GaugeVec
	currentPods     *prometheus.GaugeVec
	reconcileErrors *prometheus.CounterVec
	scaleOperations *prometheus.CounterVec
}

// NewMetrics creates a new Metrics instance.
func NewMetrics() *Metrics {
	return &Metrics{
		desiredPods: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gitlab_runner_kubernetes_autoscaler_pause_pods_desired",
				Help: "The desired number of pause pods based on current policy.",
			},
			metricLabels,
		),
		currentPods: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gitlab_runner_kubernetes_autoscaler_pause_pods_current",
				Help: "The current number of pause pods.",
			},
			metricLabels,
		),
		reconcileErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_kubernetes_autoscaler_reconcile_errors_total",
				Help: "Total number of reconciliation errors.",
			},
			append(append([]string{}, metricLabels...), "reason"),
		),
		scaleOperations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_kubernetes_autoscaler_scale_operations_total",
				Help: "Total number of scale operations performed.",
			},
			append(append([]string{}, metricLabels...), "direction"), // direction: up, down
		),
	}
}

// Describe implements prometheus.Collector.
func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	m.desiredPods.Describe(ch)
	m.currentPods.Describe(ch)
	m.reconcileErrors.Describe(ch)
	m.scaleOperations.Describe(ch)
}

// Collect implements prometheus.Collector.
func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	m.desiredPods.Collect(ch)
	m.currentPods.Collect(ch)
	m.reconcileErrors.Collect(ch)
	m.scaleOperations.Collect(ch)
}

// SetDesiredPods sets the desired pods gauge.
func (m *Metrics) SetDesiredPods(runner, runnerName, systemID string, count int) {
	m.desiredPods.WithLabelValues(runner, runnerName, systemID).Set(float64(count))
}

// SetCurrentPods sets the current pods gauge.
func (m *Metrics) SetCurrentPods(runner, runnerName, systemID string, count int) {
	m.currentPods.WithLabelValues(runner, runnerName, systemID).Set(float64(count))
}

// IncReconcileErrors increments the reconcile errors counter.
func (m *Metrics) IncReconcileErrors(runner, runnerName, systemID, reason string) {
	m.reconcileErrors.WithLabelValues(runner, runnerName, systemID, reason).Inc()
}

// IncScaleUp increments the scale up counter.
func (m *Metrics) IncScaleUp(runner, runnerName, systemID string) {
	m.scaleOperations.WithLabelValues(runner, runnerName, systemID, "up").Inc()
}

// IncScaleDown increments the scale down counter.
func (m *Metrics) IncScaleDown(runner, runnerName, systemID string) {
	m.scaleOperations.WithLabelValues(runner, runnerName, systemID, "down").Inc()
}

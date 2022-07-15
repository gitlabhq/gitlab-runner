package machine

import (
	"github.com/prometheus/client_golang/prometheus"
)

func (m *machineProvider) collectDetails() (data machinesData) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	for _, details := range m.details {
		if !details.isDead() {
			data.Add(details)
		}
	}
	return
}

// Describe implements prometheus.Collector.
func (m *machineProvider) Describe(ch chan<- *prometheus.Desc) {
	m.totalActions.Describe(ch)
	m.creationHistogram.Describe(ch)
	m.stoppingHistogram.Describe(ch)
	m.removalHistogram.Describe(ch)
	m.failedCreationHistogram.Describe(ch)
	ch <- m.currentStatesDesc
}

// Collect implements prometheus.Collector.
func (m *machineProvider) Collect(ch chan<- prometheus.Metric) {
	data := m.collectDetails()
	ch <- prometheus.MustNewConstMetric(
		m.currentStatesDesc,
		prometheus.GaugeValue,
		float64(data.Acquired),
		"acquired",
	)
	ch <- prometheus.MustNewConstMetric(
		m.currentStatesDesc,
		prometheus.GaugeValue,
		float64(data.Creating),
		"creating",
	)
	ch <- prometheus.MustNewConstMetric(
		m.currentStatesDesc,
		prometheus.GaugeValue,
		float64(data.Idle),
		"idle",
	)
	ch <- prometheus.MustNewConstMetric(
		m.currentStatesDesc,
		prometheus.GaugeValue,
		float64(data.Used),
		"used",
	)
	ch <- prometheus.MustNewConstMetric(
		m.currentStatesDesc,
		prometheus.GaugeValue,
		float64(data.Removing),
		"removing",
	)
	ch <- prometheus.MustNewConstMetric(
		m.currentStatesDesc,
		prometheus.GaugeValue,
		float64(data.StuckOnRemoving),
		"stuck-on-removing",
	)

	m.totalActions.Collect(ch)
	m.creationHistogram.Collect(ch)
	m.stoppingHistogram.Collect(ch)
	m.removalHistogram.Collect(ch)
	m.failedCreationHistogram.Collect(ch)
}

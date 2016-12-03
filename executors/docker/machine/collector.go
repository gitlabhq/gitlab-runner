package machine

import (
	"github.com/prometheus/client_golang/prometheus"
)

var machinesDataDesc = prometheus.NewDesc("ci_machines_provider", "The current number of machines in given state.", []string{"state"}, nil)
var providerStatisticsDesc = prometheus.NewDesc("ci_machines_provided", "The total number of machines created.", []string{"type"}, nil)

func (m *machineProvider) collectDetails() (data machinesData) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	for _, details := range m.details {
		data.Add(details.State)
	}
	return
}

// Describe implements prometheus.Collector.
func (m *machineProvider) Describe(ch chan<- *prometheus.Desc) {
	ch <- machinesDataDesc
	ch <- providerStatisticsDesc
}

// Collect implements prometheus.Collector.
func (m *machineProvider) Collect(ch chan<- prometheus.Metric) {
	data := m.collectDetails()
	ch <- prometheus.MustNewConstMetric(machinesDataDesc, prometheus.GaugeValue, float64(data.Acquired), "state=acquired")
	ch <- prometheus.MustNewConstMetric(machinesDataDesc, prometheus.GaugeValue, float64(data.Creating), "state=creating")
	ch <- prometheus.MustNewConstMetric(machinesDataDesc, prometheus.GaugeValue, float64(data.Idle), "state=idle")
	ch <- prometheus.MustNewConstMetric(machinesDataDesc, prometheus.GaugeValue, float64(data.Used), "state=used")
	ch <- prometheus.MustNewConstMetric(machinesDataDesc, prometheus.GaugeValue, float64(data.Removing), "state=removing")

	ch <- prometheus.MustNewConstMetric(machinesDataDesc, prometheus.CounterValue, float64(m.statistics.Created), "type=created")
	ch <- prometheus.MustNewConstMetric(machinesDataDesc, prometheus.CounterValue, float64(m.statistics.Used), "type=used")
	ch <- prometheus.MustNewConstMetric(machinesDataDesc, prometheus.CounterValue, float64(m.statistics.Removed), "type=removed")
}

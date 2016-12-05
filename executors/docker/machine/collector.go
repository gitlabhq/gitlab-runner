package machine

import (
	"github.com/prometheus/client_golang/prometheus"
)

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
	m.machinesDataDesc = prometheus.NewDesc("ci_"+m.name+"_provider", "The current number of machines in given state.", []string{"state"}, nil)
	m.providerStatisticsDesc = prometheus.NewDesc("ci_"+m.name, "The total number of machines created.", []string{"type"}, nil)

	ch <- m.machinesDataDesc
	ch <- m.providerStatisticsDesc
}

// Collect implements prometheus.Collector.
func (m *machineProvider) Collect(ch chan<- prometheus.Metric) {
	data := m.collectDetails()
	ch <- prometheus.MustNewConstMetric(m.machinesDataDesc, prometheus.GaugeValue, float64(data.Acquired), "acquired")
	ch <- prometheus.MustNewConstMetric(m.machinesDataDesc, prometheus.GaugeValue, float64(data.Creating), "creating")
	ch <- prometheus.MustNewConstMetric(m.machinesDataDesc, prometheus.GaugeValue, float64(data.Idle), "idle")
	ch <- prometheus.MustNewConstMetric(m.machinesDataDesc, prometheus.GaugeValue, float64(data.Used), "used")
	ch <- prometheus.MustNewConstMetric(m.machinesDataDesc, prometheus.GaugeValue, float64(data.Removing), "removing")

	ch <- prometheus.MustNewConstMetric(m.providerStatisticsDesc, prometheus.CounterValue, float64(m.statistics.Created), "created")
	ch <- prometheus.MustNewConstMetric(m.providerStatisticsDesc, prometheus.CounterValue, float64(m.statistics.Used), "used")
	ch <- prometheus.MustNewConstMetric(m.providerStatisticsDesc, prometheus.CounterValue, float64(m.statistics.Removed), "removed")
}

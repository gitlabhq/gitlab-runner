package machine

import (
	"github.com/prometheus/client_golang/prometheus"
)

// collectDetails snapshots the current machine population, bucketed by
// the operator-supplied target labels cached on each machineDetails.
// Each bucket holds the per-state counts for one (target_zone, target_region,
// target_project, target_machine_type) tuple, so
// Collect can emit one (state, target_*) series per bucket.
//
// Reading details.targets under details.Lock() pairs with the create path
// in provider.go which writes targets while holding the same lock.
func (m *machineProvider) collectDetails() map[targetLabels]*machinesData {
	m.lock.RLock()
	defer m.lock.RUnlock()

	out := make(map[targetLabels]*machinesData)
	for _, details := range m.details {
		details.Lock()
		info := details.info()
		targets := details.targets
		details.Unlock()

		if info.isDead() {
			continue
		}
		bucket, ok := out[targets]
		if !ok {
			bucket = &machinesData{}
			out[targets] = bucket
		}
		bucket.Add(info)
	}
	return out
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
	buckets := m.collectDetails()
	// Always emit at least one bucket so scrapers see fresh per-state
	// zeros even before the first machine has been created. The fallback
	// uses zero-valued target labels, matching the no-target-config case.
	if len(buckets) == 0 {
		buckets = map[targetLabels]*machinesData{{}: {}}
	}
	for targets, data := range buckets {
		emit := func(state string, value int) {
			labels := append([]string{state}, targets.values()...)
			ch <- prometheus.MustNewConstMetric(
				m.currentStatesDesc,
				prometheus.GaugeValue,
				float64(value),
				labels...,
			)
		}
		emit("acquired", data.Acquired)
		emit("creating", data.Creating)
		emit("idle", data.Idle)
		emit("used", data.Used)
		emit("removing", data.Removing)
		emit("stuck-on-removing", data.StuckOnRemoving)
	}

	m.totalActions.Collect(ch)
	m.creationHistogram.Collect(ch)
	m.stoppingHistogram.Collect(ch)
	m.removalHistogram.Collect(ch)
	m.failedCreationHistogram.Collect(ch)
}

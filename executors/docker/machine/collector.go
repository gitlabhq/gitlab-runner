package machine

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
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

		bucket, ok := out[targets]
		if !ok {
			bucket = &machinesData{}
			out[targets] = bucket
		}
		bucket.Add(info)
	}
	return out
}

type machineMetrics struct {
	totalActions            *prometheus.CounterVec
	currentStatesDesc       *prometheus.Desc
	creationHistogram       *prometheus.HistogramVec
	creationPhaseHistogram  *prometheus.HistogramVec
	failedCreationHistogram *prometheus.HistogramVec
	stoppingHistogram       prometheus.Histogram
	removalHistogram        prometheus.Histogram
	maxGrowthRateDesc       *prometheus.Desc
	idleTargetDesc          *prometheus.Desc
	warmupDesc              *prometheus.Desc
	warmupTimedOutDesc      *prometheus.Desc
	warmupReadyDesc         *prometheus.Desc
}

func newMachineMetrics(name string, config *common.MachineConfig) machineMetrics {
	constLabels := prometheus.Labels{"executor": name}
	runnerLabels := []string{"runner", "runner_name", "system_id"}

	metrics := machineMetrics{
		totalActions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "gitlab_runner_autoscaling_actions_total",
				Help:        "The total number of actions executed by the provider.",
				ConstLabels: constLabels,
			},
			append([]string{"action"}, targetLabelNames...),
		),
		currentStatesDesc: prometheus.NewDesc(
			"gitlab_runner_autoscaling_machine_states",
			"The current number of machines per state in this provider.",
			append([]string{"state"}, targetLabelNames...),
			constLabels,
		),
		creationHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "gitlab_runner_autoscaling_machine_creation_duration_seconds",
				Help:        "Histogram of machine creation time.",
				Buckets:     config.GetCreationDurationSecondsBuckets(),
				ConstLabels: constLabels,
			},
			targetLabelNames,
		),
		failedCreationHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "gitlab_runner_autoscaling_machine_failed_creation_duration_seconds",
				Help:        "Histogram of machine failed creation timings",
				Buckets:     []float64{1, 5, 10, 30, 60, 120, 240, 480, 900},
				ConstLabels: constLabels,
			},
			targetLabelNames,
		),
		stoppingHistogram: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:        "gitlab_runner_autoscaling_machine_stopping_duration_seconds",
				Help:        "Histogram of machine stopping time.",
				Buckets:     []float64{1, 3, 5, 10, 30, 50, 60, 80, 90, 120},
				ConstLabels: constLabels,
			},
		),
		removalHistogram: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:        "gitlab_runner_autoscaling_machine_removal_duration_seconds",
				Help:        "Histogram of machine removal time.",
				Buckets:     []float64{1, 3, 5, 10, 30, 50, 60, 80, 90, 120},
				ConstLabels: constLabels,
			},
		),
		maxGrowthRateDesc: prometheus.NewDesc(
			"gitlab_runner_autoscaling_max_growth_rate",
			"The maximum number of machines that can be in the creation state at once (0 = unlimited).",
			runnerLabels,
			constLabels,
		),
		idleTargetDesc: prometheus.NewDesc(
			"gitlab_runner_autoscaling_idle_target",
			"The number of available machines the autoscaler aims to keep after applying IdleScaleFactor and IdleCountMin.",
			runnerLabels,
			constLabels,
		),
		warmupDesc: prometheus.NewDesc(
			"gitlab_runner_autoscaling_warmup",
			"1 while the runner is in warm-up, 0 otherwise.",
			runnerLabels,
			constLabels,
		),
		warmupTimedOutDesc: prometheus.NewDesc(
			"gitlab_runner_autoscaling_warmup_timed_out",
			"1 if warm-up ended before the pool reached the ready threshold, 0 otherwise.",
			runnerLabels,
			constLabels,
		),
		warmupReadyDesc: prometheus.NewDesc(
			"gitlab_runner_autoscaling_warmup_ready_seconds",
			"Seconds from the start of warm-up until the pool reached the ready threshold. Absent until it does.",
			runnerLabels,
			constLabels,
		),
	}

	if config.CreationPhaseMetricsEnabled() {
		metrics.creationPhaseHistogram = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "gitlab_runner_autoscaling_machine_creation_phase_duration_seconds",
				Help:        "Histogram of machine creation time for each Docker Machine phase.",
				Buckets:     config.GetCreationPhaseDurationSecondsBuckets(),
				ConstLabels: constLabels,
			},
			append([]string{"phase", "result"}, targetLabelNames...),
		)
	}

	return metrics
}

// collectors lists the vec-backed metrics; the *Desc ones are emitted by hand in Collect.
func (m *machineMetrics) collectors() []prometheus.Collector {
	cs := []prometheus.Collector{
		m.totalActions,
		m.creationHistogram,
		m.failedCreationHistogram,
		m.stoppingHistogram,
		m.removalHistogram,
	}
	if m.creationPhaseHistogram != nil {
		cs = append(cs, m.creationPhaseHistogram)
	}
	return cs
}

// Describe implements prometheus.Collector.
func (m *machineProvider) Describe(ch chan<- *prometheus.Desc) {
	for _, c := range m.collectors() {
		c.Describe(ch)
	}
	ch <- m.currentStatesDesc
	ch <- m.maxGrowthRateDesc
	ch <- m.idleTargetDesc
	ch <- m.warmupDesc
	ch <- m.warmupTimedOutDesc
	ch <- m.warmupReadyDesc
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

	for _, c := range m.collectors() {
		c.Collect(ch)
	}

	now := time.Now()
	m.autoscalingLimitsLock.Lock()
	for runner, limits := range m.autoscalingLimits {
		if now.Sub(limits.updatedAt) > autoscalingLimitsTTL {
			delete(m.autoscalingLimits, runner)
			continue
		}
		ch <- prometheus.MustNewConstMetric(
			m.maxGrowthRateDesc, prometheus.GaugeValue, limits.maxGrowthRate,
			runner, limits.runnerName, limits.systemID)
		ch <- prometheus.MustNewConstMetric(
			m.idleTargetDesc, prometheus.GaugeValue, limits.idleTarget,
			runner, limits.runnerName, limits.systemID)
		if !limits.warmupEnabled {
			continue
		}
		g := m.warmupGaugesFor(runner, now)
		ch <- prometheus.MustNewConstMetric(
			m.warmupDesc, prometheus.GaugeValue, boolToFloat(g.active),
			runner, limits.runnerName, limits.systemID)
		ch <- prometheus.MustNewConstMetric(
			m.warmupTimedOutDesc, prometheus.GaugeValue, boolToFloat(g.timedOut),
			runner, limits.runnerName, limits.systemID)
		if g.reached {
			ch <- prometheus.MustNewConstMetric(
				m.warmupReadyDesc, prometheus.GaugeValue, g.readyS,
				runner, limits.runnerName, limits.systemID)
		}
	}
	m.autoscalingLimitsLock.Unlock()
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

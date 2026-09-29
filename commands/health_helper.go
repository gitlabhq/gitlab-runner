package commands

import (
	"sync"
	"time"

	"github.com/jpillora/backoff"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type healthData struct {
	failures      int
	backoff       *backoff.Backoff
	disabledUntil time.Time
}

type healthHelper struct {
	healthy     map[string]*healthData
	healthyLock sync.Mutex

	healthCheckFailures *prometheus.CounterVec
	healthyGauge        *prometheus.GaugeVec

	now func() time.Time
}

func newHealthHelper() healthHelper {
	return healthHelper{
		now: time.Now,
		healthCheckFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_worker_health_check_failures_total",
				Help: "Total number of runner worker health check failures",
			},
			[]string{"runner", "runner_name", "system_id"},
		),
		healthyGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gitlab_runner_worker_healthy",
				Help: "1 while the runner worker is healthy, 0 from exceeding unhealthy_requests_limit until a job request succeeds",
			},
			[]string{"runner", "runner_name", "system_id"},
		),
	}
}

func (mr *healthHelper) getHealth(runner *common.RunnerConfig) *healthData {
	if mr.healthy == nil {
		mr.healthy = map[string]*healthData{}
	}

	id := runner.UniqueID()
	health := mr.healthy[id]
	if health == nil {
		health = &healthData{
			backoff: &backoff.Backoff{
				Min:    common.DefaultUnhealthyBackoffInitial,
				Max:    runner.GetUnhealthyInterval(),
				Factor: 2,
				Jitter: true,
			},
		}
		mr.healthy[id] = health
	}

	return health
}

func (mr *healthHelper) isHealthy(runner *common.RunnerConfig) bool {
	mr.healthyLock.Lock()
	defer mr.healthyLock.Unlock()

	mr.runnerHealthCheckFailures(runner).Add(0)

	id := runner.UniqueID()
	health := mr.getHealth(runner)
	limit := runner.GetUnhealthyRequestsLimit()
	if health.failures < limit || runner.GetUnhealthyInterval() <= 0 {
		mr.runnerHealthy(runner).Set(1)
		return true
	}

	if mr.now().Before(health.disabledUntil) {
		return false
	}

	logrus.WithFields(logrus.Fields{
		"unhealthy_requests":       health.failures,
		"unhealthy_requests_limit": limit,
		"unhealthy_attempt":        health.backoff.Attempt(),
	}).Warningf("Runner %q is not healthy, but check for a new job will be forced!", id)

	// one more failure re-disables with a longer backoff
	health.failures = limit - 1
	return true
}

func (mr *healthHelper) runnerHealthCheckFailures(runner *common.RunnerConfig) prometheus.Counter {
	return mr.healthCheckFailures.WithLabelValues(runner.ShortDescription(), runner.Name, runner.GetSystemID())
}

func (mr *healthHelper) runnerHealthy(runner *common.RunnerConfig) prometheus.Gauge {
	return mr.healthyGauge.WithLabelValues(runner.ShortDescription(), runner.Name, runner.GetSystemID())
}

func (mr *healthHelper) markHealth(runner *common.RunnerConfig, healthy bool) {
	mr.healthyLock.Lock()
	defer mr.healthyLock.Unlock()

	id := runner.UniqueID()

	health := mr.getHealth(runner)
	if healthy {
		health.failures = 0
		health.backoff.Reset()
		health.disabledUntil = time.Time{}
		return
	}

	mr.runnerHealthCheckFailures(runner).Inc()

	health.failures++
	limit := runner.GetUnhealthyRequestsLimit()
	if health.failures < limit || mr.now().Before(health.disabledUntil) {
		return
	}

	// unhealthy_interval = 0 opts out of disabling, as before the backoff
	if runner.GetUnhealthyInterval() <= 0 {
		return
	}

	disabledFor := health.backoff.Duration()
	health.disabledUntil = mr.now().Add(disabledFor)
	mr.runnerHealthy(runner).Set(0)

	logrus.WithFields(logrus.Fields{
		"unhealthy_requests":       health.failures,
		"unhealthy_requests_limit": limit,
		"unhealthy_backoff":        disabledFor,
		"unhealthy_interval":       runner.GetUnhealthyInterval(),
	}).Errorf("Runner %q is unhealthy and will be disabled for %s!", id, disabledFor)
}

func (mr *healthHelper) Describe(ch chan<- *prometheus.Desc) {
	mr.healthCheckFailures.Describe(ch)
	mr.healthyGauge.Describe(ch)
}

func (mr *healthHelper) Collect(ch chan<- prometheus.Metric) {
	mr.healthCheckFailures.Collect(ch)
	mr.healthyGauge.Collect(ch)
}

package prometheus

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var numJobFailuresDesc = prometheus.NewDesc(
	"gitlab_runner_failed_jobs_total",
	"Total number of failed jobs",
	[]string{"runner", "runner_name", "failure_reason"},
	nil,
)

type failurePermutation struct {
	runnerDescription string
	runnerName        string
	reason            common.JobFailureReason
}

type FailuresCollector struct {
	lock sync.RWMutex

	failures map[failurePermutation]int64
}

func (fc *FailuresCollector) RecordFailure(reason common.JobFailureReason, runnerConfig common.RunnerConfig) {
	failure := failurePermutation{
		runnerDescription: runnerConfig.ShortDescription(),
		runnerName:        runnerConfig.Name,
		reason:            reason,
	}

	fc.lock.Lock()
	defer fc.lock.Unlock()

	fc.failures[failure]++
}

func (fc *FailuresCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- numJobFailuresDesc
}

func (fc *FailuresCollector) Collect(ch chan<- prometheus.Metric) {
	fc.lock.RLock()
	defer fc.lock.RUnlock()

	for failure, number := range fc.failures {
		ch <- prometheus.MustNewConstMetric(
			numJobFailuresDesc,
			prometheus.CounterValue,
			float64(number),
			failure.runnerDescription,
			failure.runnerName,
			string(failure.reason),
		)
	}
}

func NewFailuresCollector() *FailuresCollector {
	return &FailuresCollector{
		failures: make(map[failurePermutation]int64),
	}
}

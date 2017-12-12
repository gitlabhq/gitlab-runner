package common

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var numBuildsDesc = prometheus.NewDesc(
	"ci_runner_builds",
	"The current number of running builds.",
	[]string{"runner", "state", "stage", "executor_stage"},
	nil,
)

var numBuildFailuresDesc = prometheus.NewDesc(
	"ci_runner_failed_jobs_total",
	"Total number of failed jobs",
	[]string{"runner", "state", "stage", "executor_stage", "failure_reason"},
	nil,
)

type statePermutation struct {
	runner        string
	buildState    BuildRuntimeState
	buildStage    BuildStage
	executorStage ExecutorStage
}

func newStatePermutationFromBuild(build *Build) statePermutation {
	return statePermutation{
		runner:        build.Runner.ShortDescription(),
		buildState:    build.CurrentState,
		buildStage:    build.CurrentStage,
		executorStage: build.CurrentExecutorStage(),
	}
}

type failurePermutation struct {
	statePermutation

	reason JobFailureReason
}

func newFailurePermutation(job *Build, reason JobFailureReason) failurePermutation {
	failurePerm := failurePermutation{
		reason: reason,
	}
	failurePerm.statePermutation = newStatePermutationFromBuild(job)

	return failurePerm
}

type runnerCounter struct {
	builds   int
	requests int
}

type BuildsHelper struct {
	counters      map[string]*runnerCounter
	builds        []*Build
	buildFailures map[failurePermutation]int
	lock          sync.Mutex
}

func (b *BuildsHelper) getRunnerCounter(runner *RunnerConfig) *runnerCounter {
	if b.counters == nil {
		b.counters = make(map[string]*runnerCounter)
	}

	counter, _ := b.counters[runner.Token]
	if counter == nil {
		counter = &runnerCounter{}
		b.counters[runner.Token] = counter
	}
	return counter
}

func (b *BuildsHelper) AcquireBuild(runner *RunnerConfig) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	counter := b.getRunnerCounter(runner)

	if runner.Limit > 0 && counter.builds >= runner.Limit {
		// Too many builds
		return false
	}

	counter.builds++
	return true
}

func (b *BuildsHelper) ReleaseBuild(runner *RunnerConfig) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	counter := b.getRunnerCounter(runner)
	if counter.builds > 0 {
		counter.builds--
		return true
	}
	return false
}

func (b *BuildsHelper) AcquireRequest(runner *RunnerConfig) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	counter := b.getRunnerCounter(runner)

	if counter.requests >= runner.GetRequestConcurrency() {
		return false
	}

	counter.requests++
	return true
}

func (b *BuildsHelper) ReleaseRequest(runner *RunnerConfig) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	counter := b.getRunnerCounter(runner)
	if counter.requests > 0 {
		counter.requests--
		return true
	}
	return false
}

func (b *BuildsHelper) AddBuild(build *Build) {
	b.lock.Lock()
	defer b.lock.Unlock()

	runners := make(map[int]bool)
	projectRunners := make(map[int]bool)

	for _, otherBuild := range b.builds {
		if otherBuild.Runner.Token != build.Runner.Token {
			continue
		}
		runners[otherBuild.RunnerID] = true

		if otherBuild.JobInfo.ProjectID != build.JobInfo.ProjectID {
			continue
		}
		projectRunners[otherBuild.ProjectRunnerID] = true
	}

	for {
		if !runners[build.RunnerID] {
			break
		}
		build.RunnerID++
	}

	for {
		if !projectRunners[build.ProjectRunnerID] {
			break
		}
		build.ProjectRunnerID++
	}

	b.builds = append(b.builds, build)
	return
}

func (b *BuildsHelper) RemoveBuild(deleteBuild *Build) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	for idx, build := range b.builds {
		if build == deleteBuild {
			b.builds = append(b.builds[0:idx], b.builds[idx+1:]...)
			return true
		}
	}
	return false
}

func (b *BuildsHelper) RecordFailure(job *Build, reason JobFailureReason) {
	b.lock.Lock()
	defer b.lock.Unlock()

	failure := newFailurePermutation(job, reason)

	if len(b.buildFailures) == 0 {
		b.buildFailures = make(map[failurePermutation]int)
	}

	if _, ok := b.buildFailures[failure]; ok {
		b.buildFailures[failure]++
	} else {
		b.buildFailures[failure] = 1
	}
}

func (b *BuildsHelper) BuildsCount() int {
	b.lock.Lock()
	defer b.lock.Unlock()

	return len(b.builds)
}

func (b *BuildsHelper) statesAndStages() map[statePermutation]int {
	b.lock.Lock()
	defer b.lock.Unlock()

	data := make(map[statePermutation]int)
	for _, build := range b.builds {
		state := newStatePermutationFromBuild(build)
		if _, ok := data[state]; ok {
			data[state]++
		} else {
			data[state] = 1
		}
	}
	return data
}

// Describe implements prometheus.Collector.
func (b *BuildsHelper) Describe(ch chan<- *prometheus.Desc) {
	ch <- numBuildsDesc
	ch <- numBuildFailuresDesc
}

// Collect implements prometheus.Collector.
func (b *BuildsHelper) Collect(ch chan<- prometheus.Metric) {
	data := b.statesAndStages()

	for state, count := range data {
		ch <- prometheus.MustNewConstMetric(
			numBuildsDesc,
			prometheus.GaugeValue,
			float64(count),
			state.runner,
			string(state.buildState),
			string(state.buildStage),
			string(state.executorStage),
		)
	}

	for failure, count := range b.buildFailures {
		ch <- prometheus.MustNewConstMetric(
			numBuildFailuresDesc,
			prometheus.CounterValue,
			float64(count),
			failure.runner,
			string(failure.buildState),
			string(failure.buildStage),
			string(failure.executorStage),
			string(failure.reason),
		)
	}
}

func (b *BuildsHelper) ListJobsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain")

	var jobs []string
	for _, job := range b.builds {
		jobDescription := fmt.Sprintf(
			"id=%d url=%s state=%s stage=%s executor_stage=%s",
			job.ID, job.RepoCleanURL(),
			job.CurrentState, job.CurrentStage, job.CurrentExecutorStage(),
		)
		jobs = append(jobs, jobDescription)
	}

	w.Write([]byte(strings.Join(jobs, "\n")))
}

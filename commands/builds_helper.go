package commands

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"gitlab.com/gitlab-org/gitlab-runner/common"

	"github.com/prometheus/client_golang/prometheus"
)

var numBuildsDesc = prometheus.NewDesc(
	"ci_runner_builds",
	"The current number of running builds.",
	[]string{"runner", "state", "stage", "executor_stage"},
	nil,
)

type statePermutation struct {
	runner        string
	buildState    common.BuildRuntimeState
	buildStage    common.BuildStage
	executorStage common.ExecutorStage
}

func newStatePermutationFromBuild(build *common.Build) statePermutation {
	return statePermutation{
		runner:        build.Runner.ShortDescription(),
		buildState:    build.CurrentState,
		buildStage:    build.CurrentStage,
		executorStage: build.CurrentExecutorStage(),
	}
}

type runnerCounter struct {
	builds   int
	requests int
}

type buildsHelper struct {
	counters map[string]*runnerCounter
	builds   []*common.Build
	lock     sync.Mutex
}

func (b *buildsHelper) getRunnerCounter(runner *common.RunnerConfig) *runnerCounter {
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

func (b *buildsHelper) acquireBuild(runner *common.RunnerConfig) bool {
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

func (b *buildsHelper) releaseBuild(runner *common.RunnerConfig) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	counter := b.getRunnerCounter(runner)
	if counter.builds > 0 {
		counter.builds--
		return true
	}
	return false
}

func (b *buildsHelper) acquireRequest(runner *common.RunnerConfig) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	counter := b.getRunnerCounter(runner)

	if counter.requests >= runner.GetRequestConcurrency() {
		return false
	}

	counter.requests++
	return true
}

func (b *buildsHelper) releaseRequest(runner *common.RunnerConfig) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	counter := b.getRunnerCounter(runner)
	if counter.requests > 0 {
		counter.requests--
		return true
	}
	return false
}

func (b *buildsHelper) addBuild(build *common.Build) {
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

func (b *buildsHelper) removeBuild(deleteBuild *common.Build) bool {
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

func (b *buildsHelper) buildsCount() int {
	b.lock.Lock()
	defer b.lock.Unlock()

	return len(b.builds)
}

func (b *buildsHelper) statesAndStages() map[statePermutation]int {
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
func (b *buildsHelper) Describe(ch chan<- *prometheus.Desc) {
	ch <- numBuildsDesc
}

// Collect implements prometheus.Collector.
func (b *buildsHelper) Collect(ch chan<- prometheus.Metric) {
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
}

func (b *buildsHelper) ListJobsHandler(w http.ResponseWriter, r *http.Request) {
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

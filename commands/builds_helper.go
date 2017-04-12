package commands

import (
	"sync"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"

	"github.com/prometheus/client_golang/prometheus"
)

var numBuildsDesc = prometheus.NewDesc("ci_runner_builds", "The current number of running builds.", []string{"state", "stage"}, nil)

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

		if otherBuild.ProjectID != build.ProjectID {
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

func (b *buildsHelper) statesAndStages() map[common.BuildRuntimeState]map[common.BuildStage]int {
	b.lock.Lock()
	defer b.lock.Unlock()

	data := make(map[common.BuildRuntimeState]map[common.BuildStage]int)
	for _, build := range b.builds {
		state := build.CurrentState
		stage := build.CurrentStage

		if data[state] == nil {
			data[state] = make(map[common.BuildStage]int)
		}
		data[state][stage]++
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

	for state, scripts := range data {
		for stage, count := range scripts {
			ch <- prometheus.MustNewConstMetric(numBuildsDesc, prometheus.GaugeValue, float64(count),
				string(state), string(stage))
		}
	}
}

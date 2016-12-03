package commands

import (
	"sync"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"

	"github.com/prometheus/client_golang/prometheus"
)

var numBuildsDesc = prometheus.NewDesc("ci_runner_builds", "The current number of running builds.", []string{"state", "stage"}, nil)

type buildsHelper struct {
	counts map[string]int
	builds []*common.Build
	lock   sync.Mutex
}

func (b *buildsHelper) acquire(runner *common.RunnerConfig) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	// Check number of builds
	count, _ := b.counts[runner.Token]
	if runner.Limit > 0 && count >= runner.Limit {
		// Too many builds
		return false
	}

	// Create a new build
	if b.counts == nil {
		b.counts = make(map[string]int)
	}
	b.counts[runner.Token]++
	return true
}

func (b *buildsHelper) release(runner *common.RunnerConfig) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	_, ok := b.counts[runner.Token]
	if ok {
		b.counts[runner.Token]--
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

func (b *buildsHelper) statesAndStages() map[common.BuildRuntimeState]map[common.ShellScriptStage]int {
	b.lock.Lock()
	defer b.lock.Unlock()

	data := make(map[common.BuildRuntimeState]map[common.ShellScriptStage]int)
	for _, build := range b.builds {
		if data[build.CurrentState] == nil {
			data[build.CurrentState] = make(map[common.ShellScriptStage]int)
		}
		data[build.CurrentState][build.CurrentStage]++
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
				"state="+string(state), "stage="+string(stage))
		}
	}
}

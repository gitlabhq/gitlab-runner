package commands

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/session"

	"github.com/prometheus/client_golang/prometheus"
)

var numBuildsDesc = prometheus.NewDesc(
	"gitlab_runner_jobs",
	"The current number of running builds.",
	[]string{"runner", "state", "stage", "executor_stage"},
	nil,
)

var requestConcurrencyDesc = prometheus.NewDesc(
	"gitlab_runner_request_concurrency",
	"The current number of concurrent requests for a new job",
	[]string{"runner"},
	nil,
)

var requestConcurrencyExceededDesc = prometheus.NewDesc(
	"gitlab_runner_request_concurrency_exceeded_total",
	"Counter tracking exceeding of request concurrency",
	[]string{"runner"},
	nil,
)

type statePermutation struct {
	runner        string
	buildState    common.BuildRuntimeState
	buildStage    common.BuildStage
	executorStage common.ExecutorStage
}

func newStatePermutationFromBuild(build *common.Build) statePermutation {
	state := build.CurrentState()

	// the "finished" state was broken out into "success" and "failed",
	// but our metrics are a public API, so to maintain backwards compatibility
	// for now we convert these states back to "finished".
	//
	// DEPRECATED
	// TODO: Remove in 14.0. For more details read
	// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26900
	switch state {
	case common.BuildRunRuntimeFailed, common.BuildRunRuntimeCanceled:
		state = common.BuildRuntimeState("finished")
	}

	return statePermutation{
		runner:        build.Runner.ShortDescription(),
		buildState:    state,
		buildStage:    build.CurrentStage(),
		executorStage: build.CurrentExecutorStage(),
	}
}

type runnerCounter struct {
	builds   int
	requests int

	requestConcurrencyExceeded int
}

type buildsHelper struct {
	counters map[string]*runnerCounter
	builds   []*common.Build
	lock     sync.Mutex

	jobsTotal            *prometheus.CounterVec
	jobDurationHistogram *prometheus.HistogramVec
}

func (b *buildsHelper) getRunnerCounter(runner *common.RunnerConfig) *runnerCounter {
	if b.counters == nil {
		b.counters = make(map[string]*runnerCounter)
	}

	counter := b.counters[runner.Token]
	if counter == nil {
		counter = &runnerCounter{}
		b.counters[runner.Token] = counter
	}
	return counter
}

func (b *buildsHelper) findSessionByURL(url string) *session.Session {
	b.lock.Lock()
	defer b.lock.Unlock()

	for _, build := range b.builds {
		if strings.HasPrefix(url, build.Session.Endpoint+"/") {
			return build.Session
		}
	}

	return nil
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
		counter.requestConcurrencyExceeded++

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
	if build == nil {
		return
	}

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
	b.jobsTotal.WithLabelValues(build.Runner.ShortDescription()).Inc()
}

func (b *buildsHelper) removeBuild(deleteBuild *common.Build) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.jobDurationHistogram.
		WithLabelValues(deleteBuild.Runner.ShortDescription()).
		Observe(deleteBuild.Duration().Seconds())

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
		data[state]++
	}
	return data
}

func (b *buildsHelper) runnersCounters() map[string]*runnerCounter {
	b.lock.Lock()
	defer b.lock.Unlock()

	data := make(map[string]*runnerCounter)
	for token, counter := range b.counters {
		data[helpers.ShortenToken(token)] = counter
	}

	return data
}

// Describe implements prometheus.Collector.
func (b *buildsHelper) Describe(ch chan<- *prometheus.Desc) {
	ch <- numBuildsDesc
	ch <- requestConcurrencyDesc
	ch <- requestConcurrencyExceededDesc

	b.jobsTotal.Describe(ch)
	b.jobDurationHistogram.Describe(ch)
}

// Collect implements prometheus.Collector.
func (b *buildsHelper) Collect(ch chan<- prometheus.Metric) {
	builds := b.statesAndStages()
	for state, count := range builds {
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

	counters := b.runnersCounters()
	for runner, counter := range counters {
		ch <- prometheus.MustNewConstMetric(
			requestConcurrencyDesc,
			prometheus.GaugeValue,
			float64(counter.requests),
			runner,
		)

		ch <- prometheus.MustNewConstMetric(
			requestConcurrencyExceededDesc,
			prometheus.CounterValue,
			float64(counter.requestConcurrencyExceeded),
			runner,
		)
	}

	b.jobsTotal.Collect(ch)
	b.jobDurationHistogram.Collect(ch)
}

func (b *buildsHelper) ListJobsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("X-List-Version", "2")
	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	for _, job := range b.builds {
		url := CreateJobURL(job.RepoCleanURL(), job.ID)

		_, _ = fmt.Fprintf(
			w,
			"url=%s state=%s stage=%s executor_stage=%s duration=%s\n",
			url,
			job.CurrentState(),
			job.CurrentStage(),
			job.CurrentExecutorStage(),
			job.Duration(),
		)
	}
}

func CreateJobURL(projectURL string, jobID int) string {
	r := regexp.MustCompile(`(\.git$)?`)
	URL := r.ReplaceAllString(projectURL, "")

	return fmt.Sprintf("%s/-/jobs/%d", URL, jobID)
}

func newBuildsHelper() buildsHelper {
	return buildsHelper{
		jobsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_jobs_total",
				Help: "Total number of handled jobs",
			},
			[]string{"runner"},
		),
		jobDurationHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gitlab_runner_job_duration_seconds",
				Help:    "Histogram of job durations",
				Buckets: []float64{30, 60, 300, 600, 1800, 3600, 7200, 10800, 18000, 36000},
			},
			[]string{"runner"},
		),
	}
}

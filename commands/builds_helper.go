package commands

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/session"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	concurrencyIncreaseFactor = 1.1  // +10%
	concurrencyDecreaseFactor = 0.95 // -5%
)

var numBuildsDesc = prometheus.NewDesc(
	"gitlab_runner_jobs",
	"The current number of running builds.",
	[]string{"runner", "runner_name", "system_id", "state", "stage", "executor_stage"},
	nil,
)

var requestConcurrencyDesc = prometheus.NewDesc(
	"gitlab_runner_request_concurrency",
	"The current number of concurrent requests for a new job",
	[]string{"runner", "system_id"},
	nil,
)

var requestConcurrencyExceededDesc = prometheus.NewDesc(
	"gitlab_runner_request_concurrency_exceeded_total",
	"Count of excess requests above the configured request_concurrency limit",
	[]string{"runner", "system_id"},
	nil,
)

var requestConcurrencyHardLimitDesc = prometheus.NewDesc(
	"gitlab_runner_request_concurrency_hard_limit",
	"Configured request_concurrency limit",
	[]string{"runner", "system_id"},
	nil,
)

var requestConcurrencyAdaptiveLimitDesc = prometheus.NewDesc(
	"gitlab_runner_request_concurrency_adaptive_limit",
	"Computed adaptive request concurrency limit",
	[]string{"runner", "system_id"},
	nil,
)

var requestConcurrencyUsedLimitDesc = prometheus.NewDesc(
	"gitlab_runner_request_concurrency_used_limit",
	"Used request concurrency limit",
	[]string{"runner", "system_id"},
	nil,
)

type statePermutation struct {
	runner        string
	runnerName    string
	systemID      string
	buildState    common.BuildRuntimeState
	buildStage    common.BuildStage
	executorStage common.ExecutorStage
}

func newStatePermutationFromBuild(build *common.Build) statePermutation {
	return statePermutation{
		runner:        build.Runner.ShortDescription(),
		runnerName:    build.Runner.Name,
		systemID:      build.Runner.GetSystemID(),
		buildState:    build.CurrentState(),
		buildStage:    build.CurrentStage(),
		executorStage: build.CurrentExecutorStage(),
	}
}

type runnerCounter struct {
	systemID   string
	runnerName string

	builds   int
	requests int

	hardConcurrencyLimit       int
	adaptiveConcurrencyLimit   float64
	usedConcurrencyLimit       int
	requestConcurrencyExceeded int
}

type buildsHelper struct {
	counters              map[string]*runnerCounter
	buildStagesStartTimes map[*common.Build]map[common.BuildStage]time.Time
	builds                []*common.Build
	lock                  sync.Mutex

	jobsTotal                  *prometheus.CounterVec
	jobDurationHistogram       *prometheus.HistogramVec
	jobStagesDurationHistogram *prometheus.HistogramVec
	jobQueueDurationHistogram  *prometheus.HistogramVec
	jobQueueSize               *prometheus.GaugeVec
	jobQueueDepth              *prometheus.GaugeVec

	acceptableJobQueuingDurationExceeded *prometheus.CounterVec
}

func (b *buildsHelper) getRunnerCounter(runner *common.RunnerConfig) *runnerCounter {
	if b.counters == nil {
		b.counters = make(map[string]*runnerCounter)
	}

	counter := b.counters[runner.Token]
	if counter == nil {
		counter = &runnerCounter{systemID: runner.GetSystemID(), runnerName: runner.Name}
		b.counters[runner.Token] = counter
	}
	return counter
}

func (b *buildsHelper) findSessionByURL(url string) (*session.Session, error) {
	if url == "" {
		return nil, fmt.Errorf("empty URL provided")
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	if len(b.builds) == 0 {
		return nil, fmt.Errorf("no active builds found")
	}
	for _, build := range b.builds {
		if build.Session == nil {
			continue
		}

		if build.Session.Endpoint == "" {
			continue
		}
		if strings.HasPrefix(url, build.Session.Endpoint+"/") {
			return build.Session, nil
		}
	}

	return nil, fmt.Errorf("no session found matching URL: %s", url)
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

	concurrency := runner.GetRequestConcurrency()
	counter.hardConcurrencyLimit = concurrency

	if runner.IsFeatureFlagOn(featureflags.UseAdaptiveRequestConcurrency) {
		// concurrency is the adaptive concurrency value rounded up, between 1 and the max request concurrency
		concurrency = min(max(1, int(math.Ceil(counter.adaptiveConcurrencyLimit))), runner.GetRequestConcurrency())
	}

	counter.usedConcurrencyLimit = concurrency
	if counter.requests >= concurrency {
		counter.requestConcurrencyExceeded++

		return false
	}

	counter.requests++
	return true
}

func (b *buildsHelper) releaseRequest(runner *common.RunnerConfig, hasJob bool) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	counter := b.getRunnerCounter(runner)

	if runner.IsFeatureFlagOn(featureflags.UseAdaptiveRequestConcurrency) {
		// if the request returned a job, increase the concurrency by 10%, if not, decrease by 5%
		if hasJob {
			counter.adaptiveConcurrencyLimit *= concurrencyIncreaseFactor
		} else {
			counter.adaptiveConcurrencyLimit *= concurrencyDecreaseFactor
		}
		// adjust adaptive concurrency between 1 and max request concurrency
		counter.adaptiveConcurrencyLimit = min(max(1, counter.adaptiveConcurrencyLimit), float64(runner.GetRequestConcurrency()))
	}

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

	for runners[build.RunnerID] {
		build.RunnerID++
	}

	for projectRunners[build.ProjectRunnerID] {
		build.ProjectRunnerID++
	}

	b.builds = append(b.builds, build)
	b.jobsTotal.WithLabelValues(build.Runner.ShortDescription(), build.Runner.Name, build.Runner.GetSystemID()).Inc()
	b.jobQueueDurationHistogram.
		WithLabelValues(
			build.Runner.ShortDescription(),
			build.Runner.Name,
			build.Runner.GetSystemID(),
			build.JobInfo.ProjectJobsRunningOnInstanceRunnersCount,
		).
		Observe(build.JobInfo.TimeInQueueSeconds)
	b.jobQueueSize.
		WithLabelValues(
			build.Runner.ShortDescription(),
			build.Runner.Name,
			build.Runner.GetSystemID(),
		).
		Set(float64(build.JobInfo.QueueSize))
	b.jobQueueDepth.
		WithLabelValues(
			build.Runner.ShortDescription(),
			build.Runner.Name,
			build.Runner.GetSystemID(),
		).
		Set(float64(build.JobInfo.QueueDepth))

	b.evaluateJobQueuingDuration(build.Runner, build.JobInfo)
	b.initializeBuildStageMetrics(build)
}

func (b *buildsHelper) evaluateJobQueuingDuration(runner *common.RunnerConfig, jobInfo common.JobInfo) {
	counterForRunner := b.acceptableJobQueuingDurationExceeded.
		WithLabelValues(
			runner.ShortDescription(),
			runner.Name,
			runner.GetSystemID(),
		)

	// This .Add(0) will not change the value of the metric when threshold was
	// not exceeded, but will make sure that the metric for each runner is always
	// available
	counterForRunner.Add(0)

	// If configuration is not present we don't care about the metric
	if runner.Monitoring == nil || len(runner.Monitoring.JobQueuingDurations) < 1 {
		return
	}

	jobQueueDurationCfg := runner.Monitoring.JobQueuingDurations.GetActiveConfiguration()

	// If no configuration matches current time we don't care about the metric
	if jobQueueDurationCfg == nil {
		return
	}

	threshold := jobQueueDurationCfg.Threshold.Seconds()

	// Threshold not configured, zeroed or invalid (negative) means we're not interested in this feature
	if threshold <= 0 {
		return
	}

	// If threshold is not exceeded, then all is good and there is no need for other checks
	if jobInfo.TimeInQueueSeconds <= threshold {
		return
	}

	// If JobProjectsRunningOnInstanceRunnersCount doesn't match the definition it means that exceeded
	// threshold is acceptable in such case.
	// If the definition was not configured (or the regular expression in the config.toml file was invalid
	// and couldn't be compiled) we treat that as "matched" and count the case in
	if !jobQueueDurationCfg.JobsRunningForProjectMatched(jobInfo.ProjectJobsRunningOnInstanceRunnersCount) {
		return
	}

	// Timing expectation not met for this case. Let's increase the counter
	counterForRunner.Inc()
}

func (b *buildsHelper) removeBuild(deleteBuild *common.Build) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.jobDurationHistogram.
		WithLabelValues(deleteBuild.Runner.ShortDescription(), deleteBuild.Runner.Name, deleteBuild.Runner.GetSystemID()).
		Observe(deleteBuild.FinalDuration().Seconds())

	for idx, build := range b.builds {
		if build == deleteBuild {
			b.builds = append(b.builds[0:idx], b.builds[idx+1:]...)
			delete(b.buildStagesStartTimes, deleteBuild)

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

	for token, counter := range b.counters {
		// 'idle' state will ensure the metric is always present, even if no
		// builds are being processed at the moment
		idleState := statePermutation{
			runner:        helpers.ShortenToken(token),
			runnerName:    counter.runnerName,
			systemID:      counter.systemID,
			buildState:    "idle",
			buildStage:    "idle",
			executorStage: "idle",
		}
		data[idleState] = 0
	}

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

func (b *buildsHelper) initializeBuildStageMetrics(build *common.Build) {
	if !build.IsFeatureFlagOn(featureflags.ExportHighCardinalityMetrics) {
		return
	}

	// the receiver lock is held at this point
	if b.buildStagesStartTimes == nil {
		b.buildStagesStartTimes = make(map[*common.Build]map[common.BuildStage]time.Time)
	}

	if b.buildStagesStartTimes[build] == nil {
		b.buildStagesStartTimes[build] = make(map[common.BuildStage]time.Time)
	}

	build.OnBuildStageStartFn = func(stage common.BuildStage) {
		b.handleOnBuildStageStart(build, stage)
	}

	build.OnBuildStageEndFn = func(stage common.BuildStage) {
		b.handleOnBuildStageEnd(build, stage)
	}
}

func (b *buildsHelper) handleOnBuildStageStart(build *common.Build, stage common.BuildStage) {
	b.lock.Lock()
	b.buildStagesStartTimes[build][stage] = time.Now()
	b.lock.Unlock()
}

func (b *buildsHelper) handleOnBuildStageEnd(build *common.Build, stage common.BuildStage) {
	b.lock.Lock()
	duration := time.Since(b.buildStagesStartTimes[build][stage])
	b.lock.Unlock()

	b.jobStagesDurationHistogram.
		With(prometheus.Labels{
			"runner":      build.Runner.ShortDescription(),
			"runner_name": build.Runner.Name,
			"system_id":   build.Runner.GetSystemID(),
			"stage":       string(stage),
		}).
		Observe(duration.Seconds())
}

// Describe implements prometheus.Collector.
func (b *buildsHelper) Describe(ch chan<- *prometheus.Desc) {
	ch <- numBuildsDesc
	ch <- requestConcurrencyDesc
	ch <- requestConcurrencyExceededDesc
	ch <- requestConcurrencyHardLimitDesc
	ch <- requestConcurrencyAdaptiveLimitDesc
	ch <- requestConcurrencyUsedLimitDesc

	b.jobsTotal.Describe(ch)
	b.jobDurationHistogram.Describe(ch)
	b.jobQueueDurationHistogram.Describe(ch)
	b.jobQueueSize.Describe(ch)
	b.jobQueueDepth.Describe(ch)
	b.acceptableJobQueuingDurationExceeded.Describe(ch)
	b.jobStagesDurationHistogram.Describe(ch)
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
			state.runnerName,
			state.systemID,
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
			counter.systemID,
		)

		ch <- prometheus.MustNewConstMetric(
			requestConcurrencyExceededDesc,
			prometheus.CounterValue,
			float64(counter.requestConcurrencyExceeded),
			runner,
			counter.systemID,
		)

		ch <- prometheus.MustNewConstMetric(
			requestConcurrencyHardLimitDesc,
			prometheus.GaugeValue,
			float64(counter.hardConcurrencyLimit),
			runner,
			counter.systemID,
		)

		ch <- prometheus.MustNewConstMetric(
			requestConcurrencyAdaptiveLimitDesc,
			prometheus.GaugeValue,
			counter.adaptiveConcurrencyLimit,
			runner,
			counter.systemID,
		)

		ch <- prometheus.MustNewConstMetric(
			requestConcurrencyUsedLimitDesc,
			prometheus.GaugeValue,
			float64(counter.usedConcurrencyLimit),
			runner,
			counter.systemID,
		)
	}

	b.jobsTotal.Collect(ch)
	b.jobDurationHistogram.Collect(ch)
	b.jobQueueDurationHistogram.Collect(ch)
	b.jobQueueSize.Collect(ch)
	b.jobQueueDepth.Collect(ch)
	b.acceptableJobQueuingDurationExceeded.Collect(ch)
	b.jobStagesDurationHistogram.Collect(ch)
}

func (b *buildsHelper) ListJobsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("X-List-Version", "2")
	w.Header().Add(common.ContentType, "text/plain")
	w.WriteHeader(http.StatusOK)

	b.lock.Lock()
	defer b.lock.Unlock()

	for _, job := range b.builds {
		_, _ = fmt.Fprintf(
			w,
			"url=%s state=%s stage=%s executor_stage=%s duration=%s\n",
			job.JobURL(),
			job.CurrentState(),
			job.CurrentStage(),
			job.CurrentExecutorStage(),
			job.CurrentDuration(),
		)
	}
}

func newBuildsHelper() buildsHelper {
	return buildsHelper{
		jobsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_jobs_total",
				Help: "Total number of handled jobs",
			},
			[]string{"runner", "runner_name", "system_id"},
		),
		jobDurationHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gitlab_runner_job_duration_seconds",
				Help:    "Histogram of job durations",
				Buckets: []float64{30, 60, 300, 600, 1800, 3600, 7200, 10800, 18000, 36000},
			},
			[]string{"runner", "runner_name", "system_id"},
		),
		jobQueueDurationHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gitlab_runner_job_queue_duration_seconds",
				Help:    "A histogram representing job queue duration.",
				Buckets: []float64{1, 3, 10, 30, 60, 120, 300, 900, 1800, 3600},
			},
			[]string{"runner", "runner_name", "system_id", "project_jobs_running"},
		),
		jobQueueSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gitlab_runner_job_queue_size",
				Help: "A gauge representing the size of the queue for the runner",
			},
			[]string{"runner", "runner_name", "system_id"},
		),
		jobQueueDepth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gitlab_runner_job_queue_depth",
				Help: "A gauge representing the search depth in the queue for the runner",
			},
			[]string{"runner", "runner_name", "system_id"},
		),
		acceptableJobQueuingDurationExceeded: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_acceptable_job_queuing_duration_exceeded_total",
				Help: "Counts how often jobs exceed the configured queuing time threshold",
			},
			[]string{"runner", "runner_name", "system_id"},
		),
		jobStagesDurationHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gitlab_runner_job_stage_duration_seconds",
				Help:    "Histogram of each job stage duration",
				Buckets: []float64{1, 3, 10, 30, 60, 120, 300, 900, 1800, 3600},
			},
			[]string{"runner", "runner_name", "system_id", "stage"},
		),
	}
}

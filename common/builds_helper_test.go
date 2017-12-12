package common

import (
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fakeRunner = &RunnerConfig{
	RunnerCredentials: RunnerCredentials{
		Token: "a1b2c3d4e5f6",
	},
}

func TestBuildsHelperCollect(t *testing.T) {
	b := &BuildsHelper{}
	build := &Build{
		CurrentState: BuildRunStatePending,
		CurrentStage: BuildStagePrepare,
		Runner:       fakeRunner,
	}

	b.AddBuild(build)
	b.RecordFailure(build, RunnerSystemFailure)

	ch := make(chan prometheus.Metric, 50)
	b.Collect(ch)

	assert.Len(t, ch, 2)

	buildState := <- ch
	buildFailure := <- ch

	buildStateMetric := &dto.Metric{}
	buildState.Write(buildStateMetric)
	assert.Equal(t, float64(1), *buildStateMetric.Gauge.Value)
	require.Len(t, buildStateMetric.Label, 4)

	stateMetricLabels := make(map[string]string)
	for _, l := range buildStateMetric.Label {
		stateMetricLabels[*l.Name] = *l.Value
	}

	assert.Equal(t, string(BuildRunStatePending), stateMetricLabels["state"])
	assert.Equal(t, string(BuildStagePrepare), stateMetricLabels["stage"])
	assert.Equal(t, string(fakeRunner.ShortDescription()), stateMetricLabels["runner"])
	assert.Equal(t, "", stateMetricLabels["executor_stage"])

	buildFailureMetric := &dto.Metric{}
	buildFailure.Write(buildFailureMetric)
	assert.Equal(t, float64(1), *buildFailureMetric.Counter.Value)
	require.Len(t, buildFailureMetric.Label, 5)

	failureMetricLabels := make(map[string]string)
	for _, l := range buildFailureMetric.Label {
		failureMetricLabels[*l.Name] = *l.Value
	}

	assert.Equal(t, string(RunnerSystemFailure), failureMetricLabels["failure_reason"])
	assert.Equal(t, string(BuildRunStatePending), failureMetricLabels["state"])
	assert.Equal(t, string(BuildStagePrepare), failureMetricLabels["stage"])
	assert.Equal(t, string(fakeRunner.ShortDescription()), failureMetricLabels["runner"])
	assert.Equal(t, "", failureMetricLabels["executor_stage"])
}

func TestBuildsHelperAcquireRequestWithLimit(t *testing.T) {
	runner := RunnerConfig{
		RequestConcurrency: 2,
	}

	b := &BuildsHelper{}
	result := b.AcquireRequest(&runner)
	require.True(t, result)

	result = b.AcquireRequest(&runner)
	require.True(t, result)

	result = b.AcquireRequest(&runner)
	require.False(t, result, "allow only two requests")

	result = b.ReleaseRequest(&runner)
	require.True(t, result)

	result = b.ReleaseRequest(&runner)
	require.True(t, result)

	result = b.ReleaseRequest(&runner)
	require.False(t, result, "release only two requests")
}

func TestBuildsHelperAcquireRequestWithDefault(t *testing.T) {
	runner := RunnerConfig{
		RequestConcurrency: 0,
	}

	b := &BuildsHelper{}
	result := b.AcquireRequest(&runner)
	require.True(t, result)

	result = b.AcquireRequest(&runner)
	require.False(t, result, "allow only one request")

	result = b.ReleaseRequest(&runner)
	require.True(t, result)

	result = b.ReleaseRequest(&runner)
	require.False(t, result, "release only one request")

	result = b.AcquireRequest(&runner)
	require.True(t, result)

	result = b.ReleaseRequest(&runner)
	require.True(t, result)

	result = b.ReleaseRequest(&runner)
	require.False(t, result, "nothing to release")
}

func TestBuildsHelperAcquireBuildWithLimit(t *testing.T) {
	runner := RunnerConfig{
		Limit: 1,
	}

	b := &BuildsHelper{}
	result := b.AcquireBuild(&runner)
	require.True(t, result)

	result = b.AcquireBuild(&runner)
	require.False(t, result, "allow only one build")

	result = b.ReleaseBuild(&runner)
	require.True(t, result)

	result = b.ReleaseBuild(&runner)
	require.False(t, result, "release only one build")
}

func TestBuildsHelperAcquireBuildUnlimited(t *testing.T) {
	runner := RunnerConfig{
		Limit: 0,
	}

	b := &BuildsHelper{}
	result := b.AcquireBuild(&runner)
	require.True(t, result)

	result = b.AcquireBuild(&runner)
	require.True(t, result)

	result = b.ReleaseBuild(&runner)
	require.True(t, result)

	result = b.ReleaseBuild(&runner)
	require.True(t, result)
}

var testBuildCurrentID int

func getTestBuild() *Build {
	testBuildCurrentID++

	runner := RunnerConfig{}
	runner.Token = "a1b2c3d4"
	jobInfo := JobInfo{
		ProjectID: 1,
	}

	build := &Build{}
	build.ID = testBuildCurrentID
	build.Runner = &runner
	build.JobInfo = jobInfo

	return build
}

func concurrentlyCallStatesAndStages(b *BuildsHelper, finished chan struct{}, wg *sync.WaitGroup) {
	for {
		select {
		case <-finished:
			wg.Done()
			return
		case <-time.After(1 * time.Millisecond):
			b.statesAndStages()
		}
	}
}

func handleTestBuild(wg *sync.WaitGroup, b *BuildsHelper, build *Build) {
	stages := []BuildStage{
		BuildStagePrepare,
		BuildStageGetSources,
		BuildStageRestoreCache,
		BuildStageDownloadArtifacts,
		BuildStageUserScript,
		BuildStageAfterScript,
		BuildStageArchiveCache,
		BuildStageUploadArtifacts,
	}
	states := []BuildRuntimeState{
		BuildRunStatePending,
		BuildRunRuntimeRunning,
		BuildRunRuntimeFinished,
	}

	b.AddBuild(build)
	time.Sleep(10 * time.Millisecond)
	for _, stage := range stages {
		for _, state := range states {
			build.CurrentStage = stage
			build.CurrentState = state
			time.Sleep(5 * time.Millisecond)
		}
	}
	time.Sleep(5 * time.Millisecond)
	b.RemoveBuild(build)

	time.Sleep(5 * time.Millisecond)
	wg.Done()
}

func TestBuildHelperCollectWhenRemovingBuild(t *testing.T) {
	t.Log("This test tries to simulate concurrency. It can be false-positive! But failure is always a sign that something is wrong.")
	b := &BuildsHelper{}
	statesAndStagesCallConcurrency := 10
	finished := make(chan struct{})

	wg1 := &sync.WaitGroup{}
	wg1.Add(statesAndStagesCallConcurrency)
	for i := 0; i < statesAndStagesCallConcurrency; i++ {
		go concurrentlyCallStatesAndStages(b, finished, wg1)
	}

	var builds []*Build
	for i := 1; i < 30; i++ {
		builds = append(builds, getTestBuild())
	}

	wg2 := &sync.WaitGroup{}
	wg2.Add(len(builds))
	for _, build := range builds {
		go handleTestBuild(wg2, b, build)
	}
	wg2.Wait()

	close(finished)
	wg1.Wait()
}

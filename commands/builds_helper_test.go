//go:build !integration

package commands

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/config/runner"
	"gitlab.com/gitlab-org/gitlab-runner/common/config/runner/monitoring"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/session"
)

const (
	testToken = "testoken" // No typo here! 8 characters to make it equal to the computed ShortDescription()
	testName  = "qwerty123"
)

func TestBuildsHelperAcquireRequestWithLimit(t *testing.T) {
	runner := common.RunnerConfig{
		RequestConcurrency: 2,
	}

	b := newBuildsHelper()
	result := b.acquireRequest(&runner)
	require.True(t, result)

	result = b.acquireRequest(&runner)
	require.False(t, result, "allow only one requests (adaptive limit)")

	result = b.releaseRequest(&runner, false, false)
	require.True(t, result)

	result = b.releaseRequest(&runner, false, false)
	require.False(t, result, "release only two requests")
}

func TestBuildsHelperAcquireRequestWithAdaptiveLimit(t *testing.T) {
	runner := common.RunnerConfig{
		RequestConcurrency: 2,
	}

	b := newBuildsHelper()
	result := b.acquireRequest(&runner)
	require.True(t, result)

	result = b.releaseRequest(&runner, true, false)
	require.True(t, result)

	result = b.acquireRequest(&runner)
	require.True(t, result)

	result = b.acquireRequest(&runner)
	require.False(t, result, "allow only two requests")

	result = b.releaseRequest(&runner, false, false)
	require.True(t, result)

	result = b.releaseRequest(&runner, false, false)
	require.False(t, result, "release only two requests")
}

func TestBuildsHelperAcquireRequestWithDefault(t *testing.T) {
	runner := common.RunnerConfig{
		RequestConcurrency: 0,
	}

	b := newBuildsHelper()
	result := b.acquireRequest(&runner)
	require.True(t, result)

	result = b.acquireRequest(&runner)
	require.False(t, result, "allow only one request")

	result = b.releaseRequest(&runner, false, false)
	require.True(t, result)

	result = b.releaseRequest(&runner, false, false)
	require.False(t, result, "release only one request")

	result = b.acquireRequest(&runner)
	require.True(t, result)

	result = b.releaseRequest(&runner, false, false)
	require.True(t, result)

	result = b.releaseRequest(&runner, false, false)
	require.False(t, result, "nothing to release")
}

func TestBuildsHelper_ReleaseRequest_AdaptiveGrowsOnCleanSuccess(t *testing.T) {
	runner := common.RunnerConfig{
		RequestConcurrency: 10,
		RunnerSettings: common.RunnerSettings{
			FeatureFlags: map[string]bool{featureflags.UseAdaptiveRequestConcurrency: true},
		},
	}

	b := newBuildsHelper()
	// 1.1^n reaches 10 at n≈25; 50 iterations is comfortably past the clamp.
	for i := 0; i < 50; i++ {
		require.True(t, b.acquireRequest(&runner))
		b.releaseRequest(&runner, true, false)
	}

	counter := b.counters[runner.Token]
	require.NotNil(t, counter)
	assert.InDelta(t, 10.0, counter.adaptiveConcurrencyLimit, 0.01,
		"clean first-try successes must grow adaptive limit up to the configured max")
}

func TestBuildsHelper_ReleaseRequest_RetriedDoesNotGrowAdaptive(t *testing.T) {
	runner := common.RunnerConfig{
		RequestConcurrency: 10,
		RunnerSettings: common.RunnerSettings{
			FeatureFlags: map[string]bool{featureflags.UseAdaptiveRequestConcurrency: true},
		},
	}

	b := newBuildsHelper()
	for i := 0; i < 20; i++ {
		require.True(t, b.acquireRequest(&runner))
		b.releaseRequest(&runner, true, true)
	}

	counter := b.counters[runner.Token]
	require.NotNil(t, counter)
	assert.InDelta(t, 1.0, counter.adaptiveConcurrencyLimit, 0.01,
		"retried requests must be treated as non-capacity signals and keep adaptive clamped at 1")
}

func TestBuildsHelper_ReleaseRequest_RetriedAfterWarmupShrinksAdaptiveFast(t *testing.T) {
	runner := common.RunnerConfig{
		RequestConcurrency: 20,
		RunnerSettings: common.RunnerSettings{
			FeatureFlags: map[string]bool{featureflags.UseAdaptiveRequestConcurrency: true},
		},
	}

	b := newBuildsHelper()
	for i := 0; i < 100; i++ {
		require.True(t, b.acquireRequest(&runner))
		b.releaseRequest(&runner, true, false)
	}
	counter := b.counters[runner.Token]
	require.NotNil(t, counter)
	require.InDelta(t, 20.0, counter.adaptiveConcurrencyLimit, 0.01, "warmup should reach max")

	for i := 0; i < 6; i++ {
		require.True(t, b.acquireRequest(&runner))
		b.releaseRequest(&runner, true, true)
	}

	assert.InDelta(t, 1.0, counter.adaptiveConcurrencyLimit, 0.01,
		"multiplicative decrease must collapse the limit to the floor in a handful of retried requests")
}

func TestBuildsHelper_ReleaseRequest_RetriedDecreasesFasterThanEmptyResponse(t *testing.T) {
	makeRunner := func() *common.RunnerConfig {
		return &common.RunnerConfig{
			RequestConcurrency: 20,
			RunnerSettings: common.RunnerSettings{
				FeatureFlags: map[string]bool{featureflags.UseAdaptiveRequestConcurrency: true},
			},
		}
	}

	warmTo20 := func(b *buildsHelper, runner *common.RunnerConfig) {
		for i := 0; i < 100; i++ {
			require.True(t, b.acquireRequest(runner))
			b.releaseRequest(runner, true, false)
		}
		require.InDelta(t, 20.0, b.counters[runner.Token].adaptiveConcurrencyLimit, 0.01)
	}

	bRetried := newBuildsHelper()
	rRetried := makeRunner()
	warmTo20(&bRetried, rRetried)

	bEmpty := newBuildsHelper()
	rEmpty := makeRunner()
	warmTo20(&bEmpty, rEmpty)

	for i := 0; i < 3; i++ {
		require.True(t, bRetried.acquireRequest(rRetried))
		bRetried.releaseRequest(rRetried, true, true)

		require.True(t, bEmpty.acquireRequest(rEmpty))
		bEmpty.releaseRequest(rEmpty, false, false)
	}

	assert.Less(t, bRetried.counters[rRetried.Token].adaptiveConcurrencyLimit,
		bEmpty.counters[rEmpty.Token].adaptiveConcurrencyLimit,
		"retried must shrink adaptive faster than passive empty responses")
}

func TestBuildsHelper_ReleaseRequest_RetriedIgnoredWhenAdaptiveDisabled(t *testing.T) {
	runner := common.RunnerConfig{
		RequestConcurrency: 5,
		RunnerSettings: common.RunnerSettings{
			FeatureFlags: map[string]bool{featureflags.UseAdaptiveRequestConcurrency: false},
		},
	}

	b := newBuildsHelper()
	require.True(t, b.acquireRequest(&runner))
	b.releaseRequest(&runner, true, true)

	counter := b.counters[runner.Token]
	require.NotNil(t, counter)
	assert.Equal(t, 0.0, counter.adaptiveConcurrencyLimit,
		"adaptive limit must remain untouched when the feature flag is off")
}

func TestBuildsHelperAcquireBuildWithLimit(t *testing.T) {
	runner := common.RunnerConfig{
		Limit: 1,
	}

	b := newBuildsHelper()
	result := b.acquireBuild(&runner)
	require.True(t, result)

	result = b.acquireBuild(&runner)
	require.False(t, result, "allow only one build")

	result = b.releaseBuild(&runner)
	require.True(t, result)

	result = b.releaseBuild(&runner)
	require.False(t, result, "release only one build")
}

func TestBuildsHelperAcquireBuildUnlimited(t *testing.T) {
	runner := common.RunnerConfig{
		Limit: 0,
	}

	b := newBuildsHelper()
	result := b.acquireBuild(&runner)
	require.True(t, result)

	result = b.acquireBuild(&runner)
	require.True(t, result)

	result = b.releaseBuild(&runner)
	require.True(t, result)

	result = b.releaseBuild(&runner)
	require.True(t, result)
}

func TestBuildsHelperFindSessionByURL(t *testing.T) {
	sess, err := session.NewSession(nil)
	require.NoError(t, err)
	build := common.Build{
		Session: sess,
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				Token: "abcd1234",
			},
		},
	}

	h := newBuildsHelper()
	h.addBuild(&build)

	foundSession, err := h.findSessionByURL(sess.Endpoint + "/action")
	require.NoError(t, err)
	assert.Equal(t, sess, foundSession)

	foundSession, err = h.findSessionByURL("/session/hash/action")
	assert.Nil(t, foundSession)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no session found matching URL")

	// Test empty URL
	foundSession, err = h.findSessionByURL("")
	assert.Nil(t, foundSession)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty URL provided")

	// Test with no builds
	h = newBuildsHelper()
	foundSession, err = h.findSessionByURL(sess.Endpoint + "/action")
	assert.Nil(t, foundSession)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no active builds found")
}

func TestBuildsHelper_ListJobsHandler(t *testing.T) {
	tests := map[string]struct {
		build          *common.Build
		expectedOutput []string
	}{
		"no jobs": {
			build: nil,
		},
		"job exists": {
			build: &common.Build{
				Runner: &common.RunnerConfig{},
				Job: spec.Job{
					ID:      1,
					JobInfo: spec.JobInfo{ProjectID: 1},
					GitInfo: spec.GitInfo{RepoURL: "https://gitlab.example.com/my-namespace/my-project.git"},
				},
			},
			expectedOutput: []string{
				"url=https://gitlab.example.com/my-namespace/my-project/-/jobs/1",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			writer := httptest.NewRecorder()

			req, err := http.NewRequest(http.MethodGet, "/", nil)
			require.NoError(t, err)

			b := newBuildsHelper()
			b.addBuild(test.build)
			b.ListJobsHandler(writer, req)

			resp := writer.Result()
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "2", resp.Header.Get("X-List-Version"))
			assert.Equal(t, "text/plain", resp.Header.Get(common.ContentType))

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if len(test.expectedOutput) == 0 {
				assert.Empty(t, body)
				return
			}

			for _, expectedOutput := range test.expectedOutput {
				assert.Contains(t, string(body), expectedOutput)
			}
		})
	}
}

func TestRestrictHTTPMethods(t *testing.T) {
	tests := map[string]int{
		http.MethodGet:  http.StatusOK,
		http.MethodHead: http.StatusOK,
		http.MethodPost: http.StatusMethodNotAllowed,
		"FOOBAR":        http.StatusMethodNotAllowed,
	}

	for method, expectedStatusCode := range tests {
		t.Run(method, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("hello world"))
			})

			server := httptest.NewServer(restrictHTTPMethods(mux, http.MethodGet, http.MethodHead))

			req, err := http.NewRequest(method, server.URL, nil)
			require.NoError(t, err)

			resp, err := server.Client().Do(req)
			require.NoError(t, err)
			require.Equal(t, expectedStatusCode, resp.StatusCode)
		})
	}
}

func TestBuildsHelper_evaluateJobQueuingDuration(t *testing.T) {
	type jobInfo struct {
		timeInQueueSeconds                       float64
		projectJobsRunningOnInstanceRunnersCount string
	}

	basicJob := jobInfo{
		timeInQueueSeconds:                       (15 * time.Second).Seconds(),
		projectJobsRunningOnInstanceRunnersCount: "0",
	}

	tc := map[string]struct {
		monitoringSectionMissing bool
		jobQueuingSectionMissing bool
		threshold                time.Duration
		jobsRunningForProject    string
		jobInfo                  jobInfo
		expectedValue            float64
	}{
		"no monitoring section in configuration": {
			monitoringSectionMissing: true,
			jobInfo:                  basicJob,
			expectedValue:            0,
		},
		"no jobQueuingDuration section in configuration": {
			jobQueuingSectionMissing: true,
			jobInfo:                  basicJob,
			expectedValue:            0,
		},
		"zeroed configuration": {
			jobInfo:       basicJob,
			expectedValue: 0,
		},
		"jobsRunningForProject not configured and threshold not exceeded": {
			threshold:     60 * time.Second,
			jobInfo:       basicJob,
			expectedValue: 0,
		},
		"jobsRunningForProject not configured and threshold exceeded": {
			threshold:     10 * time.Second,
			jobInfo:       basicJob,
			expectedValue: 1,
		},
		"jobsRunningForProject configured and matched and threshold not exceeded": {
			threshold:             60 * time.Second,
			jobsRunningForProject: ".*",
			jobInfo:               basicJob,
			expectedValue:         0,
		},
		"jobsRunningForProject configured and matched and threshold exceeded": {
			threshold:             10 * time.Second,
			jobsRunningForProject: ".*",
			jobInfo:               basicJob,
			expectedValue:         1,
		},
		"jobsRunningForProject configured and not matched and threshold not exceeded": {
			threshold:             60 * time.Second,
			jobsRunningForProject: "Inf+",
			jobInfo:               basicJob,
			expectedValue:         0,
		},
		"jobsRunningForProject configured and not matched and threshold exceeded": {
			threshold:             10 * time.Second,
			jobsRunningForProject: "Inf+",
			jobInfo:               basicJob,
			expectedValue:         0,
		},
	}

	for tn, tt := range tc {
		t.Run(tn, func(t *testing.T) {
			build := &common.Build{
				Runner: &common.RunnerConfig{
					Name: testName,
					RunnerCredentials: common.RunnerCredentials{
						Token: testToken,
					},
					SystemID: "testSystemID",
				},
				Job: spec.Job{
					ID: 1,
					JobInfo: spec.JobInfo{
						ProjectID:                                1,
						TimeInQueueSeconds:                       tt.jobInfo.timeInQueueSeconds,
						ProjectJobsRunningOnInstanceRunnersCount: tt.jobInfo.projectJobsRunningOnInstanceRunnersCount,
					},
				},
			}

			if !tt.monitoringSectionMissing {
				build.Runner.Monitoring = &runner.Monitoring{}

				if !tt.jobQueuingSectionMissing {
					build.Runner.Monitoring.JobQueuingDurations = monitoring.JobQueuingDurations{
						&monitoring.JobQueuingDuration{
							Periods:               []string{"* * * * * * *"},
							Threshold:             tt.threshold,
							JobsRunningForProject: tt.jobsRunningForProject,
						},
					}
				}
				require.NoError(t, build.Runner.Monitoring.Compile())
			}

			b := newBuildsHelper()
			b.addBuild(build)

			ch := make(chan prometheus.Metric, 1)
			b.acceptableJobQueuingDurationExceeded.Collect(ch)

			m := <-ch

			var mm dto.Metric
			err := m.Write(&mm)
			require.NoError(t, err)

			labels := make(map[string]string)
			for _, l := range mm.GetLabel() {
				if !assert.NotNil(t, l.Name) {
					continue
				}

				if !assert.NotNil(t, l.Value) {
					continue
				}

				labels[*l.Name] = *l.Value
			}

			assert.Len(t, labels, 3)
			require.Contains(t, labels, "runner")
			assert.Equal(t, testToken, labels["runner"])
			require.Contains(t, labels, "runner_name")
			assert.Equal(t, testName, labels["runner_name"])
			require.Contains(t, labels, "system_id")
			assert.Equal(t, build.Runner.SystemID, labels["system_id"])

			assert.Equal(t, tt.expectedValue, mm.GetCounter().GetValue())
		})
	}
}

func TestJobExecutionModeTotal(t *testing.T) {
	tests := map[string]struct {
		mode             common.JobExecutionMode
		executor         string
		expectedExecutor string
		expectedValue    float64
	}{
		"steps mode": {
			mode:             common.JobExecutionModeSteps,
			executor:         "docker",
			expectedExecutor: "docker",
			expectedValue:    1,
		},
		"traditional mode": {
			mode:             common.JobExecutionModeTraditional,
			executor:         "docker+machine",
			expectedExecutor: "docker+machine",
			expectedValue:    1,
		},
		"empty executor uses unknown label": {
			mode:             common.JobExecutionModeTraditional,
			executor:         "",
			expectedExecutor: "unknown",
			expectedValue:    1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			build := &common.Build{
				Runner: &common.RunnerConfig{
					Name: testName,
					RunnerCredentials: common.RunnerCredentials{
						Token: testToken,
					},
					SystemID: "testSystemID",
				},
				Job: spec.Job{
					ID: 1,
					JobInfo: spec.JobInfo{
						ProjectID: 1,
					},
				},
			}

			b := newBuildsHelper()
			b.addBuild(build)
			build.OnJobExecutionModeDispatchedFn.Call(tt.mode, tt.executor)

			ch := make(chan prometheus.Metric, 1)
			b.jobExecutionModeTotal.Collect(ch)

			m := <-ch

			var mm dto.Metric
			err := m.Write(&mm)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedValue, mm.GetCounter().GetValue())

			labels := make(map[string]string)
			for _, l := range mm.GetLabel() {
				if l.Name != nil && l.Value != nil {
					labels[*l.Name] = *l.Value
				}
			}

			assert.Equal(t, string(tt.mode), labels["mode"])
			assert.Equal(t, tt.expectedExecutor, labels["executor"])
		})
	}
}

func TestPrepareStageMetrics(t *testing.T) {
	build := &common.Build{
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				Token: testToken,
			},
		},
		Job: spec.Job{
			ID: 1,
			JobInfo: spec.JobInfo{
				ProjectID: 1,
			},
		},
	}

	build.Runner.Environment = append(build.Runner.Environment, fmt.Sprintf("%s=true", featureflags.ExportHighCardinalityMetrics))

	bh := newBuildsHelper()
	bh.addBuild(build)

	bh.initializeBuildStageMetrics(build)

	// verify that the FF toggle will work correctly
	require.NotNil(t, bh.buildStagesStartTimes)

	bh.handleOnBuildStageStart(build, common.BuildStagePrepare)
	time.Sleep(100 * time.Millisecond)
	bh.handleOnBuildStageEnd(build, common.BuildStagePrepare)

	ch := make(chan prometheus.Metric, 1)
	bh.jobStagesDurationHistogram.Collect(ch)

	var mm dto.Metric
	_ = (<-ch).Write(&mm)

	require.NotEmpty(t, mm.Label)
	require.NotNil(t, mm.Histogram)
	require.Equal(t, int(*mm.Histogram.SampleCount), 1)
	require.GreaterOrEqual(t, *mm.Histogram.SampleSum, float64(0.1))
}

func TestPrepareStageMetricsNoFF(t *testing.T) {
	build := &common.Build{
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				Token: testToken,
			},
		},
		Job: spec.Job{
			ID: 1,
			JobInfo: spec.JobInfo{
				ProjectID: 1,
			},
		},
	}

	bh := newBuildsHelper()
	bh.addBuild(build)

	bh.initializeBuildStageMetrics(build)

	require.Nil(t, bh.buildStagesStartTimes)
}

func TestEnsureJobsTotalIsZero(t *testing.T) {
	runner := &common.RunnerConfig{
		Name: testName,
		RunnerCredentials: common.RunnerCredentials{
			Token: testToken,
		},
		SystemID: "testSystemID",
	}

	bh := newBuildsHelper()
	bh.getRunnerCounter(runner)

	ch := make(chan prometheus.Metric, 1)
	bh.jobsTotal.Collect(ch)

	var mm dto.Metric
	err := (<-ch).Write(&mm)
	require.NoError(t, err)

	assert.Equal(t, float64(0), mm.GetCounter().GetValue())

	labels := make(map[string]string)
	for _, l := range mm.GetLabel() {
		if l.Name != nil && l.Value != nil {
			labels[*l.Name] = *l.Value
		}
	}

	assert.Equal(t, runner.ShortDescription(), labels["runner"])
	assert.Equal(t, runner.Name, labels["runner_name"])
	assert.Equal(t, runner.GetSystemID(), labels["system_id"])
}

//go:build !integration

package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/testutil"
)

type testFilterJobState struct {
	buildState  common.BuildRuntimeState
	healthCheck time.Time
	retries     int
}

func (t testFilterJobState) Job() *common.Job {
	encodedJob := &common.EncodedJob{
		State: &common.EncodedJobRuntimeState{},
	}

	encodedJob.State.BuildState = t.buildState
	encodedJob.State.HealthCheckAt = t.healthCheck
	encodedJob.State.Retries = t.retries

	job, _ := encodedJob.FromEncoded()

	return job
}

func TestCanResumeJobFilter(t *testing.T) {
	tests := map[string]struct {
		cfg                  *common.StoreConfig
		jobState             testFilterJobState
		expectedFilterResult bool
	}{
		"Job is running": {
			cfg: &common.StoreConfig{
				HealthTimeout: testutil.Ptr(1),
				MaxRetries:    testutil.Ptr(3),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeRunning,
				healthCheck: time.Now().Add(-time.Minute),
				retries:     1,
			},
			expectedFilterResult: true,
		},
		"Job is failed": {
			cfg: &common.StoreConfig{
				HealthTimeout: testutil.Ptr(1),
				MaxRetries:    testutil.Ptr(3),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeFailed,
				healthCheck: time.Now().Add(-time.Minute),
				retries:     1,
			},
			expectedFilterResult: false,
		},
		"Job is pending": {
			cfg: &common.StoreConfig{
				HealthTimeout: testutil.Ptr(1),
				MaxRetries:    testutil.Ptr(3),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunStatePending,
				healthCheck: time.Now().Add(-time.Minute),
				retries:     1,
			},
			expectedFilterResult: true,
		},
		"Job health check timeout and has retries": {
			cfg: &common.StoreConfig{
				HealthTimeout: testutil.Ptr(int((5 * time.Minute).Seconds())),
				MaxRetries:    testutil.Ptr(3),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeRunning,
				healthCheck: time.Now().Add(-10 * time.Minute),
				retries:     2,
			},
			expectedFilterResult: true,
		},
		"Job health check timeout and has no retries": {
			cfg: &common.StoreConfig{
				HealthTimeout: testutil.Ptr(int((5 * time.Minute).Seconds())),
				MaxRetries:    testutil.Ptr(3),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeRunning,
				healthCheck: time.Now().Add(-10 * time.Minute),
				retries:     3,
			},
			expectedFilterResult: false,
		},
		"Job retries greater than max retries": {
			cfg: &common.StoreConfig{
				HealthTimeout: testutil.Ptr(int((5 * time.Minute).Seconds())),
				MaxRetries:    testutil.Ptr(3),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeRunning,
				healthCheck: time.Now().Add(-time.Minute),
				retries:     4,
			},
			expectedFilterResult: false,
		},
		"Job health check within timeout": {
			cfg: &common.StoreConfig{
				HealthTimeout: testutil.Ptr(int((5 * time.Minute).Seconds())),
				MaxRetries:    testutil.Ptr(3),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeRunning,
				healthCheck: time.Now().Add(-2 * time.Minute),
				retries:     1,
			},
			expectedFilterResult: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			require.Equal(t, tt.expectedFilterResult, newCanResumeJobFilter(tt.cfg)(tt.jobState.Job()))
		})
	}
}

func TestCanDeleteJobFilter(t *testing.T) {
	tests := map[string]struct {
		cfg                  *common.StoreConfig
		jobState             testFilterJobState
		expectedFilterResult bool
	}{
		"Job is running not stale yet": {
			cfg: &common.StoreConfig{
				StaleTimeout: testutil.Ptr(60),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeRunning,
				healthCheck: time.Now().Add(-time.Second),
			},
			expectedFilterResult: false,
		},
		"Job is running but is stale": {
			cfg: &common.StoreConfig{
				StaleTimeout: testutil.Ptr(1),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeRunning,
				healthCheck: time.Now().Add(-time.Minute),
			},
			expectedFilterResult: true,
		},
		"Job is failed and not stale": {
			cfg: &common.StoreConfig{
				StaleTimeout: testutil.Ptr(60),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeFailed,
				healthCheck: time.Now().Add(-time.Second),
			},
			expectedFilterResult: true,
		},
		"Job is pending not stale": {
			cfg: &common.StoreConfig{
				StaleTimeout: testutil.Ptr(60),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunStatePending,
				healthCheck: time.Now().Add(-time.Second),
			},
			expectedFilterResult: false,
		},
		"Job is pending and stale": {
			cfg: &common.StoreConfig{
				StaleTimeout: testutil.Ptr(1),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunStatePending,
				healthCheck: time.Now().Add(-time.Minute),
			},
			expectedFilterResult: true,
		},
		"Job is cancelled": {
			cfg: &common.StoreConfig{
				StaleTimeout: testutil.Ptr(1),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeCanceled,
				healthCheck: time.Now().Add(-time.Minute),
			},
			expectedFilterResult: true,
		},
		"Job is terminated": {
			cfg: &common.StoreConfig{
				StaleTimeout: testutil.Ptr(1),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeTerminated,
				healthCheck: time.Now().Add(-time.Minute),
			},
			expectedFilterResult: true,
		},
		"Job is timed out": {
			cfg: &common.StoreConfig{
				StaleTimeout: testutil.Ptr(1),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeTimedout,
				healthCheck: time.Now().Add(-time.Minute),
			},
			expectedFilterResult: true,
		},
		"Job is running and over retry limit": {
			cfg: &common.StoreConfig{
				StaleTimeout:  testutil.Ptr(60),
				HealthTimeout: testutil.Ptr(60),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeRunning,
				healthCheck: time.Now().Add(-time.Second),
			},
			expectedFilterResult: false,
		},
		"Job is unhealthy and over retry limit": {
			cfg: &common.StoreConfig{
				StaleTimeout:  testutil.Ptr(3600),
				HealthTimeout: testutil.Ptr(1),
				MaxRetries:    testutil.Ptr(3),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeRunning,
				healthCheck: time.Now().Add(-time.Minute),
				retries:     4,
			},
			expectedFilterResult: true,
		},
		"Job is unhealthy and at retry limit": {
			cfg: &common.StoreConfig{
				StaleTimeout:  testutil.Ptr(3600),
				HealthTimeout: testutil.Ptr(1),
				MaxRetries:    testutil.Ptr(3),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeRunning,
				healthCheck: time.Now().Add(-time.Minute),
				retries:     3,
			},
			expectedFilterResult: true,
		},
		"Job is unhealthy and below": {
			cfg: &common.StoreConfig{
				StaleTimeout:  testutil.Ptr(3600),
				HealthTimeout: testutil.Ptr(1),
				MaxRetries:    testutil.Ptr(3),
			},
			jobState: testFilterJobState{
				buildState:  common.BuildRunRuntimeRunning,
				healthCheck: time.Now().Add(-time.Minute),
				retries:     2,
			},
			expectedFilterResult: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			require.Equal(t, tt.expectedFilterResult, newCanDeleteJobFilter(tt.cfg)(tt.jobState.Job()))
		})
	}
}

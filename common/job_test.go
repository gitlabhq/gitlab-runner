//go:build !integration

package common

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestJob(t *testing.T) {
	response, err := GetSuccessfulBuild()
	require.NoError(t, err)

	job := NewJob(&response)
	require.NotNil(t, job)
	assert.Equal(t, response, *job.JobResponse)
	assert.NotNil(t, job.State)
}

func TestStatefulJobManager_RequestJob_Store_Empty(t *testing.T) {
	response, err := GetSuccessfulBuild()
	require.NoError(t, err)

	store := NewMockJobStore(t)
	store.On("Request").Return(nil, nil).Once()

	nw := NewMockNetwork(t)
	nw.On("RequestJob", mock.Anything, mock.Anything, mock.Anything).Return(&response, true).Once()

	manager := NewStatefulJobManager(
		nw,
		store,
		nil,
		&RunnerConfig{},
	)

	newJob, healthy := manager.RequestJob(context.Background(), nil)
	require.True(t, healthy)
	require.Equal(t, response, *newJob.JobResponse)
	require.NotNil(t, newJob.State)
	require.NotNil(t, manager.job)
}

func TestStatefulJobManager_RequestJob_Store(t *testing.T) {
	response, err := GetSuccessfulBuild()
	require.NoError(t, err)

	job := NewJob(&response)
	// simulate job being restored
	job.State.SetStage(BuildStageResolveSecrets)

	store := NewMockJobStore(t)
	store.On("Request").Return(job, nil).Once()
	store.On("Update", job).Return(nil).Once()

	manager := NewStatefulJobManager(
		nil,
		store,
		nil,
		&RunnerConfig{},
	)

	newJob, healthy := manager.RequestJob(context.Background(), nil)
	// wait for the update goroutine to start
	time.Sleep(100 * time.Millisecond)
	require.True(t, healthy)
	require.Equal(t, response, *newJob.JobResponse)
	require.NotNil(t, newJob.State)
	require.True(t, newJob.State.IsResumed())
}

func TestStatefulJobManager_RequestJob_Store_Error_Should_Call_Network(t *testing.T) {
	response, err := GetSuccessfulBuild()
	require.NoError(t, err)

	store := NewMockJobStore(t)
	store.On("Request").Return(nil, errors.New("err")).Once()

	nw := NewMockNetwork(t)
	nw.On("RequestJob", mock.Anything, mock.Anything, mock.Anything).Return(&response, true).Once()

	manager := NewStatefulJobManager(
		nw,
		store,
		nil,
		&RunnerConfig{},
	)

	newJob, healthy := manager.RequestJob(context.Background(), nil)
	require.True(t, healthy)
	require.Equal(t, response, *newJob.JobResponse)
	require.NotNil(t, newJob.State)
}

func TestStatefulJobManager_ProcessJob_Not_Resumed(t *testing.T) {
	ptr := func(v int) *int {
		return &v
	}

	response, err := GetSuccessfulBuild()
	require.NoError(t, err)

	cfg := RunnerConfig{}
	cfg.Store = &StoreConfig{
		HealthInterval: ptr(1),
	}

	healthCheck := time.Now()

	job := NewJob(&response)
	job.State.SetSentTrace(11)
	job.State.healthCheckAt = healthCheck

	store := NewMockJobStore(t)
	store.On("Update", job).Return(nil)

	trace := NewMockJobTrace(t)
	trace.On("Start").Once()
	ch := make(chan struct{})
	trace.On("Done").
		Run(func(mock.Arguments) {
			time.AfterFunc(time.Second*2, func() {
				close(ch)
			})
		}).
		Return((<-chan struct{})(ch)).
		Once()

	trace.On("Done").Return((<-chan struct{})(ch))

	manager := NewStatefulJobManager(
		nil,
		store,
		func(jobManager JobManager, config RunnerConfig, jobCredentials *JobCredentials, startOffset int64) (JobTrace, error) {
			assert.Equal(t, 11, int(startOffset))
			return trace, nil
		},
		&cfg,
	)
	manager.job = job

	newTrace, err := manager.ProcessJob(nil)
	time.Sleep(2 * time.Second)
	require.NoError(t, err)
	assert.Equal(t, newTrace, trace)
	assert.NotEqual(t, healthCheck, job.State.GetHealthCheckAt())
}

func TestStatefulJobManager_ProcessJob_Resumed(t *testing.T) {
	response, err := GetSuccessfulBuild()
	require.NoError(t, err)

	cfg := RunnerConfig{}
	cfg.Store = &StoreConfig{
		HealthInterval: ptr(1),
	}

	healthCheck := time.Now()

	job := NewJob(&response)
	job.State.SetSentTrace(11)
	job.State.SetBuildState(BuildRunRuntimeRunning)
	job.State.SetStage(BuildStageResolveSecrets)
	job.State.healthCheckAt = healthCheck
	job.State.Resume()

	store := NewMockJobStore(t)
	store.On("Update", job).Return(nil)

	trace := NewMockJobTrace(t)
	trace.On("Start").Once()
	// Disable has to be called as well when a Job is considered resumed
	trace.On("Disable").Once()
	ch := make(chan struct{})
	trace.On("Done").
		Run(func(mock.Arguments) {
			time.AfterFunc(time.Second*2, func() {
				close(ch)
			})
		}).
		Return((<-chan struct{})(ch)).
		Once()

	trace.On("Done").Return((<-chan struct{})(ch))

	manager := NewStatefulJobManager(
		nil,
		store,
		func(jobManager JobManager, config RunnerConfig, jobCredentials *JobCredentials, startOffset int64) (JobTrace, error) {
			assert.Equal(t, 11, int(startOffset))
			return trace, nil
		},
		&cfg,
	)
	manager.job = job

	newTrace, err := manager.ProcessJob(nil)
	time.Sleep(2 * time.Second)
	require.NoError(t, err)
	assert.Equal(t, newTrace, trace)
	assert.NotEqual(t, healthCheck, job.State.GetHealthCheckAt())
}

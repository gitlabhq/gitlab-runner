package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var (
	jobConfig      = common.RunnerConfig{}
	jobCredentials = &common.JobCredentials{ID: -1}
	jobOutputLimit = common.RunnerConfig{OutputLimit: 1}
)

func matchJobState(
	jobInfo common.UpdateJobInfo,
	id int,
	state common.JobState,
	failureReason common.JobFailureReason,
) bool {
	if jobInfo.ID != id {
		return false
	}
	if jobInfo.State != state {
		return false
	}
	if jobInfo.FailureReason != failureReason {
		return false
	}
	return true
}

// nolint:unparam
func generateJobInfoMatcher(id int, state common.JobState, failureReason common.JobFailureReason) interface{} {
	return mock.MatchedBy(func(jobInfo common.UpdateJobInfo) bool {
		return matchJobState(jobInfo, id, state, failureReason)
	})
}

func ignoreOptionalTouchJob(mockNetwork *common.MockNetwork) {
	touchMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Running, "")

	// due to timing the `trace.touchJob()` can be executed
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, touchMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Maybe()
}

func TestIgnoreStatusChange(t *testing.T) {
	jobInfoMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	// expect to receive just one status
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, jobInfoMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once()

	b, err := newJobTrace(mockNetwork, jobConfig, jobCredentials)
	require.NoError(t, err)

	b.start()
	b.Success()
	b.Fail(errors.New("test"), "script_failure")
}

func TestTouchJobAbort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	keepAliveUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Running, "")
	updateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	// abort while running
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, keepAliveUpdateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateAbort}).Once()

	// try to send status at least once more
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateAbort}).Once()

	b, err := newJobTrace(mockNetwork, jobConfig, jobCredentials)
	require.NoError(t, err)

	b.updateInterval = 0
	b.SetCancelFunc(cancel)

	b.start()
	assert.NotNil(t, <-ctx.Done(), "should abort the job")
	b.Success()
}

func TestSendPatchAbort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	// abort while running
	// 1. on `incrementalUpdate() -> sendPatch()`
	// 2. on `finalTraceUpdate() -> sendPatch()`
	mockNetwork.On("PatchTrace", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(common.NewPatchTraceResult(0, common.PatchAbort, 0)).Twice()

	// try to send status at least once more
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateAbort}).Once()

	b, err := newJobTrace(mockNetwork, jobConfig, jobCredentials)
	require.NoError(t, err)

	b.SetCancelFunc(cancel)

	fmt.Fprint(b, "Trace")
	b.start()
	assert.NotNil(t, <-ctx.Done(), "should abort the job")
	b.Success()
}

func TestJobOutputLimit(t *testing.T) {
	traceMessage := "abcde"

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	b, err := newJobTrace(mockNetwork, jobOutputLimit, jobCredentials)
	require.NoError(t, err)

	// prevent any UpdateJob before `b.Success()` call
	b.updateInterval = 25 * time.Second

	updateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	receivedTrace := bytes.NewBuffer([]byte{})
	mockNetwork.On("PatchTrace", jobOutputLimit, jobCredentials, mock.Anything, mock.Anything).
		Return(common.NewPatchTraceResult(1078, common.PatchSucceeded, 0)).
		Once().
		Run(func(args mock.Arguments) {
			// the 1078 == len(data)
			data := args.Get(2).([]byte)
			receivedTrace.Write(data)
		})

	mockNetwork.On("UpdateJob", jobOutputLimit, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once()

	b.start()
	// Write 5k to the buffer
	for i := 0; i < 1024; i++ {
		fmt.Fprint(b, traceMessage)
	}
	b.Success()

	expectedLogLimitExceededMsg := "Job's log exceeded limit of"

	assert.Contains(t, receivedTrace.String(), traceMessage)
	assert.Contains(t, receivedTrace.String(), expectedLogLimitExceededMsg)
}

func TestJobMasking(t *testing.T) {
	maskedValues := []string{"masked"}
	traceMessage := "This string should be masked"
	traceMaskedMessage := "This string should be [MASKED]"

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	ignoreOptionalTouchJob(mockNetwork)

	mockNetwork.On("PatchTrace", mock.Anything, mock.Anything, []byte(traceMaskedMessage), 0).
		Return(common.NewPatchTraceResult(len(traceMaskedMessage), common.PatchSucceeded, 0))

	mockNetwork.On("UpdateJob", mock.Anything, mock.Anything, mock.Anything).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded})

	jobTrace, err := newJobTrace(mockNetwork, jobConfig, jobCredentials)
	require.NoError(t, err)

	jobTrace.SetMasked(maskedValues)
	jobTrace.start()

	_, err = jobTrace.Write([]byte(traceMessage))
	require.NoError(t, err)
	jobTrace.Success()
}

func TestJobFinishTraceUpdateRetry(t *testing.T) {
	updateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	ignoreOptionalTouchJob(mockNetwork)

	// accept just 3 bytes
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("My trace send"), 0).
		Return(common.NewPatchTraceResult(3, common.PatchSucceeded, 0)).Once()

	// retry when trying to send next bytes
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("trace send"), 3).
		Return(common.NewPatchTraceResult(0, common.PatchFailed, 0)).Once()

	// accept 6 more bytes
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("trace send"), 3).
		Return(common.NewPatchTraceResult(9, common.PatchSucceeded, 0)).Once()

	// restart most of trace
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("send"), 9).
		Return(common.NewPatchTraceResult(6, common.PatchRangeMismatch, 0)).Once()

	// accept rest of trace
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("ce send"), 6).
		Return(common.NewPatchTraceResult(13, common.PatchSucceeded, 0)).Once()

	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once()

	b, err := newJobTrace(mockNetwork, jobConfig, jobCredentials)
	require.NoError(t, err)

	b.updateInterval = 10 * time.Millisecond

	b.start()
	fmt.Fprint(b, "My trace send")
	b.Success()
}

func TestJobMaxTracePatchSize(t *testing.T) {
	updateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	ignoreOptionalTouchJob(mockNetwork)

	// expect just 5 bytes
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("My tr"), 0).
		Return(common.NewPatchTraceResult(5, common.PatchSucceeded, 0)).Once()

	// expect next 5 bytes
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("ace s"), 5).
		Return(common.NewPatchTraceResult(10, common.PatchSucceeded, 0)).Once()

	// expect last 3 bytes
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("end"), 10).
		Return(common.NewPatchTraceResult(13, common.PatchSucceeded, 0)).Once()

	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once()

	b, err := newJobTrace(mockNetwork, jobConfig, jobCredentials)
	require.NoError(t, err)

	b.updateInterval = 10 * time.Millisecond
	b.maxTracePatchSize = 5

	b.start()
	fmt.Fprint(b, "My trace send")
	b.Success()
}

func TestJobFinishStatusUpdateRetry(t *testing.T) {
	updateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	ignoreOptionalTouchJob(mockNetwork)

	// fail job 5 times
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateFailed}).Times(5)

	// accept job
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once()

	b, err := newJobTrace(mockNetwork, jobConfig, jobCredentials)
	require.NoError(t, err)

	b.updateInterval = 10 * time.Millisecond

	b.start()
	b.Success()
}

func TestJobIncrementalPatchSend(t *testing.T) {
	var wg sync.WaitGroup

	finalUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	ignoreOptionalTouchJob(mockNetwork)

	// ensure that PatchTrace gets executed first
	wg.Add(1)
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("test trace"), 0).
		Return(common.NewPatchTraceResult(10, common.PatchSucceeded, 0)).Once().
		Run(func(args mock.Arguments) {
			wg.Done()
		})

	// wait for the final `UpdateJob` to be executed
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, finalUpdateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once()

	b, err := newJobTrace(mockNetwork, jobConfig, jobCredentials)
	require.NoError(t, err)

	b.updateInterval = time.Millisecond * 10
	b.start()
	fmt.Fprint(b, "test trace")
	wg.Wait()
	b.Success()
}

func TestJobIncrementalStatusRefresh(t *testing.T) {
	var wg sync.WaitGroup

	incrementalUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Running, "")
	finalUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	// ensure that incremental UpdateJob gets executed first
	wg.Add(1)
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, incrementalUpdateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once().
		Run(func(args mock.Arguments) {
			wg.Done()
		})

	// wait for the final `UpdateJob` to be executed
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, finalUpdateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once()

	b, err := newJobTrace(mockNetwork, jobConfig, jobCredentials)
	require.NoError(t, err)

	b.updateInterval = time.Millisecond * 10

	// Test for: https://gitlab.com/gitlab-org/gitlab-ce/issues/63972
	// 1. lock, to prevent incrementalUpdate to read state
	// 2. inject final state as early as possible
	b.lock.Lock()
	b.start()
	b.state = common.Success
	b.lock.Unlock()

	wg.Wait()
	b.finish()
}

func TestUpdateIntervalChanges(t *testing.T) {
	testTrace := "Test trace"
	touchUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Running, "")
	finalUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	traceUpdateIntervalDefault := 10 * time.Millisecond

	tests := map[string]struct {
		initialUpdateInterval   time.Duration
		requestedUpdateInterval int
		finalUpdateInterval     time.Duration
	}{
		"negative updateInterval requested": {
			initialUpdateInterval:   traceUpdateIntervalDefault,
			requestedUpdateInterval: -10,
			finalUpdateInterval:     traceUpdateIntervalDefault,
		},
		"zero updateInterval requested": {
			initialUpdateInterval:   traceUpdateIntervalDefault,
			requestedUpdateInterval: 0,
			finalUpdateInterval:     traceUpdateIntervalDefault,
		},
		"positive updateInterval requested": {
			initialUpdateInterval:   traceUpdateIntervalDefault,
			requestedUpdateInterval: 10,
			finalUpdateInterval:     10 * time.Second,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Run("sendPatch", func(t *testing.T) {
				client := new(common.MockNetwork)
				defer client.AssertExpectations(t)

				waitForPatch := new(sync.WaitGroup)
				waitForPatch.Add(1)

				client.On("PatchTrace", jobConfig, jobCredentials, []byte(testTrace), 0).
					Return(common.NewPatchTraceResult(
						len(testTrace),
						common.PatchSucceeded,
						tt.requestedUpdateInterval,
					)).
					Run(func(_ mock.Arguments) {
						waitForPatch.Done()
					}).
					Once()

				// Ignore all subequent touch jobs
				ignoreOptionalTouchJob(client)

				client.On("UpdateJob", jobConfig, jobCredentials, finalUpdateMatcher).
					Return(common.UpdateJobResult{State: common.UpdateSucceeded}).
					Once()

				trace, err := newJobTrace(client, jobConfig, jobCredentials)
				require.NoError(t, err)

				trace.updateInterval = tt.initialUpdateInterval

				trace.start()
				assert.Equal(t, tt.initialUpdateInterval, trace.getUpdateInterval())

				_, err = fmt.Fprint(trace, testTrace)
				require.NoError(t, err)

				waitForPatch.Wait()
				assert.Equal(t, tt.finalUpdateInterval, trace.getUpdateInterval())
				trace.Success()
			})

			t.Run("touchJob", func(t *testing.T) {
				client := new(common.MockNetwork)
				defer client.AssertExpectations(t)

				waitForTouchJob := new(sync.WaitGroup)
				waitForTouchJob.Add(1)

				client.On("UpdateJob", jobConfig, jobCredentials, touchUpdateMatcher).
					Return(common.UpdateJobResult{
						State:             common.UpdateSucceeded,
						NewUpdateInterval: time.Duration(tt.requestedUpdateInterval) * time.Second,
					}).
					Run(func(_ mock.Arguments) {
						waitForTouchJob.Done()
					}).
					Once()

				client.On("UpdateJob", jobConfig, jobCredentials, finalUpdateMatcher).
					Return(common.UpdateJobResult{State: common.UpdateSucceeded}).
					Once()

				trace, err := newJobTrace(client, jobConfig, jobCredentials)
				require.NoError(t, err)

				trace.updateInterval = tt.initialUpdateInterval
				trace.forceSendInterval = 0

				trace.start()
				assert.Equal(t, tt.initialUpdateInterval, trace.getUpdateInterval())

				waitForTouchJob.Wait()
				assert.Equal(t, tt.finalUpdateInterval, trace.getUpdateInterval())
				trace.Success()
			})

			t.Run("finalStatusUpdate", func(t *testing.T) {
				client := new(common.MockNetwork)
				defer client.AssertExpectations(t)

				waitForFinalUpdate := new(sync.WaitGroup)
				waitForFinalUpdate.Add(1)

				ignoreOptionalTouchJob(client)

				client.On("UpdateJob", jobConfig, jobCredentials, finalUpdateMatcher).
					Return(common.UpdateJobResult{
						State:             common.UpdateSucceeded,
						NewUpdateInterval: time.Duration(tt.requestedUpdateInterval) * time.Second,
					}).
					Run(func(_ mock.Arguments) {
						waitForFinalUpdate.Done()
					}).
					Once()

				trace, err := newJobTrace(client, jobConfig, jobCredentials)
				require.NoError(t, err)

				trace.updateInterval = tt.initialUpdateInterval

				trace.start()
				assert.Equal(t, tt.initialUpdateInterval, trace.getUpdateInterval())
				trace.Success()

				waitForFinalUpdate.Wait()
				assert.Equal(t, tt.finalUpdateInterval, trace.getUpdateInterval())
			})
		})
	}
}

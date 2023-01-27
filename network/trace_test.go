//go:build !integration

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
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

var (
	jobConfig      = common.RunnerConfig{}
	jobCredentials = &common.JobCredentials{ID: -1}
	jobOutputLimit = common.RunnerConfig{OutputLimit: 1}
)

func matchJobState(
	jobInfo common.UpdateJobInfo,
	id int64,
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
func generateJobInfoMatcher(id int64, state common.JobState, failureReason common.JobFailureReason) interface{} {
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

func newTestJobTrace(network *common.MockNetwork, config common.RunnerConfig) (*clientJobTrace, error) {
	trace, err := newJobTrace(network, config, jobCredentials)
	if err != nil {
		return nil, err
	}

	// use long mask token to ensure that tests that check for incremental updates
	// use \n to flush to the network.
	trace.SetMasked(common.MaskOptions{Phrases: []string{"really_long_mask_token_for_tests"}})

	return trace, err
}

func TestIgnoreStatusChange(t *testing.T) {
	jobInfoMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	// expect to receive just one status
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, jobInfoMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once()

	b, err := newTestJobTrace(mockNetwork, jobConfig)
	require.NoError(t, err)

	b.start()
	b.Success()
	b.Fail(errors.New("test"), common.JobFailureData{Reason: "script_failure"})
}

func TestTouchJobAbort(t *testing.T) {
	abortCtx, abort := context.WithCancel(context.Background())
	defer abort()

	cancelCtx, cancel := context.WithCancel(context.Background())
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

	b, err := newTestJobTrace(mockNetwork, jobConfig)
	require.NoError(t, err)

	b.updateInterval = 0
	b.SetAbortFunc(abort)
	b.SetCancelFunc(cancel)

	b.start()
	assert.NotNil(t, <-abortCtx.Done(), "should abort the job")
	assert.Nil(t, cancelCtx.Err(), "should not cancel job")
	b.Success()
}

func TestTouchJobCancel(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	abortCtx, abort := context.WithCancel(context.Background())
	defer abort()

	keepAliveUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Running, "")
	updateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	// cancel while running
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, keepAliveUpdateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded, CancelRequested: true}).Once()

	// try to send status at least once more
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded, CancelRequested: true}).Once()

	b, err := newTestJobTrace(mockNetwork, jobConfig)
	require.NoError(t, err)

	b.updateInterval = 0
	b.SetCancelFunc(cancel)
	b.SetAbortFunc(abort)

	b.start()
	assert.NotNil(t, <-cancelCtx.Done(), "should cancel the job")
	assert.NoError(t, abortCtx.Err())
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
	mockNetwork.On("PatchTrace", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(common.NewPatchTraceResult(0, common.PatchAbort, 0)).Twice()

	ignoreOptionalTouchJob(mockNetwork)

	// try to send status at least once more
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateAbort}).Once()

	b, err := newTestJobTrace(mockNetwork, jobConfig)
	require.NoError(t, err)

	b.SetAbortFunc(cancel)
	b.updateInterval = time.Microsecond

	fmt.Fprint(b, "Trace\n")
	b.start()
	assert.NotNil(t, <-ctx.Done(), "should abort the job")
	b.Success()
}

func TestJobOutputLimit(t *testing.T) {
	traceMessage := "abcde"
	traceMessageSize := 1024

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	b, err := newTestJobTrace(mockNetwork, jobOutputLimit)
	require.NoError(t, err)

	// prevent any UpdateJob before `b.Success()` call
	b.updateInterval = 25 * time.Second

	updateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	expectedLogLimitExceededMsg := fmt.Sprintf(
		"\n\x1b[33;1mJob's log exceeded limit of %v bytes.\n"+
			"Job execution will continue but no more output will be collected.\x1b[0;m\n",
		traceMessageSize,
	)
	expectedLogLength := jobOutputLimit.OutputLimit*traceMessageSize + len(expectedLogLimitExceededMsg)

	receivedTrace := bytes.NewBuffer([]byte{})
	mockNetwork.On("PatchTrace", jobOutputLimit, jobCredentials, mock.Anything, mock.Anything, mock.Anything).
		Return(common.NewPatchTraceResult(expectedLogLength, common.PatchSucceeded, 0)).
		Once().
		Run(func(args mock.Arguments) {
			// the expectedLogLength == len(data)
			data := args.Get(2).([]byte)
			receivedTrace.Write(data)
		})

	mockNetwork.On("UpdateJob", jobOutputLimit, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once()

	b.start()
	// Write 5k to the buffer
	for i := 0; i < traceMessageSize; i++ {
		fmt.Fprint(b, traceMessage)
	}
	b.Success()

	assert.Contains(t, receivedTrace.String(), traceMessage)
	assert.Contains(t, receivedTrace.String(), expectedLogLimitExceededMsg)
}

func TestJobMasking(t *testing.T) {
	maskedValues := common.MaskOptions{Phrases: []string{"masked"}}
	traceMessage := "This string should be masked"
	traceMaskedMessage := "This string should be [MASKED]"

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	ignoreOptionalTouchJob(mockNetwork)

	mockNetwork.On("PatchTrace", mock.Anything, mock.Anything, []byte(traceMaskedMessage), 0, false).
		Return(common.NewPatchTraceResult(len(traceMaskedMessage), common.PatchSucceeded, 0))

	mockNetwork.On("UpdateJob", mock.Anything, mock.Anything, mock.Anything).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded})

	jobTrace, err := newTestJobTrace(mockNetwork, jobConfig)
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

	b, err := newTestJobTrace(mockNetwork, jobConfig)
	require.NoError(t, err)

	// accept just 3 bytes
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("My trace send"), 0, false).
		Return(common.NewPatchTraceResult(3, common.PatchSucceeded, 0)).
		Once()

	// retry when trying to send next bytes
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("trace send"), 3, false).
		Return(common.NewPatchTraceResult(0, common.PatchFailed, 0)).
		Run(func(args mock.Arguments) {
			// Ensure that short interval is used on retry to speed-up test
			b.setUpdateInterval(time.Microsecond)
		}).
		Once()

	// accept 6 more bytes
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("trace send"), 3, false).
		Return(common.NewPatchTraceResult(9, common.PatchSucceeded, 0)).
		Once()

	// restart most of trace
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("send"), 9, false).
		Return(common.NewPatchTraceResult(6, common.PatchRangeMismatch, 0)).
		Once()

	// accept rest of trace
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("ce send"), 6, false).
		Return(common.NewPatchTraceResult(13, common.PatchSucceeded, 0)).
		Once()

	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).
		Once()

	b.start()
	fmt.Fprint(b, "My trace send")
	b.Success()
}

func TestJobDelayedTraceProcessingWithRejection(t *testing.T) {
	updateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	ignoreOptionalTouchJob(mockNetwork)

	receiveTraceInChunks := func() {
		// accept just 10 bytes
		mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("My trace s"), 0, false).
			Return(common.NewPatchTraceResult(10, common.PatchSucceeded, 1)).
			Once()

		// accept next 3 bytes
		mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("end"), 10, false).
			Return(common.NewPatchTraceResult(13, common.PatchSucceeded, 1)).
			Once()
	}

	respondNotYetCompleted := func() {
		// send back that job was not accepted twice
		mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
			Return(common.UpdateJobResult{
				State:             common.UpdateAcceptedButNotCompleted,
				NewUpdateInterval: 1,
			}).
			Twice()
	}

	requestResetContent := func() {
		mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
			Return(common.UpdateJobResult{
				State:             common.UpdateTraceValidationFailed,
				NewUpdateInterval: 1,
			}).
			Once()
	}

	acceptTrace := func() {
		mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
			Return(common.UpdateJobResult{
				State:             common.UpdateSucceeded,
				NewUpdateInterval: 1,
			}).Once()
	}

	// execute the following workflow
	// 1. Runner sends trace in chunks initially
	receiveTraceInChunks()

	// 2. Rails responds that trace was not yet accepted, Runner retries
	respondNotYetCompleted()

	// 3. Rails requests content reset
	requestResetContent()

	// 4. Runner resends all chunks
	receiveTraceInChunks()

	// 5. Rails responds that trace was not yet accepted, Runner retries
	respondNotYetCompleted()

	// 6. Rails finally accepts trace
	acceptTrace()

	b, err := newTestJobTrace(mockNetwork, jobConfig)
	require.NoError(t, err)

	b.maxTracePatchSize = 10

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
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("My tr"), 0, false).
		Return(common.NewPatchTraceResult(5, common.PatchSucceeded, 0)).Once()

	// expect next 5 bytes
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("ace s"), 5, false).
		Return(common.NewPatchTraceResult(10, common.PatchSucceeded, 0)).Once()

	// expect last 3 bytes
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("end"), 10, false).
		Return(common.NewPatchTraceResult(13, common.PatchSucceeded, 0)).Once()

	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once()

	b, err := newTestJobTrace(mockNetwork, jobConfig)
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

	b, err := newTestJobTrace(mockNetwork, jobConfig)
	require.NoError(t, err)

	ignoreOptionalTouchJob(mockNetwork)

	// fail job 5 times
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateFailed}).
		Run(func(args mock.Arguments) {
			// Ensure that short interval is used on retry to speed-up test
			b.setUpdateInterval(time.Microsecond)
		}).
		Times(5)

	// accept job
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once()

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
	mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("123456789\n"), 0, false).
		Return(common.NewPatchTraceResult(10, common.PatchSucceeded, 0)).Once().
		Run(func(args mock.Arguments) {
			wg.Done()
		})

	// wait for the final `UpdateJob` to be executed
	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, finalUpdateMatcher).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once()

	b, err := newTestJobTrace(mockNetwork, jobConfig)
	require.NoError(t, err)

	b.updateInterval = time.Millisecond * 10
	b.start()
	fmt.Fprint(b, "123456789\n")
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

	b, err := newTestJobTrace(mockNetwork, jobConfig)
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

func TestCancelingJobIncrementalUpdate(t *testing.T) {
	tests := map[string]struct {
		patchCanceling bool
	}{
		"patch doesn't return canceling": {
			patchCanceling: false,
		},
		"patch returns canceling": {
			patchCanceling: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			var wg sync.WaitGroup

			finalUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

			mockNetwork := new(common.MockNetwork)
			defer mockNetwork.AssertExpectations(t)

			wg.Add(4)

			mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("123456789\n"), 0, false).
				Return(common.PatchTraceResult{
					SentOffset:      10,
					CancelRequested: tt.patchCanceling,
					State:           common.PatchSucceeded,
				}).
				Run(func(args mock.Arguments) {
					wg.Done()
				}).
				Once()

			keepAliveUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Running, "")
			mockNetwork.On("UpdateJob", jobConfig, jobCredentials, keepAliveUpdateMatcher).
				Return(common.UpdateJobResult{State: common.UpdateSucceeded, CancelRequested: true}).
				Run(func(args mock.Arguments) {
					wg.Done()
				}).Twice()

			// When `UpdateJob` requested cancelation we continue to send the trace.
			mockNetwork.On("PatchTrace", jobConfig, jobCredentials, []byte("987654321\n"), 10, false).
				Return(common.PatchTraceResult{SentOffset: 20, CancelRequested: true, State: common.PatchSucceeded}).
				Run(func(args mock.Arguments) {
					wg.Done()
				}).
				Once()

			// We might get additional touch jobs calls we can ignore them.
			mockNetwork.On("UpdateJob", jobConfig, jobCredentials, keepAliveUpdateMatcher).
				Return(common.UpdateJobResult{State: common.UpdateSucceeded, CancelRequested: true}).
				Maybe()

			// Wait for the final `UpdateJob` to be executed
			mockNetwork.On("UpdateJob", jobConfig, jobCredentials, finalUpdateMatcher).
				Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Once()

			b, err := newTestJobTrace(mockNetwork, jobConfig)
			require.NoError(t, err)

			b.updateInterval = time.Millisecond * 10
			b.maxTracePatchSize = 10
			b.forceSendInterval = time.Millisecond
			b.start()
			fmt.Fprint(b, "123456789\n987654321\n")
			wg.Wait()
			b.Success()
		})
	}
}

func TestUpdateIntervalChanges(t *testing.T) {
	testTrace := "Test trace\n"
	touchUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Running, "")
	finalUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, "")

	traceUpdateIntervalDefault := 10 * time.Millisecond

	tests := map[string]struct {
		initialUpdateInterval    time.Duration
		requestedUpdateInterval  int
		patchStateResponse       common.PatchState
		updateStateResponse      common.UpdateState
		afterPatchUpdateInterval time.Duration
		afterTouchUpdateInterval time.Duration
		afterFinalUpdateInterval time.Duration
	}{
		"negative interval requested": {
			initialUpdateInterval:    traceUpdateIntervalDefault,
			requestedUpdateInterval:  -10,
			patchStateResponse:       common.PatchSucceeded,
			updateStateResponse:      common.UpdateSucceeded,
			afterPatchUpdateInterval: traceUpdateIntervalDefault,
			afterTouchUpdateInterval: traceUpdateIntervalDefault,
			// final-update resets interval to default
			afterFinalUpdateInterval: common.DefaultUpdateInterval,
		},
		"zero interval requested": {
			initialUpdateInterval:    traceUpdateIntervalDefault,
			requestedUpdateInterval:  0,
			patchStateResponse:       common.PatchSucceeded,
			updateStateResponse:      common.UpdateSucceeded,
			afterPatchUpdateInterval: traceUpdateIntervalDefault,
			afterTouchUpdateInterval: traceUpdateIntervalDefault,
			// final-update resets interval to default
			afterFinalUpdateInterval: common.DefaultUpdateInterval,
		},
		"positive interval requested": {
			initialUpdateInterval:    traceUpdateIntervalDefault,
			requestedUpdateInterval:  10,
			patchStateResponse:       common.PatchSucceeded,
			updateStateResponse:      common.UpdateSucceeded,
			afterPatchUpdateInterval: 10 * time.Second,
			afterTouchUpdateInterval: 10 * time.Second,
			afterFinalUpdateInterval: 10 * time.Second,
		},
		"positive interval applied on a failure": {
			initialUpdateInterval:   traceUpdateIntervalDefault,
			requestedUpdateInterval: 10,
			// We use *Abort as it exits immediately,
			// instead of retrying, but still does update interval
			patchStateResponse:       common.PatchAbort,
			updateStateResponse:      common.UpdateAbort,
			afterPatchUpdateInterval: 10 * time.Second,
			afterTouchUpdateInterval: 10 * time.Second,
			afterFinalUpdateInterval: 10 * time.Second,
		},
		"over-limit interval requested": {
			initialUpdateInterval:    traceUpdateIntervalDefault,
			requestedUpdateInterval:  int(common.MaxUpdateInterval.Seconds()) + 10,
			patchStateResponse:       common.PatchSucceeded,
			updateStateResponse:      common.UpdateSucceeded,
			afterPatchUpdateInterval: common.MaxUpdateInterval,
			afterTouchUpdateInterval: common.MaxUpdateInterval,
			afterFinalUpdateInterval: common.MaxUpdateInterval,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Run("sendPatch", func(t *testing.T) {
				client := new(common.MockNetwork)
				defer client.AssertExpectations(t)

				waitForPatch := new(sync.WaitGroup)
				waitForPatch.Add(1)

				client.On("PatchTrace", jobConfig, jobCredentials, []byte(testTrace), 0, mock.Anything).
					Return(common.NewPatchTraceResult(
						len(testTrace),
						tt.patchStateResponse,
						tt.requestedUpdateInterval,
					)).
					Run(func(_ mock.Arguments) {
						waitForPatch.Done()
					}).
					Once()

				if tt.patchStateResponse != common.PatchSucceeded {
					// Ensure that if we test failure `PatchTrace` gets finally accepted
					client.On("PatchTrace", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
						Return(common.NewPatchTraceResult(
							len(testTrace),
							common.PatchSucceeded,
							0,
						)).Once()
				}

				// Ignore all subequent touch jobs
				ignoreOptionalTouchJob(client)

				client.On("UpdateJob", jobConfig, jobCredentials, finalUpdateMatcher).
					Return(common.UpdateJobResult{State: common.UpdateSucceeded}).
					Once()

				trace, err := newTestJobTrace(client, jobConfig)
				require.NoError(t, err)

				trace.updateInterval = tt.initialUpdateInterval

				trace.start()
				assert.Equal(t, tt.initialUpdateInterval, trace.getUpdateInterval())

				_, err = fmt.Fprint(trace, testTrace)
				require.NoError(t, err)

				waitForPatch.Wait()

				// we need to wait a little to ensure that `PatchTrace` response was processed
				assert.Eventually(
					t,
					func() bool { return tt.afterPatchUpdateInterval == trace.getUpdateInterval() },
					time.Second,
					10*time.Millisecond,
				)

				trace.Success()
			})

			t.Run("touchJob", func(t *testing.T) {
				client := new(common.MockNetwork)
				defer client.AssertExpectations(t)

				waitForTouchJob := new(sync.WaitGroup)
				waitForTouchJob.Add(1)

				client.On("UpdateJob", jobConfig, jobCredentials, touchUpdateMatcher).
					Return(common.UpdateJobResult{
						State:             tt.updateStateResponse,
						NewUpdateInterval: time.Duration(tt.requestedUpdateInterval) * time.Second,
					}).
					Run(func(_ mock.Arguments) {
						waitForTouchJob.Done()
					}).
					Once()

				client.On("UpdateJob", jobConfig, jobCredentials, finalUpdateMatcher).
					Return(common.UpdateJobResult{State: common.UpdateSucceeded}).
					Once()

				trace, err := newTestJobTrace(client, jobConfig)
				require.NoError(t, err)

				trace.updateInterval = tt.initialUpdateInterval

				trace.start()
				assert.Equal(t, tt.initialUpdateInterval, trace.getUpdateInterval())

				waitForTouchJob.Wait()

				// we need to wait a little to ensure that `UpdateJob` response was processed
				assert.Eventually(
					t,
					func() bool { return tt.afterTouchUpdateInterval == trace.getUpdateInterval() },
					time.Second,
					10*time.Millisecond,
				)

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
						State:             tt.updateStateResponse,
						NewUpdateInterval: time.Duration(tt.requestedUpdateInterval) * time.Second,
					}).
					Run(func(_ mock.Arguments) {
						waitForFinalUpdate.Done()
					}).
					Once()

				trace, err := newTestJobTrace(client, jobConfig)
				require.NoError(t, err)

				trace.updateInterval = tt.initialUpdateInterval

				trace.start()
				assert.Equal(t, tt.initialUpdateInterval, trace.getUpdateInterval())
				trace.Success()

				waitForFinalUpdate.Wait()
				assert.Equal(t, tt.afterFinalUpdateInterval, trace.getUpdateInterval())
			})
		})
	}
}

// TestJobChecksum validates a completness of crc32 checksum as sent in
// `UpdateJob`. It ensures that checksum engine generates a checksum of a
// masked content that is send in a chunks to Rails
func TestJobChecksum(t *testing.T) {
	maskedValues := common.MaskOptions{Phrases: []string{"masked"}}
	traceMessage := "This string should be masked $$$$"
	traceMaskedMessage := "This string should be [MASKED] $$$$"

	expectedJobInfo := common.UpdateJobInfo{
		ID:    -1,
		State: "success",
		Output: common.JobTraceOutput{
			Checksum: "crc32:0fc72945", // this is a checksum of `traceMaskedMessage`
			Bytesize: 35,
		},
	}

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	// 22 is an offset of a space before `[MASKED]`
	mockNetwork.On("PatchTrace", mock.Anything, mock.Anything, []byte(traceMaskedMessage[0:22]), 0, false).
		Return(common.NewPatchTraceResult(22, common.PatchSucceeded, 0)).Once()

	mockNetwork.On("PatchTrace", mock.Anything, mock.Anything, []byte(traceMaskedMessage[22:]), 22, false).
		Return(common.NewPatchTraceResult(len(traceMaskedMessage), common.PatchSucceeded, 0)).Once()

	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, expectedJobInfo).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded})

	jobTrace, err := newTestJobTrace(mockNetwork, jobConfig)
	require.NoError(t, err)

	jobTrace.maxTracePatchSize = 22
	jobTrace.SetMasked(maskedValues)
	jobTrace.start()

	_, err = jobTrace.Write([]byte(traceMessage))
	require.NoError(t, err)
	jobTrace.Success()
}

func TestJobBytesize(t *testing.T) {
	maskedValues := common.MaskOptions{Phrases: []string{"secret"}}
	traceMessage := "Build trace with secret and multi-byte ü character"
	traceMaskedMessage := "Build trace with [MASKED] and multi-byte ü character"

	expectedJobInfo := common.UpdateJobInfo{
		ID:    -1,
		State: "success",
		Output: common.JobTraceOutput{
			Checksum: "crc32:984a6af7",
			Bytesize: 53,
		},
	}

	mockNetwork := new(common.MockNetwork)
	defer mockNetwork.AssertExpectations(t)

	mockNetwork.On("PatchTrace", mock.Anything, mock.Anything, []byte(traceMaskedMessage), 0, false).
		Return(common.NewPatchTraceResult(len(traceMaskedMessage), common.PatchSucceeded, 0)).Once()

	mockNetwork.On("UpdateJob", jobConfig, jobCredentials, expectedJobInfo).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded})

	jobTrace, err := newTestJobTrace(mockNetwork, jobConfig)
	require.NoError(t, err)

	jobTrace.maxTracePatchSize = 100
	jobTrace.SetMasked(maskedValues)
	jobTrace.start()

	_, err = jobTrace.Write([]byte(traceMessage))
	require.NoError(t, err)
	jobTrace.Success()
}

func TestDynamicForceSendUpdate(t *testing.T) {
	intervals := map[time.Duration]time.Duration{
		common.DefaultUpdateInterval: common.MinTraceForceSendInterval,
		5 * time.Second:              common.MinTraceForceSendInterval,
		time.Minute:                  time.Minute * common.TraceForceSendUpdateIntervalMultiplier,
		common.MaxUpdateInterval:     common.MaxTraceForceSendInterval,
		common.MaxUpdateInterval * 2: common.MaxTraceForceSendInterval,
	}

	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("FF_USE_DYNAMIC_TRACE_FORCE_SEND_INTERVAL=%v", enabled), func(t *testing.T) {
			config := common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					FeatureFlags: map[string]bool{
						featureflags.UseDynamicTraceForceSendInterval: enabled,
					},
				},
			}

			trace, err := newJobTrace(nil, config, jobCredentials)
			require.NoError(t, err)

			for updateInterval, forceInterval := range intervals {
				t.Run(fmt.Sprintf("%v => %v", updateInterval, forceInterval), func(t *testing.T) {
					trace.setUpdateInterval(updateInterval)

					if enabled {
						assert.Equal(t, forceInterval, trace.forceSendInterval)
					} else {
						assert.Equal(t, common.MinTraceForceSendInterval, trace.forceSendInterval)
					}
				})
			}
		})
	}
}

package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var (
	jobConfig      = common.RunnerConfig{}
	jobCredentials = &common.JobCredentials{ID: -1}
	jobOutputLimit = common.RunnerConfig{OutputLimit: 1}

	noTrace *string
)

func matchJobState(jobInfo common.UpdateJobInfo, id int, state common.JobState, failureReason common.JobFailureReason) bool {
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

func generateJobInfoMatcher(id int, state common.JobState, trace *string, failureReason common.JobFailureReason) interface{} {
	return mock.MatchedBy(func(jobInfo common.UpdateJobInfo) bool {
		if jobInfo.Trace == nil && trace != nil {
			return false
		}
		if jobInfo.Trace != nil && trace == nil {
			return false
		}
		if jobInfo.Trace != nil && trace != nil && *jobInfo.Trace != *trace {
			return false
		}
		return matchJobState(jobInfo, id, state, failureReason)
	})
}

func generateJobInfoMatcherWithAnyTrace(id int, state common.JobState, failureReason common.JobFailureReason) interface{} {
	return mock.MatchedBy(func(jobInfo common.UpdateJobInfo) bool {
		return matchJobState(jobInfo, id, state, failureReason)
	})
}

func TestJobTraceUpdateSucceeded(t *testing.T) {
	traceMessage := "test content"
	patchTraceMatcher := mock.MatchedBy(func(tracePatch common.JobTracePatch) bool {
		return tracePatch.Offset() == 0 && string(tracePatch.Patch()) == traceMessage
	})

	tests := []struct {
		name     string
		jobState common.JobState
	}{
		{name: "Success", jobState: common.Success},
		{name: "Fail", jobState: common.Failed},
	}

	for idx, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var wg sync.WaitGroup
			jobCredentials := &common.JobCredentials{ID: idx}

			net := new(common.MockNetwork)

			net.On("PatchTrace", jobConfig, jobCredentials, patchTraceMatcher).Return(common.UpdateSucceeded).Run(func(_ mock.Arguments) { wg.Done() })

			var expectedFailureReason common.JobFailureReason
			switch test.jobState {
			case common.Success:
				expectedFailureReason = common.NoneFailure
			case common.Failed:
				expectedFailureReason = common.ScriptFailure
			}
			updateMatcher := generateJobInfoMatcher(idx, test.jobState, nil, expectedFailureReason)
			net.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).Return(common.UpdateSucceeded)

			b := newJobTrace(net, jobConfig, jobCredentials)
			// speed up execution time
			b.updateInterval = 10 * time.Millisecond
			wg.Add(1)

			b.start()
			fmt.Fprint(b, traceMessage)
			wg.Wait()

			switch test.jobState {
			case common.Success:
				b.Success()
			case common.Failed:
				b.Fail(errors.New("test"), common.ScriptFailure)
			}

			net.AssertExpectations(t)
		})
	}
}

func TestIgnoreStatusChange(t *testing.T) {
	jobInfoMatcher := generateJobInfoMatcherWithAnyTrace(jobCredentials.ID, common.Success, common.NoneFailure)

	net := new(common.MockNetwork)
	net.On("UpdateJob", jobConfig, jobCredentials, jobInfoMatcher).Return(common.UpdateSucceeded)

	b := newJobTrace(net, jobConfig, jobCredentials)
	// prevent any UpdateJob before `b.Success()` call
	b.updateInterval = 25 * time.Second

	b.start()
	b.Success()
	b.Fail(errors.New("test"), "script_failure")

	net.AssertExpectations(t)
}

func TestJobAbort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	keepAliveUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Running, nil, "")
	updateMatcher := generateJobInfoMatcherWithAnyTrace(jobCredentials.ID, common.Success, common.NoneFailure)

	net := new(common.MockNetwork)
	net.On("UpdateJob", jobConfig, jobCredentials, keepAliveUpdateMatcher).Return(common.UpdateAbort)
	net.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).Return(common.UpdateAbort)

	b := newJobTrace(net, jobConfig, jobCredentials)
	b.updateInterval = 0
	b.SetCancelFunc(cancel)

	b.start()
	assert.NotNil(t, <-ctx.Done(), "should abort the job")
	b.Success()

	net.AssertExpectations(t)
}

func TestJobOutputLimit(t *testing.T) {
	assert := assert.New(t)
	traceMessage := "abcde"

	net := new(common.MockNetwork)

	b := newJobTrace(net, jobOutputLimit, jobCredentials)
	// prevent any UpdateJob before `b.Success()` call
	b.updateInterval = 25 * time.Second

	updateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Success, nil, common.NoneFailure)

	receivedTrace := bytes.NewBuffer([]byte{})
	net.On("PatchTrace", jobOutputLimit, jobCredentials, mock.AnythingOfType("*network.tracePatch")).Return(common.UpdateSucceeded).Run(func(args mock.Arguments) {
		if patch, ok := args.Get(2).(*tracePatch); ok {
			receivedTrace.Write(patch.Patch())
		} else {
			assert.FailNow("Unexpected type on PatchTrace tracePatch parameter")
		}
	})
	net.On("UpdateJob", jobOutputLimit, jobCredentials, updateMatcher).Return(common.UpdateSucceeded).Once()

	b.start()
	// Write 5k to the buffer
	for i := 0; i < 1024; i++ {
		fmt.Fprint(b, traceMessage)
	}
	b.Success()

	expectedLogLimitExceededMsg := b.limitExceededMessage()
	bytesLimit := b.bytesLimit + len(expectedLogLimitExceededMsg)
	trace := receivedTrace.String()
	traceSize := len(trace)

	assert.Equal(bytesLimit, traceSize, "the trace should be exaclty %v bytes", bytesLimit)
	assert.Contains(trace, traceMessage)
	assert.Contains(trace, expectedLogLimitExceededMsg)

	net.AssertExpectations(t)
}

func TestJobFinishRetry(t *testing.T) {
	updateMatcher := generateJobInfoMatcherWithAnyTrace(jobCredentials.ID, common.Success, common.NoneFailure)

	net := new(common.MockNetwork)
	net.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).Return(common.UpdateFailed).Times(5)
	net.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).Return(common.UpdateSucceeded).Once()

	b := newJobTrace(net, jobConfig, jobCredentials)
	b.finishRetryInterval = time.Microsecond

	b.start()
	b.Success()

	net.AssertExpectations(t)
}

func TestJobForceSend(t *testing.T) {
	var wg sync.WaitGroup
	traceMessage := "test content"
	firstPatchMatcher := mock.MatchedBy(func(tracePatch common.JobTracePatch) bool {
		return tracePatch.Offset() == 0 && string(tracePatch.Patch()) == traceMessage
	})
	keepAliveUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Running, nil, "")
	updateMatcher := generateJobInfoMatcherWithAnyTrace(jobCredentials.ID, common.Success, common.NoneFailure)

	wg.Add(1)

	net := new(common.MockNetwork)
	net.On("PatchTrace", jobConfig, jobCredentials, firstPatchMatcher).Return(common.UpdateSucceeded).Once()
	net.On("UpdateJob", jobConfig, jobCredentials, keepAliveUpdateMatcher).Run(func(_ mock.Arguments) { wg.Done() }).Return(common.UpdateSucceeded)
	net.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).Return(common.UpdateSucceeded).Once()
	defer net.AssertExpectations(t)

	b := newJobTrace(net, jobConfig, jobCredentials)

	b.updateInterval = 500 * time.Microsecond
	b.forceSendInterval = 4 * b.updateInterval
	b.start()
	defer b.Success()

	fmt.Fprint(b, traceMessage)
	wg.Wait()
}

func runOnHijackedLogrusOutput(t *testing.T, handler func(t *testing.T, output *bytes.Buffer)) {
	oldOutput := logrus.StandardLogger().Out
	defer func() { logrus.StandardLogger().Out = oldOutput }()

	buf := bytes.NewBuffer([]byte{})
	logrus.StandardLogger().Out = buf

	handler(t, buf)
}

func TestPatchTraceRangeMismatch(t *testing.T) {
	runOnHijackedLogrusOutput(t, func(t *testing.T, output *bytes.Buffer) {
		var wg sync.WaitGroup

		traceMessage := "test content"

		wg.Add(1)

		updateTraceOffsetFn := func(args mock.Arguments) {
			patch, ok := args.Get(2).(*tracePatch)
			require.True(t, ok, "Argument needs to be a proper *tracePatch instance")
			patch.SetNewOffset(15)
		}

		fullUpdateMatcher := generateJobInfoMatcher(jobCredentials.ID, common.Running, &traceMessage, common.NoneFailure)

		net := new(common.MockNetwork)
		net.On("PatchTrace", jobConfig, jobCredentials, mock.Anything).Run(updateTraceOffsetFn).Return(common.UpdateRangeMismatch).Once()
		net.On("UpdateJob", jobConfig, jobCredentials, fullUpdateMatcher).Run(func(_ mock.Arguments) { wg.Done() }).Return(common.UpdateSucceeded).Once()
		net.On("UpdateJob", jobConfig, jobCredentials, mock.Anything).Return(common.UpdateSucceeded).Once()
		defer net.AssertExpectations(t)

		b := newJobTrace(net, jobConfig, jobCredentials)

		b.updateInterval = 500 * time.Microsecond

		b.start()
		defer b.Success()

		fmt.Fprint(b, traceMessage)

		wg.Wait()
		assert.Regexp(t, "Full job update is needed", output.String())
	})
}

func TestPatchTraceDoubleRangeMismatch(t *testing.T) {
	runOnHijackedLogrusOutput(t, func(t *testing.T, output *bytes.Buffer) {
		var wg sync.WaitGroup

		traceMessage := "test content"

		wg.Add(1)

		net := new(common.MockNetwork)
		net.On("PatchTrace", jobConfig, jobCredentials, mock.Anything).Return(common.UpdateRangeMismatch).Once()
		net.On("PatchTrace", jobConfig, jobCredentials, mock.Anything).Run(func(_ mock.Arguments) { wg.Done() }).Return(common.UpdateRangeMismatch).Once()
		net.On("PatchTrace", jobConfig, jobCredentials, mock.Anything).Return(common.UpdateSucceeded).Once()
		net.On("UpdateJob", jobConfig, jobCredentials, mock.Anything).Return(common.UpdateSucceeded).Once()
		defer net.AssertExpectations(t)

		b := newJobTrace(net, jobConfig, jobCredentials)

		b.updateInterval = 500 * time.Microsecond

		b.start()
		defer b.Success()

		fmt.Fprint(b, traceMessage)

		wg.Wait()
		assert.Regexp(t, "Resending trace patch due to range mismatch", output.String())
		assert.Regexp(t, "failed due to range mismatch", output.String())
	})
}

func TestFinalUpdateWithTrace(t *testing.T) {
	var wg sync.WaitGroup

	traceMessage := "test content"

	wg.Add(1)
	net := new(common.MockNetwork)
	b := newJobTrace(net, jobConfig, jobCredentials)

	finalUpdateMatcher := mock.MatchedBy(func(jobInfo common.UpdateJobInfo) bool {
		return *jobInfo.Trace == (traceMessage + traceMessage)
	})

	net.On("PatchTrace", jobConfig, jobCredentials, mock.Anything).Return(common.UpdateSucceeded).Once().Run(func(_ mock.Arguments) {
		b.updateInterval = 10 * time.Second
		fmt.Fprint(b, traceMessage)
		go b.Success()
	})
	net.On("PatchTrace", jobConfig, jobCredentials, mock.Anything).Return(common.UpdateFailed).Once()
	net.On("UpdateJob", jobConfig, jobCredentials, finalUpdateMatcher).Return(common.UpdateSucceeded).Once().Run(func(_ mock.Arguments) {
		wg.Done()
	})
	defer net.AssertExpectations(t)

	b.updateInterval = 500 * time.Microsecond
	b.start()

	fmt.Fprint(b, traceMessage)

	wg.Wait()
}

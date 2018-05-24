package network

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

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
				expectedFailureReason = common.JobFailureReason("")
			case common.Failed:
				expectedFailureReason = common.JobFailureReason("script_failure")
			}
			updateMatcher := generateJobInfoMatcher(idx, test.jobState, &traceMessage, expectedFailureReason)
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
				b.Fail(errors.New("test"), "script_failure")
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

	updateMatcher := generateJobInfoMatcherWithAnyTrace(jobCredentials.ID, common.Success, common.NoneFailure)

	net := new(common.MockNetwork)
	net.On("PatchTrace", jobConfig, jobCredentials, mock.AnythingOfType("*network.tracePatch")).Return(common.UpdateAbort)
	net.On("UpdateJob", jobConfig, jobCredentials, updateMatcher).Return(common.UpdateAbort)

	b := newJobTrace(net, jobConfig, jobCredentials)
	// force immediate call to `UpdateJob`
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

	updateMatcher := generateJobInfoMatcherWithAnyTrace(jobCredentials.ID, common.Success, common.NoneFailure)
	net.On("UpdateJob", jobOutputLimit, jobCredentials, updateMatcher).Return(common.UpdateSucceeded).Run(func(args mock.Arguments) {
		if updateInfo, ok := args.Get(2).(common.UpdateJobInfo); ok {
			trace := updateInfo.Trace

			expectedLogLimitExceededMsg := b.limitExceededMessage()
			bytesLimit := b.bytesLimit + len(expectedLogLimitExceededMsg)
			traceSize := len(*trace)

			assert.Equal(bytesLimit, traceSize, "the trace should be exaclty %v bytes", bytesLimit)
			assert.Contains(*trace, traceMessage)
			assert.Contains(*trace, b.limitExceededMessage())
		} else {
			assert.FailNow("Unexpected type on UpdateJob jobInfo parameter")
		}
	})

	b.start()
	// Write 5k to the buffer
	for i := 0; i < 1024; i++ {
		fmt.Fprint(b, traceMessage)
	}
	b.Success()

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
	nextEmptyPatchMatcher := mock.MatchedBy(func(tracePatch common.JobTracePatch) bool {
		return tracePatch.Offset() == len(traceMessage) && string(tracePatch.Patch()) == ""
	})
	updateMatcher := generateJobInfoMatcherWithAnyTrace(jobCredentials.ID, common.Success, common.NoneFailure)

	wg.Add(1)

	net := new(common.MockNetwork)
	net.On("PatchTrace", jobConfig, jobCredentials, firstPatchMatcher).Return(common.UpdateSucceeded).Once()
	net.On("PatchTrace", jobConfig, jobCredentials, nextEmptyPatchMatcher).Return(common.UpdateSucceeded).Run(func(_ mock.Arguments) { wg.Done() })
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

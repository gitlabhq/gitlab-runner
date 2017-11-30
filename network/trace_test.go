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

func TestJobTraceUpdateSucceeded(t *testing.T) {
	traceMessage := "test content"
	traceMatcher := mock.MatchedBy(func(trace *string) bool { return trace != nil && *trace == traceMessage })
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
			net.On("UpdateJob", jobConfig, jobCredentials, idx, common.Running, noTrace).Return(common.UpdateSucceeded).Once()
			net.On("PatchTrace", jobConfig, jobCredentials, patchTraceMatcher).Return(common.UpdateSucceeded).Run(func(_ mock.Arguments) { wg.Done() })
			net.On("UpdateJob", jobConfig, jobCredentials, idx, test.jobState, traceMatcher).Return(common.UpdateSucceeded)

			b := newJobTrace(net, jobConfig, jobCredentials)
			// speed up execution time
			b.traceUpdateInterval = 10 * time.Millisecond
			wg.Add(1)

			b.start()
			fmt.Fprint(b, traceMessage)
			wg.Wait()

			switch test.jobState {
			case common.Success:
				b.Success()
			case common.Failed:
				b.Fail(errors.New("test"))
			}

			net.AssertExpectations(t)
		})
	}
}

func TestIgnoreStatusChange(t *testing.T) {
	net := new(common.MockNetwork)
	net.On("UpdateJob", jobConfig, jobCredentials, jobCredentials.ID, common.Success, mock.AnythingOfType("*string")).Return(common.UpdateSucceeded)

	b := newJobTrace(net, jobConfig, jobCredentials)
	// prevent any UpdateJob before `b.Success()` call
	b.traceUpdateInterval = 25 * time.Second

	b.start()
	b.Success()
	b.Fail(errors.New("test"))

	net.AssertExpectations(t)
}

func TestJobAbort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net := new(common.MockNetwork)
	net.On("UpdateJob", jobConfig, jobCredentials, jobCredentials.ID, common.Running, noTrace).Return(common.UpdateAbort)
	net.On("PatchTrace", jobConfig, jobCredentials, mock.AnythingOfType("*network.tracePatch")).Return(common.UpdateAbort)
	net.On("UpdateJob", jobConfig, jobCredentials, jobCredentials.ID, common.Success, mock.AnythingOfType("*string")).Return(common.UpdateAbort)

	b := newJobTrace(net, jobConfig, jobCredentials)
	// force immediate call to `UpdateJob`
	b.traceUpdateInterval = 0
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
	b.traceUpdateInterval = 25 * time.Second

	net.On("UpdateJob", jobOutputLimit, jobCredentials, jobCredentials.ID, common.Success, mock.AnythingOfType("*string")).Return(common.UpdateSucceeded).Run(func(args mock.Arguments) {
		if trace, ok := args.Get(4).(*string); ok {
			expectedLogLimitExceededMsg := b.jobLogLimitExceededMessage()
			bytesLimit := b.logLimitBytes + len(expectedLogLimitExceededMsg)
			traceSize := len(*trace)

			assert.Equal(bytesLimit, traceSize, "the trace should be exaclty %v bytes", bytesLimit)
			assert.Contains(*trace, traceMessage)
			assert.Contains(*trace, b.jobLogLimitExceededMessage())
		} else {
			assert.FailNow("Unexpected type on UpdateJob trace parameter")
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
	net := new(common.MockNetwork)
	net.On("UpdateJob", jobConfig, jobCredentials, jobCredentials.ID, common.Success, mock.AnythingOfType("*string")).Return(common.UpdateFailed).Times(5)
	net.On("UpdateJob", jobConfig, jobCredentials, jobCredentials.ID, common.Success, mock.AnythingOfType("*string")).Return(common.UpdateSucceeded).Once()

	b := newJobTrace(net, jobConfig, jobCredentials)
	b.traceFinishRetryInterval = time.Microsecond

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

	wg.Add(1)
	net := new(common.MockNetwork)
	net.On("UpdateJob", jobConfig, jobCredentials, jobCredentials.ID, common.Running, noTrace).Return(common.UpdateSucceeded).Once()
	net.On("PatchTrace", jobConfig, jobCredentials, firstPatchMatcher).Return(common.UpdateSucceeded).Once()
	net.On("PatchTrace", jobConfig, jobCredentials, nextEmptyPatchMatcher).Return(common.UpdateSucceeded).Run(func(_ mock.Arguments) { wg.Done() })
	net.On("UpdateJob", jobConfig, jobCredentials, jobCredentials.ID, common.Success, mock.AnythingOfType("*string")).Return(common.UpdateSucceeded).Once()
	defer net.AssertExpectations(t)

	b := newJobTrace(net, jobConfig, jobCredentials)

	b.traceUpdateInterval = 500 * time.Microsecond
	b.traceForceSendInterval = 4 * b.traceUpdateInterval
	b.start()
	defer b.Success()

	fmt.Fprint(b, traceMessage)
	wg.Wait()
}

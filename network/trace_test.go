package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const successID = 4
const cancelID = 5
const retryID = 6

var jobConfig = common.RunnerConfig{}
var jobOutputLimit = common.RunnerConfig{OutputLimit: 1}

type updateTraceNetwork struct {
	common.MockNetwork
	state         common.JobState
	trace         *string
	count         int
	failureReason common.JobFailureReason
}

func (m *updateTraceNetwork) UpdateJob(config common.RunnerConfig, jobCredentials *common.JobCredentials, id int, state common.JobState, trace *string, failureReason common.JobFailureReason) common.UpdateState {
	switch id {
	case successID:
		m.count++
		m.state = state
		m.failureReason = failureReason
		m.trace = trace
		return common.UpdateSucceeded

	case cancelID:
		m.count++
		return common.UpdateAbort

	case retryID:
		if state != common.Running {
			m.count++
			if m.count >= 5 {
				m.state = state
				m.failureReason = failureReason
				m.trace = trace
				return common.UpdateSucceeded
			}
		}
		return common.UpdateFailed

	default:
		return common.UpdateFailed
	}
}

func (m *updateTraceNetwork) PatchTrace(config common.RunnerConfig, jobCredentials *common.JobCredentials, tracePatch common.JobTracePatch) common.UpdateState {
	switch jobCredentials.ID {
	case successID:
		m.count++

		buffer := &bytes.Buffer{}
		if m.trace != nil {
			buffer = bytes.NewBufferString(*m.trace)
		}

		buffer.Write(tracePatch.Patch())

		newTrace := buffer.String()
		m.trace = &newTrace

		return common.UpdateSucceeded

	case cancelID:
		m.count++
		return common.UpdateAbort

	default:
		return common.UpdateFailed
	}
}

func TestJobTraceSuccess(t *testing.T) {
	u := &updateTraceNetwork{}
	jobCredentials := &common.JobCredentials{
		ID: successID,
	}
	b := newJobTrace(u, jobConfig, jobCredentials)
	b.start()
	fmt.Fprint(b, "test content")
	b.Success()
	assert.Equal(t, "test content", *u.trace)
	assert.Equal(t, common.Success, u.state)
}

func TestJobTraceFailure(t *testing.T) {
	u := &updateTraceNetwork{}
	jobCredentials := &common.JobCredentials{
		ID: successID,
	}
	b := newJobTrace(u, jobConfig, jobCredentials)
	b.start()
	fmt.Fprint(b, "test content")
	b.Fail(errors.New("test"), "script_failure")
	assert.Equal(t, "test content", *u.trace)
	assert.Equal(t, common.Failed, u.state)
	assert.Equal(t, common.ScriptFailure, u.failureReason)
}

func TestIgnoreStatusChange(t *testing.T) {
	u := &updateTraceNetwork{}
	jobCredentials := &common.JobCredentials{
		ID: successID,
	}
	b := newJobTrace(u, jobConfig, jobCredentials)
	b.start()
	b.Success()
	b.Fail(errors.New("test"))
	assert.Equal(t, common.Success, u.state)
}

func TestJobAbort(t *testing.T) {
	traceUpdateInterval = 0

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	u := &updateTraceNetwork{}
	jobCredentials := &common.JobCredentials{
		ID: cancelID,
	}
	b := newJobTrace(u, jobConfig, jobCredentials)
	b.SetCancelFunc(cancel)
	b.start()
	assert.NotNil(t, <-ctx.Done(), "should abort the job")
	b.Success()
}

func TestJobOutputLimit(t *testing.T) {
	traceUpdateInterval = 5 * time.Second

	u := &updateTraceNetwork{}
	jobCredentials := &common.JobCredentials{
		ID: successID,
	}
	b := newJobTrace(u, jobOutputLimit, jobCredentials)
	b.start()

	// Write 5k to the buffer
	for i := 0; i < 1024; i++ {
		fmt.Fprint(b, "abcde")
	}
	b.Success()

	t.Logf("Trace length: %d", len(*u.trace))

	assert.True(t, len(*u.trace) < 2000, "the output should be less than 2000 bytes")
	assert.Contains(t, *u.trace, "Job's log exceeded limit")
}

func TestJobFinishRetry(t *testing.T) {
	traceFinishRetryInterval = time.Microsecond

	u := &updateTraceNetwork{}
	jobCredentials := &common.JobCredentials{
		ID: retryID,
	}
	b := newJobTrace(u, jobOutputLimit, jobCredentials)
	b.start()
	b.Success()
	assert.Equal(t, 5, u.count, "it should retry a few times")
	assert.Equal(t, common.Success, u.state)
}

func TestJobForceSend(t *testing.T) {
	traceUpdateInterval = 0
	traceForceSendInterval = time.Minute

	u := &updateTraceNetwork{}
	jobCredentials := &common.JobCredentials{
		ID: successID,
	}
	b := newJobTrace(u, jobOutputLimit, jobCredentials)
	b.start()
	defer b.Success()

	fmt.Fprint(b, "test")

	started := time.Now()
	for time.Since(started) < time.Second {
		if u.trace != nil &&
			*u.trace == "test" {
			u.count = 0
			break
		}
	}
	assert.True(t, u.count == 0, "it didn't update the trace yet")
	assert.Equal(t, common.Running, u.state)

	traceForceSendInterval = 0

	started = time.Now()
	for time.Since(started) < time.Second {
		if u.count > 0 {
			break
		}
	}
	assert.True(t, u.count > 0, "it forcefully update trace more then once")
	assert.Equal(t, "test", *u.trace)
	assert.Equal(t, common.Running, u.state)
}

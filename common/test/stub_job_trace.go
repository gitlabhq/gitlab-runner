package common

import (
	"bytes"
	"context"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// StubJobTrace is a testing utility struct that implements common.JobTrace
// and saves tracing information inside a buffer.
type StubJobTrace struct {
	buffer     *bytes.Buffer
	ctx        context.Context
	cancelFunc context.CancelFunc
}

func (fjt *StubJobTrace) Success()                                              {}
func (fjt *StubJobTrace) Fail(err error, failureReason common.JobFailureReason) {}
func (fjt *StubJobTrace) SetFailuresCollector(fc common.FailuresCollector)      {}
func (fjt *StubJobTrace) IsStdout() bool                                        { return false }
func (fjt *StubJobTrace) Context() context.Context                              { return fjt.ctx }

// Cancel invokes internal Context canelFunc
func (fjt *StubJobTrace) Cancel() { fjt.cancelFunc() }

func (fjt *StubJobTrace) Write(p []byte) (n int, err error) {
	return fjt.buffer.Write(p)
}

// Read returns all the traced output
func (fjt *StubJobTrace) Read() string {
	return fjt.buffer.String()
}

// NewStubJobTrace provides a working StubJobTrace
func NewStubJobTrace() *StubJobTrace {
	ctx, cancel := context.WithCancel(context.Background())

	return &StubJobTrace{
		buffer:     bytes.NewBuffer([]byte{}),
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

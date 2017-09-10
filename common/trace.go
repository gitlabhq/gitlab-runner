package common

import (
	"context"
	"io"
	"os"
)

type Trace struct {
	Writer     io.Writer
	CancelFunc context.CancelFunc
}

func (s *Trace) Write(p []byte) (n int, err error) {
	if s.Writer == nil {
		return 0, os.ErrInvalid
	}
	return s.Writer.Write(p)
}

func (s *Trace) Success() {
}

func (s *Trace) Fail(err error, failureReason JobFailureReason) {
}

func (s *Trace) SetCancelFunc(cancelFunc context.CancelFunc) {
	s.CancelFunc = cancelFunc
}

func (s *Trace) IsStdout() bool {
	return true
}

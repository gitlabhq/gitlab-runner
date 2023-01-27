package common

import (
	"context"
	"io"
	"os"
	"sync"
)

type Trace struct {
	Writer     io.Writer
	cancelFunc context.CancelFunc
	abortFunc  context.CancelFunc
	mutex      sync.Mutex
}

type masker interface {
	SetMasked(MaskOptions)
}

type JobFailureData struct {
	Reason   JobFailureReason
	ExitCode int
}

func (s *Trace) Write(p []byte) (n int, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.Writer == nil {
		return 0, os.ErrInvalid
	}
	return s.Writer.Write(p)
}

func (s *Trace) SetMasked(opts MaskOptions) {
	masker, ok := s.Writer.(masker)
	if ok {
		masker.SetMasked(opts)
	}
}

func (s *Trace) SetDebugModeEnabled(_ bool) {
}

func (s *Trace) Success() {
}

func (s *Trace) Fail(err error, failureData JobFailureData) {
}

func (s *Trace) SetCancelFunc(cancelFunc context.CancelFunc) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.cancelFunc = cancelFunc
}

func (s *Trace) Cancel() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cancelFunc == nil {
		return false
	}

	s.cancelFunc()
	return true
}

func (s *Trace) SetAbortFunc(abortFunc context.CancelFunc) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.abortFunc = abortFunc
}

func (s *Trace) Abort() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.abortFunc == nil {
		return false
	}

	// Abort always have much higher importance than Cancel
	// as abort interrupts the execution
	s.cancelFunc = nil
	s.abortFunc()
	return true
}

func (s *Trace) SetFailuresCollector(fc FailuresCollector) {}

func (s *Trace) IsStdout() bool {
	return true
}

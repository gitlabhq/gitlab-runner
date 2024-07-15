//go:build !integration

package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

const logPauseMarker = `{"script": "some/script"}`
const logResumeMarker = `{"command_exit_code": 0}`

func TestEntrypointLogForwarder(t *testing.T) {
	t.Run("forward, pause, resume", func(t *testing.T) {
		gatherer := newFakeLogGatherer()
		sink := newFakeSink()
		ctx, cancel := context.WithCancel(context.Background())
		runnerErr := make(chan error)

		lf := &entrypointLogForwarder{
			LogGatherer: gatherer.Emitter,
			LogSink:     sink,
		}

		go lf.Run(ctx, "some container", runnerErr)

		gatherer.EmitLogLn("1")
		gatherer.EmitLogLn("2")

		gatherer.EmitLogLn(logPauseMarker)

		gatherer.EmitLogLn("3")
		gatherer.EmitLogLn("4")

		gatherer.EmitLogLn(logResumeMarker)

		gatherer.EmitLogLn("5")

		cancel()
		<-ctx.Done()

		err := <-runnerErr
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, "1\n2\n5\n", sink.String())
	})

	t.Run("gatherer errors", func(t *testing.T) {
		ctx := context.Background()
		gatherer := newFakeLogGatherer()
		runnerErr := make(chan error)

		lf := &entrypointLogForwarder{
			LogGatherer: gatherer.Emitter,
			LogSink:     newNopCloser(io.Discard),
		}

		go lf.Run(ctx, "some container", runnerErr)

		gatherer.EmitLogLn("blupp")
		gatherer.EmitErr(fmt.Errorf("some random error"))

		err := <-runnerErr
		assert.ErrorContains(t, err, "some random error")
	})

	t.Run("multiple writes, one line", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		gatherer := newFakeLogGatherer()
		sink := newFakeSink()
		runnerErr := make(chan error)

		lf := &entrypointLogForwarder{
			LogGatherer: gatherer.Emitter,
			LogSink:     sink,
		}

		go lf.Run(ctx, "some container", runnerErr)

		gatherer.EmitLog("cat")
		gatherer.EmitLog("badger")
		gatherer.EmitLogLn("axolotl")

		cancel()
		<-ctx.Done()

		err := <-runnerErr
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, "catbadgeraxolotl\n", sink.String())

		callCount := sink.CallCount()
		assert.Equal(t, 1, callCount, "expected write on the sink to be called once, got called %d times", callCount)
	})

	t.Run("one write, multiple lines", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		gatherer := newFakeLogGatherer()
		sink := newFakeSink()
		runnerErr := make(chan error)

		lf := &entrypointLogForwarder{
			LogGatherer: gatherer.Emitter,
			LogSink:     sink,
		}

		go lf.Run(ctx, "some container", runnerErr)

		gatherer.EmitLogLn("cat\nbadger\naxolotl")

		cancel()
		<-ctx.Done()

		err := <-runnerErr
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, "cat\nbadger\naxolotl\n", sink.String())

		callCount := sink.CallCount()
		assert.Equal(t, 3, callCount, "expected write on the sink to be called 3 times, got called %d times", callCount)
	})

	t.Run("with timestamp", func(t *testing.T) {
		const someTimestamp = "2024-07-25T09:50:54.008163908Z"

		ctx, cancel := context.WithCancel(context.Background())
		gatherer := newFakeLogGatherer()
		sink := newFakeSink()
		runnerErr := make(chan error)

		lf := &entrypointLogForwarder{
			LogGatherer:   gatherer.Emitter,
			LogSink:       sink,
			WithTimestamp: true,
		}

		go lf.Run(ctx, "some container", runnerErr)

		gatherer.EmitLogLn("cow")
		gatherer.EmitLogLn(someTimestamp + " " + logPauseMarker)
		gatherer.EmitLogLn("no cow")
		// we are missing the space between the TS and the marker, thus the marker won't do anything
		gatherer.EmitLogLn(someTimestamp + logResumeMarker)
		gatherer.EmitLogLn("still no cow")
		gatherer.EmitLogLn(someTimestamp + " " + logResumeMarker)
		gatherer.EmitLogLn("sheep")

		cancel()
		<-ctx.Done()

		err := <-runnerErr
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, "cow\nsheep\n", sink.String())
	})

	t.Run("flushes on context cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		gatherer := newFakeLogGatherer()
		sink := newFakeSink()
		runnerErr := make(chan error)

		lf := &entrypointLogForwarder{
			LogGatherer:   gatherer.Emitter,
			LogSink:       sink,
			WithTimestamp: true,
		}

		go lf.Run(ctx, "some container", runnerErr)

		gatherer.EmitLogLn("armadillo")
		gatherer.EmitLog("cricket")

		cancel()
		<-ctx.Done()

		err := <-runnerErr
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, "armadillo\ncricket", sink.String())
	})
}

func newFakeLogGatherer() *fakeLogEmitter {
	return &fakeLogEmitter{
		logs: make(chan string),
		errs: make(chan error),
	}
}

type fakeLogEmitter struct {
	logs chan string
	errs chan error
}

func (f *fakeLogEmitter) EmitLog(log string) {
	f.logs <- log
}

func (f *fakeLogEmitter) EmitLogLn(log string) {
	f.EmitLog(log + "\n")
}

func (f *fakeLogEmitter) EmitErr(err error) {
	f.errs <- err
}

func (f *fakeLogEmitter) Emitter(ctx context.Context, container string, sink io.WriteCloser) error {
	for {
		select {
		case log := <-f.logs:
			_, _ = sink.Write([]byte(log))
		case err := <-f.errs:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func newNopCloser(w io.Writer) nopCloser {
	return nopCloser{w}
}

type sink struct {
	sync.RWMutex
	buffer         *bytes.Buffer
	closed         bool
	writeCallCount int
}

func (s *sink) Write(p []byte) (int, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		panic("sink: write after close!")
	}
	s.writeCallCount += 1
	return s.buffer.Write(p)
}

func (s *sink) Close() error {
	s.Lock()
	defer s.Unlock()
	s.closed = true
	return nil
}

func (s *sink) CallCount() int {
	s.RLock()
	defer s.RUnlock()
	return s.writeCallCount
}

func (s *sink) String() string {
	s.RLock()
	defer s.RUnlock()
	return s.buffer.String()
}

func newFakeSink() *sink {
	return &sink{
		buffer: &bytes.Buffer{},
	}
}

var _ io.WriteCloser = &sink{}

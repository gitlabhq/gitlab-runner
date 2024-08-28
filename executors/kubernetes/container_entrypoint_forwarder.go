package kubernetes

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

const containerLoggerTimeStampFormat = time.RFC3339Nano
const containerLoggerTimeStampLength = 30 // len("2024-07-25T09:50:54.008163908Z")

// entrypointLogForwarder implements an io.ReadCloser which gathers logs from the LogShipper and pushes that forward to the
// LogSink.
// If we see markers for starting or stopping a step, we pause / resume log forwarding, so that we only forward logs
// that are not captured through other means.
type entrypointLogForwarder struct {
	LogSink       io.WriteCloser
	LogGatherer   func(ctx context.Context, container string, sink io.WriteCloser) error
	WithTimestamp bool

	writeMutex sync.Mutex
	buffer     []byte
	paused     uint32
}

// Run starts the gathering / forwarding logs.
// This method usually runs in a go routine and keeps forwarding the logs, until the context is canceled.
func (lf *entrypointLogForwarder) Run(ctx context.Context, container string, errCh chan error) {
	defer lf.flush()
	err := lf.LogGatherer(ctx, container, lf)
	if errCh != nil {
		errCh <- err
	}
}

func (lf *entrypointLogForwarder) writeLine(p []byte) (int, error) {
	l := len(p)

	if ok, cmdStatus := lf.commandStatus(p); ok {
		// when we see the exit marker of a step, we should resume forwarding logs _after_ this very line/marker
		if cmdStatus.IsExited() {
			defer atomic.StoreUint32(&lf.paused, 0)
		}
		// when we see a step is being started, we pause and don't forward logs anymore
		if cmdStatus.BuildStage() != "" {
			atomic.StoreUint32(&lf.paused, 1)
		}
	}

	if atomic.LoadUint32(&lf.paused) == 1 {
		return l, nil
	}

	return lf.LogSink.Write(p)
}

// Write writes to the underlying io.WriteCloser iff currently not paused
// This Write splits the incoming bytes into lines, and calls write on the underlying writer once per line. We do this,
// so that we can inspect the lines individually, even though a write might happen with multiple lines in one go or
// multiple writes might happen for one single line.
func (lf *entrypointLogForwarder) Write(p []byte) (n int, err error) {
	lf.writeMutex.Lock()
	defer lf.writeMutex.Unlock()

	alreadyWritten := 0

	for i, b := range p {
		if b != '\n' {
			continue
		}

		_, err := lf.writeLine(append(lf.buffer, p[alreadyWritten:i+1]...))
		alreadyWritten = i + 1
		lf.buffer = []byte{}

		if err != nil {
			return 0, err
		}
	}

	if alreadyWritten < len(p) {
		rest := p[alreadyWritten:]
		lf.buffer = append(lf.buffer, rest...)
	}

	return len(p), nil
}

func (lf *entrypointLogForwarder) flush() error {
	lf.writeMutex.Lock()
	defer lf.writeMutex.Unlock()

	rest := lf.buffer
	if len(rest) >= 1 {
		_, err := lf.LogSink.Write(rest)
		return err
	}

	return nil
}

// Close unconditionally closes the underlying io.WriteCloser
func (lf *entrypointLogForwarder) Close() error {
	if err := lf.flush(); err != nil {
		return err
	}
	return lf.LogSink.Close()
}

// commandStatus inspects the current data if it's a [shells.StageCommandStatus]
// This is done, so we understand if the logs coming in are part of a step_command or "something else".
func (lf *entrypointLogForwarder) commandStatus(p []byte) (bool, shells.StageCommandStatus) {
	line := string(p)
	cmdStatus := shells.StageCommandStatus{}

	// check if the first part is a timestamp
	if lf.WithTimestamp {
		if len(line) < containerLoggerTimeStampLength {
			return false, cmdStatus
		}

		ts := line[:containerLoggerTimeStampLength]
		_, err := time.Parse(containerLoggerTimeStampFormat, ts)

		if err != nil {
			return false, cmdStatus
		}

		// when timestamped, the actual log line starts after the timestamp + a space
		line = line[containerLoggerTimeStampLength+1:]
	}

	ok := cmdStatus.TryUnmarshal(line)
	return ok, cmdStatus
}

var _ io.WriteCloser = &entrypointLogForwarder{}

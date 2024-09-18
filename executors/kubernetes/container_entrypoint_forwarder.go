package kubernetes

import (
	"io"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

const containerLoggerTimeStampFormat = "2006-01-02T15:04:05.999999999Z"

// entrypointLogForwarder implements an io.WriteCloser and forwards logs to the Sink.
// If we see markers for starting or stopping a step, we pause / resume log forwarding, so that we only forward logs
// that are not captured through other means.
type entrypointLogForwarder struct {
	Sink io.WriteCloser

	buffer []byte
	paused bool
}

func (lf *entrypointLogForwarder) writeLine(p []byte) error {
	cmdStatus, ok := lf.commandStatus(p)
	if ok {
		if cmdStatus.IsExited() {
			lf.paused = false
		} else if cmdStatus.BuildStage() != "" {
			lf.paused = true
		}
	}

	if lf.paused || ok {
		return nil
	}

	_, err := lf.Sink.Write(p)
	return err
}

// Write writes to the underlying io.Writer.
// This Write splits the incoming bytes into lines, and calls write on the underlying writer once per line. We do this,
// so that we can inspect the lines individually, even though a write might happen with multiple lines in one go or
// multiple writes might happen for one single line.
func (lf *entrypointLogForwarder) Write(p []byte) (int, error) {
	alreadyWritten := 0

	for i, b := range p {
		if b != '\n' {
			continue
		}

		err := lf.writeLine(append(lf.buffer, p[alreadyWritten:i+1]...))
		lf.buffer = nil
		if err != nil {
			return 0, err
		}

		alreadyWritten = i + 1
	}

	if alreadyWritten < len(p) {
		rest := p[alreadyWritten:]
		lf.buffer = append(lf.buffer, rest...)
	}

	return len(p), nil
}

func (lf *entrypointLogForwarder) flush() error {
	rest := lf.buffer
	if len(rest) >= 1 {
		_, err := lf.Sink.Write(rest)
		return err
	}

	return nil
}

// Close flushes the remaining buffer into Sink and closes it.
func (lf *entrypointLogForwarder) Close() error {
	if err := lf.flush(); err != nil {
		defer lf.Sink.Close()
		return err
	}

	return lf.Sink.Close()
}

// commandStatus inspects the current data if it's a [shells.StageCommandStatus]
// This is done, so we understand if the logs coming in are part of a step_command or "something else".
func (lf *entrypointLogForwarder) commandStatus(p []byte) (shells.StageCommandStatus, bool) {
	cmdStatus := shells.StageCommandStatus{}

	// check if the first part resembles a timestamp
	if len(p) < len(containerLoggerTimeStampFormat) ||
		p[len(containerLoggerTimeStampFormat)] != ' ' {
		return cmdStatus, false
	}

	line := string(p)
	ts := line[:len(containerLoggerTimeStampFormat)]
	_, err := time.Parse(containerLoggerTimeStampFormat, ts)

	if err != nil {
		return cmdStatus, false
	}

	// the actual log line starts after the timestamp + a space
	line = line[len(containerLoggerTimeStampFormat)+1:]

	ok := cmdStatus.TryUnmarshal(line)
	return cmdStatus, ok
}

var _ io.WriteCloser = &entrypointLogForwarder{}

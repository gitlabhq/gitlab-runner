package service_helpers

// This file contains executor-agnostic code related to capturing service
// container logs and streaming them to the job's trace log.

import (
	"bytes"
	"io"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

// InlineServiceLogWriter must implement io.WriteCloser
var _ io.WriteCloser = &InlineServiceLogWriter{}

// InlineServiceLogWriter implements an io.WriteCloser that prefixes log
// messages with the container name, and colourizes the message. It is intended
// to be used to write captured service container logs to this task's trace
// stream.
type InlineServiceLogWriter struct {
	sink   io.Writer // io.Writer because we do not want to close the sink
	prefix []byte
	suffix []byte
}

func (sw *InlineServiceLogWriter) Write(p []byte) (int, error) {
	n := 0

	for n < len(p) {
		end := bytes.IndexByte(p[n:], '\n')
		if end < 0 {
			end = len(p[n:])
		}

		if _, err := sw.sink.Write(sw.prefix); err != nil {
			return n, err
		}

		nn, err := sw.sink.Write(p[n : n+end])
		n += nn
		if len(p[n:]) > 0 && err == nil {
			n++
		}
		if err != nil {
			return n, err
		}

		if _, err := sw.sink.Write(sw.suffix); err != nil {
			return n, err
		}
	}

	return n, nil
}

// Don't actually close the underlying sink in this case since it's the main job
// trace.
func (sw *InlineServiceLogWriter) Close() error { return nil }

// NewInlineServiceLogWriter returns a new InlineServiceLogWriter instance which
// wraps the specified sink, and prefixes all read lines with the specified
// container's name.
func NewInlineServiceLogWriter(serviceName string, sink io.Writer) *InlineServiceLogWriter {
	return &InlineServiceLogWriter{
		prefix: []byte(helpers.ANSI_GREY + "[service:" + serviceName + "] "),
		suffix: []byte(helpers.ANSI_RESET + "\n"),
		sink:   sink,
	}
}

// Package innerstream parses the wire format the inner step-runner's
// timestamper emits and demuxes its content back into separate stdout and
// stderr writers. The outer step-runner re-stamps everything its builtins
// write, so without this every nested log line would carry two stacked
// timestamps.
//
// Header layout (see step-runner's internal/streamer/timestamper):
//
//	bytes 0..27  "YYYY-MM-DDTHH:MM:SS.uuuuuuZ "  (timestamp + space)
//	bytes 28..29 stream id (2 hex digits)
//	byte  30     stream type   ('O' = stdout, 'E' = stderr)
//	byte  31     line type     (' ' = full,   '+' = continuation)
//
// Every emitted physical line on the wire ends in '\n' — including
// '\r'-flushed partial lines, where the timestamper appends '\n' itself.
//
// A line type of '+' means "this is a continuation of the previous line on
// my stream"; the previous line's '\n' was an artificial split (buffer
// overflow or '\r' flush) and is not part of the original logical line.
// To keep the outer masker — which matches phrases byte-by-byte and resets
// on any mismatch — from being broken by these injected newlines, we hold
// the most recent line per stream and strip its trailing '\n' when the
// next line on the same stream signals it was a continuation.
package innerstream

import (
	"bytes"
	"io"
)

const (
	headerLen     = 32
	streamTypeOff = 30
	lineTypeOff   = 31

	streamStdout = 'O'
	streamStderr = 'E'
	lineFull     = ' '
	linePartial  = '+'
)

// Splitter implements io.Writer. Bytes written to it are interpreted as the
// inner timestamper's wire format and the decoded bodies are forwarded to
// the appropriate stdout/stderr writer.
type Splitter struct {
	stdout io.Writer
	stderr io.Writer

	inBuf []byte // bytes received but not yet terminated by '\n'

	pendingStdout []byte // body (incl. trailing '\n') of last stdout line
	pendingStderr []byte // body (incl. trailing '\n') of last stderr line
}

func New(stdout, stderr io.Writer) *Splitter {
	return &Splitter{stdout: stdout, stderr: stderr}
}

func (s *Splitter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		nl := bytes.IndexByte(p, '\n')
		if nl == -1 {
			s.inBuf = append(s.inBuf, p...)
			return written + len(p), nil
		}
		s.inBuf = append(s.inBuf, p[:nl+1]...)
		if err := s.consumeLine(); err != nil {
			return written, err
		}
		s.inBuf = s.inBuf[:0]
		written += nl + 1
		p = p[nl+1:]
	}
	return written, nil
}

// Flush drains any line whose continuation marker we never saw; safe to
// call multiple times. Callers invoke this after the source stream ends,
// since extended.FollowOutput.Logs has no Close hook.
func (s *Splitter) Flush() error {
	if err := s.flushOne(&s.pendingStdout, s.stdout); err != nil {
		return err
	}
	return s.flushOne(&s.pendingStderr, s.stderr)
}

func (s *Splitter) flushOne(pending *[]byte, w io.Writer) error {
	if len(*pending) == 0 {
		return nil
	}
	_, err := w.Write(*pending)
	*pending = (*pending)[:0]
	return err
}

func (s *Splitter) consumeLine() error {
	line := s.inBuf
	if len(line) <= headerLen {
		// Malformed (shorter than the prefix). Drop it rather than emit
		// header bytes to the trace.
		return nil
	}

	streamType := line[streamTypeOff]
	lineType := line[lineTypeOff]
	body := line[headerLen:]

	pending, w := &s.pendingStdout, s.stdout
	if streamType == streamStderr {
		pending, w = &s.pendingStderr, s.stderr
	}

	if len(*pending) > 0 {
		out := *pending
		if lineType == linePartial && out[len(out)-1] == '\n' {
			out = out[:len(out)-1]
		}
		if _, err := w.Write(out); err != nil {
			return err
		}
	}

	*pending = append((*pending)[:0], body...)
	return nil
}

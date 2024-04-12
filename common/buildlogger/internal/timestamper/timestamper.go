package timestamper

import (
	"bytes"
	"io"
	"math"
	"strconv"
	"time"
)

const (
	StdoutType StreamType = 'O'
	StderrType StreamType = 'E'

	PartialLineType LineType = '+'
	FullLineType    LineType = ' '

	hextable = "0123456789abcdef"

	// bufSize is the amount of data this implementation will buffer
	// when no newline character is found. It is _not_ the maximum line length
	// any consumer of the logs will receive.
	bufSize = 8 * 1024

	// fracs is the nanosecond length we append
	fracs = 6

	// additional bytes added to the format:
	// - nanosecond separator ('.')
	// - single byte for append flag
	additionalBytes = 2
)

type (
	StreamType byte
	LineType   byte
)

var (
	now = func() time.Time {
		return time.Now().UTC()
	}

	lineEscape = []byte("\n")
)

// Logger implements the standard io.Write interface and adds lightweight
// metadata in the form of:
// <date> <stream id><stream type><append flag><message>
//
// Where:
// - <date> is a RFC3339 Nano formatted date
// - <stream id> is a 2-digit hex encoded user provided stream identifier
// - <stream type> is either 'stdout' or 'stderr'
// - <append flag> is either ' ' (no-op) or '+' (append line to last line)
// - <message> is a user provided message.
//
// This format is intended to be well suited to CI/CD logs, where timed output
// can help determine the duration of executed commands.
//
// A new log line is emitted for each new-line character (\n) found within data
// provided to Write().
//
// A new log line is also emitted for the last carriage return (\r) in calls to
// Write() that don't contain a new-line character. Such lines are often used
// to display progress bars, so having them "flushed" to the underlying stream
// can help with live log viewing.
type Logger struct {
	buf bytes.Buffer
	w   io.Writer

	bufStream []byte
	timeLen   int
	timestamp bool
}

func New(w io.Writer, streamType StreamType, streamNumber uint8, timestamp bool) *Logger {
	l := &Logger{
		w:         w,
		timestamp: timestamp,
	}

	if timestamp {
		l.timeLen = len(time.Now().UTC().Format(time.RFC3339)) + fracs + additionalBytes
	}
	l.bufStream = make([]byte, l.timeLen+4)
	if timestamp {
		l.bufStream[l.timeLen-1] = ' '
	}
	l.bufStream[l.timeLen+0] = hextable[streamNumber>>4]
	l.bufStream[l.timeLen+1] = hextable[streamNumber&0x0f]
	l.bufStream[l.timeLen+2] = byte(streamType)
	l.bufStream[l.timeLen+3] = byte(FullLineType)

	return l
}

func (l *Logger) Write(p []byte) (n int, err error) {
	n, err = l.writeLines(p)
	if err != nil {
		return n, err
	}

	nn, err := l.writeCarriageReturns(p[n:])
	n += nn
	if err != nil {
		return n, err
	}

	nn, err = l.buffer(p[n:])
	n += nn
	return n, err
}

// buffer is used when we have input data that contains no newline character.
//
// l.buf is filled with data until either a newline character appears or
// we exceed bufSize. When we exceed the buffer size, we flush a new line
// and write the buffer to the underlying writer directly. To indicate that
// this has occurred, we then set the append flag for the next line to be
// written.
//
// Because we write the buffer to the underling writer when the bufSize has
// been exceeded, bufSize is not indicative of the maximum line length a
// consumer will receive, it's only used internally so that this implementation
// doesn't need to have an infinite sized buffer.
func (l *Logger) buffer(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	// if we exceed our buffer size, write directly to underlying writer
	// nolint:nestif
	if len(p)+l.buf.Len() > bufSize {
		if l.buf.Len() == 0 {
			if err := l.writeHeader(l.w); err != nil {
				return 0, err
			}
		}

		_, err := l.w.Write(l.buf.Bytes())
		if err != nil {
			return 0, err
		}
		l.buf.Reset()

		// ensure next write is a continuation
		l.bufStream[l.timeLen+3] = byte(PartialLineType)

		nn, err := l.w.Write(p)
		n += nn
		if err != nil {
			return n, err
		}

		_, err = l.w.Write(lineEscape)
		return n, err
	}

	// start new buffer
	if l.buf.Len() == 0 {
		if err := l.writeHeader(&l.buf); err != nil {
			return n, err
		}
	}

	// append to existing buffer
	return l.buf.Write(p)
}

func (l *Logger) writeLines(p []byte) (n int, err error) {
	idx := bytes.IndexByte(p, '\n')
	if idx == -1 {
		return n, err
	}

	if l.buf.Len() > 0 {
		_, err := l.w.Write(l.buf.Bytes())
		if err != nil {
			return 0, err
		}

		l.buf.Reset()

		nn, err := l.w.Write(p[:idx+1])
		n += nn
		if err != nil {
			return n, err
		}
	}

	for {
		idx := bytes.IndexByte(p[n:], '\n')
		if idx == -1 {
			return n, err
		}

		if err := l.writeHeader(l.w); err != nil {
			return n, err
		}

		nn, err := l.w.Write(p[n : n+idx+1])
		n += nn
		if err != nil {
			return n, err
		}
	}
}

func (l *Logger) writeCarriageReturns(p []byte) (n int, err error) {
	idx := bytes.LastIndexByte(p, '\r')
	if idx == -1 {
		return n, err
	}

	if l.buf.Len() > 0 {
		_, err := l.w.Write(l.buf.Bytes())
		if err != nil {
			return 0, err
		}

		l.buf.Reset()
	} else {
		if err := l.writeHeader(l.w); err != nil {
			return n, err
		}
	}

	// ensure next write is a continuation
	l.bufStream[l.timeLen+3] = byte(PartialLineType)

	nn, err := l.w.Write(p[n : n+idx+1])
	n += nn
	if err != nil {
		return n, err
	}

	_, err = l.w.Write(lineEscape)
	return n, err
}

func (l *Logger) writeHeader(w io.Writer) error {
	if l.timestamp {
		t := now()

		// time.RFC3339 doesn't add nanosecond precision, and time.RFC3339Nano doesn't
		// use a fixed length of precision. Whilst we could use a custom format, this
		// is slower, as Go as built-in optimizations for RFC3339. So here we use the
		// non-nano version, and then add nanoseconds to a fixed length. Fixed length
		// is important because it makes the logs easier for both a human and machine
		// to read.
		t.AppendFormat(l.bufStream[:0], time.RFC3339)

		// remove the 'Z'
		l.bufStream = l.bufStream[:l.timeLen-fracs-additionalBytes]
		l.bufStream[len(l.bufStream)-1] = '.' // replace 'Z' for '.'

		// ensure nanoseconds doesn't exceed our fracs precision
		nanos := t.Nanosecond() / int(math.Pow10(9-fracs))

		// add nanoseconds and append leading zeros
		leadingZeros := len(l.bufStream)
		l.bufStream = strconv.AppendInt(l.bufStream, int64(nanos), 10)
		leadingZeros = fracs - (len(l.bufStream) - leadingZeros)
		for i := 0; i < leadingZeros; i++ {
			l.bufStream = append(l.bufStream, '0')
		}

		// add 'Z' back
		l.bufStream = append(l.bufStream, 'Z')

		// expand back to full header size
		l.bufStream = l.bufStream[:l.timeLen+4]
	}
	_, err := w.Write(l.bufStream)

	l.bufStream[l.timeLen+3] = byte(FullLineType)

	return err
}

func (l *Logger) Close() error {
	if l.buf.Len() > 0 {
		l.buf.Write(lineEscape)
		_, err := l.w.Write(l.buf.Bytes())
		return err
	}
	return nil
}

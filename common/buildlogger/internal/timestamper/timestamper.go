package timestamper

import (
	"bytes"
	"io"
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

	// nanosDivisor truncates time.Time.Nanosecond() to microsecond precision.
	nanosDivisor = 1000

	format = "YYYY-mm-ddTHH:MM:SS.123456Z "
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

	// cached unix second of the last header format. When the next
	// header is in the same second we only need to rewrite the
	// microsecond digits, skipping the relatively expensive
	// time.Time.Date / Clock calls. Initialised to a sentinel that
	// no real timestamp will match so the first call always formats.
	cachedUnix int64
}

func New(w io.Writer, streamType StreamType, streamNumber uint8, timestamp bool) *Logger {
	l := &Logger{
		w:          w,
		timestamp:  timestamp,
		cachedUnix: -1,
	}

	if timestamp {
		l.timeLen = len(format)
	}
	l.bufStream = make([]byte, l.timeLen+4)
	if timestamp {
		// pre-fill the static separators of YYYY-MM-DDTHH:MM:SS.UUUUUUZ<space>
		// so writeHeader only has to fill in the digit positions.
		l.bufStream[4] = '-'
		l.bufStream[7] = '-'
		l.bufStream[10] = 'T'
		l.bufStream[13] = ':'
		l.bufStream[16] = ':'
		l.bufStream[19] = '.'
		l.bufStream[26] = 'Z'
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
		sec := t.Unix()
		buf := l.bufStream

		// Static separators were pre-filled in New(). On a same-second
		// repeat we only refresh the microsecond digits.
		if sec != l.cachedUnix {
			year, month, day := t.Date()
			hour, minute, secOfMin := t.Clock()
			buf[0] = '0' + byte(year/1000)
			buf[1] = '0' + byte((year/100)%10)
			buf[2] = '0' + byte((year/10)%10)
			buf[3] = '0' + byte(year%10)
			buf[5] = '0' + byte(int(month)/10)
			buf[6] = '0' + byte(int(month)%10)
			buf[8] = '0' + byte(day/10)
			buf[9] = '0' + byte(day%10)
			buf[11] = '0' + byte(hour/10)
			buf[12] = '0' + byte(hour%10)
			buf[14] = '0' + byte(minute/10)
			buf[15] = '0' + byte(minute%10)
			buf[17] = '0' + byte(secOfMin/10)
			buf[18] = '0' + byte(secOfMin%10)
			l.cachedUnix = sec
		}

		nanos := t.Nanosecond() / nanosDivisor
		buf[25] = '0' + byte(nanos%10)
		nanos /= 10
		buf[24] = '0' + byte(nanos%10)
		nanos /= 10
		buf[23] = '0' + byte(nanos%10)
		nanos /= 10
		buf[22] = '0' + byte(nanos%10)
		nanos /= 10
		buf[21] = '0' + byte(nanos%10)
		buf[20] = '0' + byte(nanos/10)
	}
	_, err := w.Write(l.bufStream)

	l.bufStream[l.timeLen+3] = byte(FullLineType)

	return err
}

func (l *Logger) Close() error {
	if l.buf.Len() == 0 {
		return nil
	}
	l.buf.Write(lineEscape)
	_, err := l.w.Write(l.buf.Bytes())
	l.buf.Reset()
	return err
}

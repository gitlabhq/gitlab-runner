package buildlogger

import (
	"fmt"
	"io"
	"sync"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/internal"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/internal/masker"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/internal/tokensanitizer"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/internal/urlsanitizer"
)

type Trace interface {
	Write([]byte) (int, error)
	IsStdout() bool
}

type Options struct {
	MaskPhrases       []string
	MaskTokenPrefixes []string
	Timestamping      bool
}

const (
	Stdout StreamType = 'O'
	Stderr StreamType = 'E'
)

type StreamType byte

type Logger struct {
	internal.Tee

	base   io.WriteCloser
	opts   Options
	closed bool

	// mu protects w, as Tee's Println, Debugln etc. funcs can be called
	// throughout the runner from different go routines.
	mu *sync.Mutex
	w  io.WriteCloser
}

func NewNopCloser(w io.Writer) io.WriteCloser {
	return internal.NewNopCloser(w)
}

const (
	// StreamExecutorLevel is the stream number for an executor log line
	StreamExecutorLevel = 0
	// StreamWorkLevel is the stream number for a work log line
	StreamWorkLevel = 1
	// StreamStartingServiceLevel is the starting stream number for a service log line
	StreamStartingServiceLevel = 15
)

func New(log Trace, entry *logrus.Entry, opts Options) Logger {
	l := Logger{mu: new(sync.Mutex)}
	l.opts = opts

	if log != nil {
		l.base = internal.NewNopCloser(log)
		l.w = wrap(l.base, StreamExecutorLevel, Stdout, opts)
	}

	l.Tee = internal.NewTee(l.SendRawLog, entry, log != nil && log.IsStdout())

	return l
}

func (l *Logger) Stream(streamID int, streamType StreamType) io.WriteCloser {
	// l.base being nil happens when the buildlogger hasn't been created with New() or
	// a nil was passed for the Trace parameter. This only happens in tests, and to not
	// panic we simply return a discard writer.
	if l.base == nil {
		return internal.NewNopCloser(io.Discard)
	}

	return wrap(l.base, streamID, streamType, l.opts)
}

func wrap(w io.WriteCloser, streamID int, streamType StreamType, opts Options) io.WriteCloser {
	w = tokensanitizer.New(w, opts.MaskTokenPrefixes)
	w = urlsanitizer.New(w)
	w = masker.New(w, opts.MaskPhrases)

	return w
}

func (l *Logger) WithFields(fields logrus.Fields) Logger {
	return Logger{
		Tee:  l.Tee.WithFields(fields),
		base: l.base,
		opts: l.opts,
		w:    l.w,
		mu:   l.mu,
	}
}

func (l *Logger) SendRawLog(args ...any) {
	if l.w == nil {
		return
	}

	l.mu.Lock()
	_, _ = fmt.Fprint(l.w, args...)
	l.mu.Unlock()
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return fmt.Errorf("already closed")
	}
	l.closed = true

	if l.w != nil {
		return l.w.Close()
	}

	return nil
}

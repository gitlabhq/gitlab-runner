package buildlogger

import (
	"fmt"
	"io"
	"sync"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/internal"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/internal/masker"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/internal/timestamper"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/internal/tokensanitizer"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/internal/urlsanitizer"
)

type Trace interface {
	Write([]byte) (int, error)
	IsStdout() bool
}

type Options struct {
	MaskPhrases          []string
	MaskTokenPrefixes    []string
	Timestamping         bool
	MaskAllDefaultTokens bool
	TeeOnly              bool
}

const (
	Stdout StreamType = 'O'
	Stderr StreamType = 'E'
)

type StreamType byte

type Logger struct {
	internal.Tee

	base   io.WriteCloser
	closed bool

	// mu protects w, as Tee's Println, Debugln etc. funcs can be called
	// throughout the runner from different go routines.
	mu *sync.Mutex
	w  io.WriteCloser

	maskPhrases       [][]byte
	maskTokenPrefixes [][]byte
	timestamping      bool
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

	l.maskPhrases = internal.Unique(opts.MaskPhrases)
	l.maskTokenPrefixes = internal.Unique(
		append(opts.MaskTokenPrefixes, tokensanitizer.DefaultTokenPrefixes(opts.MaskAllDefaultTokens)...),
	)
	l.timestamping = opts.Timestamping

	if log != nil {
		l.base = internal.NewNopCloser(log)
		l.w = l.wrap(l.base, StreamExecutorLevel, Stdout)
	}

	l.Tee = internal.NewTee(l.SendRawLog, entry, log != nil && log.IsStdout())
	if opts.TeeOnly {
		l.Tee = l.Tee.WithoutLog()
	}

	return l
}

func (l *Logger) Stream(streamID int, streamType StreamType) io.WriteCloser {
	// l.base being nil happens when the buildlogger hasn't been created with New() or
	// a nil was passed for the Trace parameter. This only happens in tests, and to not
	// panic we simply return a discard writer.
	if l.base == nil {
		return internal.NewNopCloser(io.Discard)
	}

	return l.wrap(l.base, streamID, streamType)
}

// StepRunnerStream returns a writer for output produced by step-runner. When
// step-runner emits lines already pre-stamped in the runner's timestamper
// format, the data flows straight through to the base trace; otherwise the
// full wrap chain is applied. This is the single hook for any future
// step-runner-specific stream behavior.
func (l *Logger) StepRunnerStream(streamID int, streamType StreamType) io.WriteCloser {
	if l.base == nil {
		return internal.NewNopCloser(io.Discard)
	}

	return &stepRunnerStream{
		passthrough: internal.NewSync(l.base),
		buildWrapped: func() io.WriteCloser {
			return l.wrap(l.base, streamID, streamType)
		},
	}
}

// stepRunnerStream picks between two writers on its first write. The runner's
// timestamper emits a fixed-width header — YYYY-MM-DDTHH:MM:SS.UUUUUUZ<space>
// — and isPreStamped validates that full shape so producers other than
// step-runner that happen to put 'Z' at byte 26 don't trip the passthrough.
//
// The wrap chain (timestamper, masker, sanitizers, sync) is built lazily via
// buildWrapped so streams that only see pre-stamped output never pay for it.
// step-runner output is the dominant case, so the saving compounds across
// streams.
type stepRunnerStream struct {
	once         sync.Once
	chosen       io.WriteCloser
	passthrough  io.WriteCloser
	buildWrapped func() io.WriteCloser
}

// isPreStamped reports whether p starts with a runner timestamper header
// (YYYY-MM-DDTHH:MM:SS.UUUUUUZ<space>). Validating the full shape, not
// just byte 26, hardens the detection against producers other than
// step-runner that might happen to put 'Z' at byte 26.
func isPreStamped(p []byte) bool {
	if len(p) < 28 {
		return false
	}
	return p[4] == '-' && p[7] == '-' && p[10] == 'T' &&
		p[13] == ':' && p[16] == ':' && p[19] == '.' &&
		p[26] == 'Z' && p[27] == ' '
}

func (s *stepRunnerStream) Write(p []byte) (int, error) {
	s.once.Do(func() {
		if isPreStamped(p) {
			s.chosen = s.passthrough
		} else {
			s.chosen = s.buildWrapped()
		}
	})
	return s.chosen.Write(p)
}

func (s *stepRunnerStream) Close() error {
	// Close-without-write: nothing has been buffered by the wrap chain
	// (it was never built), so the passthrough's Close is sufficient.
	s.once.Do(func() { s.chosen = s.passthrough })
	return s.chosen.Close()
}

// wrap wraps the underlying writer with "filters". Order here somewhat
// matters, and the order they're instantiated in is the reverse order in which
// writes are processed, e.g. last added filter is the first to process data.
//
// order:
// - sync writer to ensure that multiple writes cannot happen concurrently
// - mask phrases (masker.New)
// - mask sensitive URL parameters (urlsanitizer.New)
// - mask secrets with a prefixed token (tokentanitizer.New)
// - split log lines and add timestamps (timestamper.New)
func (l *Logger) wrap(w io.WriteCloser, streamID int, streamType StreamType) io.WriteCloser {
	if l.timestamping {
		w = timestamper.New(w, timestamper.StreamType(streamType), uint8(streamID), true)
	}

	w = tokensanitizer.New(w, l.maskTokenPrefixes)
	w = urlsanitizer.New(w)
	w = masker.New(w, l.maskPhrases)
	w = internal.NewSync(w)

	return w
}

func (l *Logger) WithFields(fields logrus.Fields) *Logger {
	return &Logger{
		Tee:               l.Tee.WithFields(fields),
		base:              l.base,
		mu:                l.mu,
		w:                 l.w,
		maskPhrases:       l.maskPhrases,
		maskTokenPrefixes: l.maskTokenPrefixes,
		timestamping:      l.timestamping,
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

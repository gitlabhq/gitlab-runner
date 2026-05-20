package buildlogger

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/innerstream"
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

// StepRunnerStream returns a writer for output produced by step-runner.
// The mode is picked on the first write:
//
//   - pre-stamped (per isPreStamped) and Timestamping is on: bytes flow
//     straight through to the base trace, preserving step-runner's headers
//     and avoiding double-stamping.
//   - pre-stamped and Timestamping is off: step-runner stamps anyway (it
//     has no knob to disable them), so bytes are demuxed by innerstream
//     and inner bodies routed through per-type wrap chains, giving the
//     stamp-free trace the caller asked for.
//   - not pre-stamped: bytes flow through one wrap chain with the
//     caller-supplied streamType.
func (l *Logger) StepRunnerStream(streamID int, streamType StreamType) io.WriteCloser {
	if l.base == nil {
		return internal.NewNopCloser(io.Discard)
	}

	return &stepRunnerStream{
		timestamping: l.timestamping,
		passthrough:  internal.NewSync(l.base),
		buildStripped: func() io.WriteCloser {
			return newInnerStreamStripper(
				l.wrap(l.base, streamID, Stdout),
				l.wrap(l.base, streamID, Stderr),
			)
		},
		buildWrapped: func() io.WriteCloser {
			return l.wrap(l.base, streamID, streamType)
		},
	}
}

type stepRunnerStream struct {
	once          sync.Once
	chosen        io.WriteCloser
	timestamping  bool
	passthrough   io.WriteCloser
	buildStripped func() io.WriteCloser
	buildWrapped  func() io.WriteCloser
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
	s.once.Do(s.pickMode(p))
	return s.chosen.Write(p)
}

func (s *stepRunnerStream) Close() error {
	// Close before any write: fall back to the wrap chain so Close has
	// something to delegate to.
	s.once.Do(s.pickMode(nil))
	return s.chosen.Close()
}

func (s *stepRunnerStream) pickMode(p []byte) func() {
	return func() {
		switch {
		case isPreStamped(p) && s.timestamping:
			s.chosen = s.passthrough
		case isPreStamped(p):
			s.chosen = s.buildStripped()
		default:
			s.chosen = s.buildWrapped()
		}
	}
}

// innerStreamStripper adapts innerstream.Splitter to io.WriteCloser. Close
// drains the splitter's last unterminated line and closes both wrap chains.
type innerStreamStripper struct {
	splitter       *innerstream.Splitter
	stdout, stderr io.WriteCloser
}

func newInnerStreamStripper(stdoutW, stderrW io.WriteCloser) *innerStreamStripper {
	return &innerStreamStripper{
		splitter: innerstream.New(stdoutW, stderrW),
		stdout:   stdoutW,
		stderr:   stderrW,
	}
}

func (s *innerStreamStripper) Write(p []byte) (int, error) {
	return s.splitter.Write(p)
}

func (s *innerStreamStripper) Close() error {
	return errors.Join(s.splitter.Flush(), s.stdout.Close(), s.stderr.Close())
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

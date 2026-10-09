package readywriter

import (
	"bytes"
	"context"
	"io"
	"slices"
	"sync"
)

const maxSocketLen = 4 * 1024

var readyMarker = []byte("step-runner is listening on socket ")

// state is where the writer is in consuming the ready line.
type state int

const (
	scanning    state = iota // looking for the marker; held may be non-empty
	capturing                // marker seen; collecting the socket path
	passthrough              // the ready line was consumed or given up on
)

// readyWriter forwards what is written to it except the ready line, which it
// consumes: the socket path is delivered over the ready channel and the line
// itself is a handshake, not job output.
type readyWriter struct {
	io.Writer
	ctx   context.Context
	ready chan string
	once  sync.Once

	state state
	// held is a trailing run of bytes that is a proper prefix of the marker.
	// It is forwarded once a later write shows it was not the marker, or on
	// Close, so a stream ending mid-prefix loses nothing.
	held   []byte
	socket bytes.Buffer
}

// New returns a writer that forwards to w everything except the ready line,
// and a channel that receives the socket path from that line, or is closed
// without a value when ctx ends first or the path is too long to be one.
// Close forwards anything still held back; it does not close w.
func New(ctx context.Context, w io.Writer) (io.WriteCloser, <-chan string) {
	ch := make(chan string, 1)
	rw := &readyWriter{Writer: w, ctx: ctx, ready: ch}
	go func() {
		<-ctx.Done()
		rw.close(false)
	}()
	return rw, ch
}

func (rw *readyWriter) close(sendSocket bool) {
	rw.once.Do(func() {
		if sendSocket {
			rw.ready <- rw.socket.String()
		}
		close(rw.ready)
	})
}

// Write accepts all of p: bytes that may start the marker are held back
// rather than refused, so n is len(p) unless the underlying write fails.
func (rw *readyWriter) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		var err error
		switch rw.state {
		case scanning:
			p, err = rw.scan(p)
		case capturing:
			p, err = rw.capture(p)
		default:
			p, err = nil, rw.forward(p)
		}
		if err != nil {
			return 0, err
		}
	}
	return n, nil
}

// scan looks for the marker in the held bytes plus p. It forwards everything
// before the marker and returns what follows it, or forwards everything that
// cannot start the marker and holds back the rest.
func (rw *readyWriter) scan(p []byte) ([]byte, error) {
	data := slices.Concat(rw.held, p)
	rw.held = nil

	if before, after, found := bytes.Cut(data, readyMarker); found {
		rw.state = capturing
		return after, rw.forward(before)
	}

	keep := markerPrefixLen(data)
	rw.held = bytes.Clone(data[len(data)-keep:])
	return nil, rw.forward(data[:len(data)-keep])
}

// markerPrefixLen returns the length of the longest suffix of data that is a
// proper prefix of the marker: the rest of the marker may still arrive in the
// next write, so those bytes cannot be forwarded yet.
func markerPrefixLen(data []byte) int {
	for n := min(len(data), len(readyMarker)-1); n > 0; n-- {
		if bytes.HasPrefix(readyMarker, data[len(data)-n:]) {
			return n
		}
	}
	return 0
}

// capture collects the socket path up to the end of the line, then signals
// ready and returns what follows. A path longer than maxSocketLen is not a
// handshake we can use: the line is forwarded as ordinary output and the
// ready channel is closed without a value.
func (rw *readyWriter) capture(p []byte) ([]byte, error) {
	line, rest, found := bytes.Cut(p, []byte{'\n'})

	if rw.socket.Len()+len(line) > maxSocketLen {
		rw.close(false)
		rw.state = passthrough
		if err := rw.forward(readyMarker); err != nil {
			return nil, err
		}
		return p, rw.forward(rw.socket.Bytes())
	}

	rw.socket.Write(line)
	if !found {
		return nil, nil
	}

	rw.close(true)
	rw.state = passthrough
	return rest, nil
}

// Close forwards whatever was held back that did not become a complete ready
// line, so the end of the stream is not truncated. The underlying writer is
// left open for its owner to close.
func (rw *readyWriter) Close() error {
	var pending []byte
	switch rw.state {
	case scanning:
		pending = rw.held
	case capturing:
		pending = append(bytes.Clone(readyMarker), rw.socket.Bytes()...)
	}
	rw.state = passthrough
	rw.held = nil
	return rw.forward(pending)
}

func (rw *readyWriter) forward(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	_, err := rw.Writer.Write(p)
	return err
}

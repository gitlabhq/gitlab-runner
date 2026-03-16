package readywriter

import (
	"bytes"
	"context"
	"io"
	"sync"
)

const maxSocketLen = 4 * 1024

var readyMarker = []byte("step-runner is listening on socket ")

type readyWriter struct {
	io.Writer
	ctx     context.Context
	ready   chan string
	matched int
	socket  bytes.Buffer
	once    sync.Once
}

func New(ctx context.Context, w io.Writer) (io.Writer, <-chan string) {
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

func (rw *readyWriter) Write(p []byte) (int, error) {
	n, err := rw.Writer.Write(p)
	if rw.matched < 0 || n == 0 {
		return n, err
	}

	data := p[:n]
	if rw.matched == 0 {
		if idx := bytes.IndexByte(data, readyMarker[0]); idx >= 0 {
			data = data[idx:]
		} else {
			return n, err
		}
	}

	for _, b := range data {
		if rw.matched == len(readyMarker) {
			if rw.handleSocketByte(b) {
				return n, err
			}
			continue
		}
		rw.matchMarkerByte(b)
	}
	return n, err
}

func (rw *readyWriter) handleSocketByte(b byte) bool {
	if b == '\n' {
		rw.close(true)
		rw.matched = -1
		return true
	}
	if rw.socket.Len() >= maxSocketLen {
		rw.close(false)
		rw.matched = -1
		return true
	}
	rw.socket.WriteByte(b)
	return false
}

func (rw *readyWriter) matchMarkerByte(b byte) {
	switch b {
	case readyMarker[rw.matched]:
		rw.matched++
	case readyMarker[0]:
		rw.matched = 1
	default:
		rw.matched = 0
	}
}

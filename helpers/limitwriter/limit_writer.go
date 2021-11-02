package limitwriter

import (
	"errors"
	"io"
)

var ErrWriteLimitExceeded = errors.New("write limit exceeded")

type limitWriter struct {
	w       io.Writer
	limit   int64
	written int64
}

func New(w io.Writer, n int64) io.Writer {
	return &limitWriter{w: w, limit: n}
}

func (w *limitWriter) Write(p []byte) (n int, err error) {
	capacity := w.limit - w.written
	if capacity <= 0 {
		return 0, io.ErrShortWrite
	}

	if int64(len(p)) > capacity {
		n, err = w.w.Write(p[:capacity])
		if err == nil {
			err = ErrWriteLimitExceeded
		}
	} else {
		n, err = w.w.Write(p)
	}

	if n < 0 {
		n = 0
	}
	w.written += int64(n)

	return n, err
}

package meter

import (
	"io"
	"sync/atomic"
	"time"
)

type writer struct {
	*meter

	w io.WriteCloser
}

func NewWriter(w io.WriteCloser, frequency time.Duration, fn UpdateCallback) io.WriteCloser {
	if frequency == 0 {
		return w
	}

	m := &writer{
		w:     w,
		meter: newMeter(),
	}

	m.start(frequency, fn)

	return m
}

func (m *writer) Write(p []byte) (int, error) {
	n, err := m.w.Write(p)
	atomic.AddUint64(&m.count, uint64(n))

	return n, err
}

func (m *writer) Close() error {
	m.doClose()

	return m.w.Close()
}

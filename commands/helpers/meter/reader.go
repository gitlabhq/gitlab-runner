package meter

import (
	"io"
	"sync/atomic"
	"time"
)

type reader struct {
	*meter

	r io.ReadCloser
}

func NewReader(r io.ReadCloser, frequency time.Duration, fn UpdateCallback) io.ReadCloser {
	if frequency == 0 {
		return r
	}

	m := &reader{
		r:     r,
		meter: newMeter(),
	}

	m.start(frequency, fn)

	return m
}

func (m *reader) Read(p []byte) (int, error) {
	n, err := m.r.Read(p)
	atomic.AddUint64(&m.count, uint64(n))

	return n, err
}

func (m *reader) Close() error {
	m.doClose()

	return m.r.Close()
}

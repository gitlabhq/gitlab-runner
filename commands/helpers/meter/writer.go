package meter

import (
	"errors"
	"io"
	"sync/atomic"
	"time"
)

type writer struct {
	*meter

	w  io.WriteCloser
	at io.WriterAt // optional: set when w also implements io.WriterAt (e.g. *os.File)
	tr truncater   // optional: set when w can be truncated (e.g. *os.File)
}

type truncater interface {
	Truncate(size int64) error
}

func NewWriter(w io.WriteCloser, frequency time.Duration, fn UpdateCallback) io.WriteCloser {
	if frequency == 0 {
		return w
	}

	mw := &writer{
		w:     w,
		meter: newMeter(),
	}
	if a, ok := w.(io.WriterAt); ok {
		mw.at = a
	}
	if t, ok := w.(truncater); ok {
		mw.tr = t
	}

	mw.start(frequency, fn)

	return mw
}

func (m *writer) Write(p []byte) (int, error) {
	n, err := m.w.Write(p)
	atomic.AddUint64(&m.count, uint64(n))

	return n, err
}

func (m *writer) WriteAt(p []byte, off int64) (int, error) {
	if m.at == nil {
		return 0, errors.New("meter: underlying writer does not implement io.WriterAt")
	}
	n, err := m.at.WriteAt(p, off)
	atomic.AddUint64(&m.count, uint64(n))
	return n, err
}

// Truncate lets a caller that writes with WriteAt, such as a parallel download, empty the file
// again after a failed attempt. The count of bytes written is left as it is.
func (m *writer) Truncate(size int64) error {
	if m.tr == nil {
		return errors.New("meter: underlying writer does not implement Truncate")
	}
	return m.tr.Truncate(size)
}

func (m *writer) Close() error {
	m.doClose()

	return m.w.Close()
}

package meter

import (
	"io"
	"sync"
	"sync/atomic"
	"time"
)

const UnknownTotalSize = 0

type TransferMeterCommand struct {
	//nolint:lll
	RunnerMeterFrequency time.Duration `long:"runner-meter-frequency" env:"RUNNER_METER_FREQUENCY" description:"If set to more than 0s it enables an interactive transfer meter"`
}

type UpdateCallback func(written uint64, since time.Duration, done bool)

type meter struct {
	r     io.ReadCloser
	w     io.WriteCloser
	count uint64

	done, notify chan struct{}
	close        sync.Once
}

func NewReader(r io.ReadCloser, frequency time.Duration, fn UpdateCallback) io.ReadCloser {
	if frequency == 0 {
		return r
	}

	m := &meter{
		r:      r,
		done:   make(chan struct{}),
		notify: make(chan struct{}),
	}

	m.start(frequency, fn)

	return m
}

func NewWriter(w io.WriteCloser, frequency time.Duration, fn UpdateCallback) io.WriteCloser {
	if frequency == 0 {
		return w
	}

	m := &meter{
		w:      w,
		done:   make(chan struct{}),
		notify: make(chan struct{}),
	}

	m.start(frequency, fn)

	return m
}

func (m *meter) start(frequency time.Duration, fn UpdateCallback) {
	if frequency < time.Second {
		frequency = time.Second
	}

	started := time.Now()

	go func() {
		defer close(m.done)

		ticker := time.NewTicker(frequency)
		defer ticker.Stop()

		for {
			fn(atomic.LoadUint64(&m.count), time.Since(started), false)

			select {
			case <-ticker.C:
			case <-m.notify:
				fn(atomic.LoadUint64(&m.count), time.Since(started), true)
				return
			}
		}
	}()
}

func (m *meter) Read(p []byte) (int, error) {
	n, err := m.r.Read(p)
	atomic.AddUint64(&m.count, uint64(n))

	return n, err
}

func (m *meter) Write(p []byte) (int, error) {
	n, err := m.w.Write(p)
	atomic.AddUint64(&m.count, uint64(n))

	return n, err
}

func (m *meter) Close() error {
	m.close.Do(func() {
		// notify we're done
		close(m.notify)
		// wait for close
		<-m.done
	})

	return m.r.Close()
}

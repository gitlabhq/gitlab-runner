package meter

import (
	"fmt"
	"io"
	"math"
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
	count uint64

	done, notify chan struct{}
	close        sync.Once
}

func New(r io.ReadCloser, frequency time.Duration, fn UpdateCallback) io.ReadCloser {
	if frequency == 0 {
		return r
	}

	if frequency < time.Second {
		frequency = time.Second
	}

	m := &meter{
		r:      r,
		done:   make(chan struct{}),
		notify: make(chan struct{}),
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

	return m
}

func (m *meter) Read(p []byte) (int, error) {
	n, err := m.r.Read(p)
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

func FormatByteRate(b uint64, d time.Duration) string {
	b = uint64(float64(b) / math.Max(time.Nanosecond.Seconds(), d.Seconds()))
	rate, prefix := formatBytes(b)
	if prefix == 0 {
		return fmt.Sprintf("%d B/s", int(rate))
	}

	return fmt.Sprintf("%.1f %cB/s", rate, prefix)
}

func FormatBytes(b uint64) string {
	size, prefix := formatBytes(b)
	if prefix == 0 {
		return fmt.Sprintf("%d B", int(size))
	}

	return fmt.Sprintf("%.2f %cB", size, prefix)
}

func formatBytes(b uint64) (float64, byte) {
	const (
		unit   = 1000
		prefix = "KMGTPE"
	)

	if b < unit {
		return float64(b), 0
	}

	div := int64(unit)
	exp := 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return float64(b) / float64(div), prefix[exp]
}

func LabelledRateFormat(w io.Writer, label string, totalSize int64) UpdateCallback {
	return func(written uint64, since time.Duration, done bool) {
		known := ""
		if totalSize > UnknownTotalSize {
			known = "/" + FormatBytes(uint64(totalSize))
		}

		line := fmt.Sprintf(
			"\r%s %s%s (%s)                ",
			label,
			FormatBytes(written),
			known,
			FormatByteRate(written, since),
		)

		if done {
			_, _ = fmt.Fprintln(w, line)
			return
		}
		_, _ = fmt.Fprint(w, line)
	}
}

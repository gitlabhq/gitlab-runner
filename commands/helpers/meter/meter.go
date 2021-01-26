package meter

import (
	"sync"
	"sync/atomic"
	"time"
)

const UnknownTotalSize = 0

type TransferMeterCommand struct {
	//nolint:lll
	TransferMeterFrequency time.Duration `long:"transfer-meter-frequency" env:"TRANSFER_METER_FREQUENCY" description:"If set to more than 0s it enables an interactive transfer meter"`
}

type UpdateCallback func(written uint64, since time.Duration, done bool)

type meter struct {
	count uint64

	done, notify chan struct{}
	close        sync.Once
}

func newMeter() *meter {
	return &meter{
		done:   make(chan struct{}),
		notify: make(chan struct{}),
	}
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

func (m *meter) doClose() {
	m.close.Do(func() {
		// notify we're done
		close(m.notify)
		// wait for close
		<-m.done
	})
}

//go:build !integration

package machine

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunnerMachinesCoordinator_WaitForGrowthCapacity(t *testing.T) {
	concurrencyTracker := func(t time.Duration) (func(), *int32) {
		var concurrency, maxConcurrency int32
		var maxConcurrencyLock sync.Mutex
		trackMaxConcurrency := func() {
			defer atomic.AddInt32(&concurrency, -1)
			c := atomic.AddInt32(&concurrency, 1)

			maxConcurrencyLock.Lock()
			if c > maxConcurrency {
				maxConcurrency = c
			}
			maxConcurrencyLock.Unlock()

			time.Sleep(t)
		}

		return trackMaxConcurrency, &maxConcurrency
	}

	tests := map[string]struct {
		maxGrowthCapacity int
		calls             int
	}{
		"growth capacity infinite": {
			calls: 1000,
		},
		"growth capacity 500": {
			maxGrowthCapacity: 500,
		},
	}

	for tn, test := range tests {
		t.Run(tn, func(t *testing.T) {
			coordinator := newRunnerMachinesCoordinator()

			wg := sync.WaitGroup{}
			calls := test.calls
			if calls == 0 {
				calls = test.maxGrowthCapacity * 2
			}
			wg.Add(calls)

			f, maxConcurrency := concurrencyTracker(1 * time.Second)
			for i := 0; i < calls; i++ {
				go func() {
					defer wg.Done()
					coordinator.waitForGrowthCapacity(test.maxGrowthCapacity, f)
				}()
			}

			wg.Wait()
			if test.maxGrowthCapacity > 0 {
				assert.Equal(t, test.maxGrowthCapacity, int(*maxConcurrency))
			} else {
				assert.Greater(t, int(*maxConcurrency), 0)
			}
		})
	}
}

func TestRunnerMachinesCoordinator_SignalMachineAvailable(t *testing.T) {
	t.Run("does not block", func(t *testing.T) {
		coordinator := newRunnerMachinesCoordinator()
		coordinator.addAvailableMachine()
	})

	t.Run("frees a waiting machine", func(t *testing.T) {
		coordinator := newRunnerMachinesCoordinator()
		readyToReceiveSignal := make(chan struct{})

		go func() {
			readyToReceiveSignal <- struct{}{}
			time.Sleep(time.Second)
			coordinator.addAvailableMachine()
		}()

		<-readyToReceiveSignal
		for !coordinator.getAvailableMachine() {
			time.Sleep(time.Second)
		}
	})
}

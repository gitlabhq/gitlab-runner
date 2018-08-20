package machine

import "sync"

// runnerMachinesCoordinator tracks the status of a specific Machine configuration, ensuring that the maximum number
// of concurrent machines being provisioned are limited.
type runnerMachinesCoordinator struct {
	growing        int
	growthCondLock sync.Mutex
	growthCond     *sync.Cond

	available chan struct{}
}

func newRunnerMachinesCoordinator() *runnerMachinesCoordinator {
	result := runnerMachinesCoordinator{
		available: make(chan struct{}),
	}
	result.growthCond = sync.NewCond(&result.growthCondLock)

	return &result
}

func (r *runnerMachinesCoordinator) waitForGrowthCapacity(maxGrowth int, f func()) {
	r.growthCondLock.Lock()
	for maxGrowth != 0 && r.growing >= maxGrowth {
		r.growthCond.Wait()
	}

	r.growing++
	r.growthCondLock.Unlock()

	defer func() {
		r.growthCondLock.Lock()
		r.growing--
		r.growthCondLock.Unlock()
		r.growthCond.Signal()
	}()

	f()
}

func (r *runnerMachinesCoordinator) availableMachineSignal() chan struct{} {
	return r.available
}

func (r *runnerMachinesCoordinator) signalMachineAvailable() {
	select {
	case r.available <- struct{}{}:
	default:
	}
}

type runnersDetails map[string]*runnerMachinesCoordinator

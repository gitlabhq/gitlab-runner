package machine

import (
	"sync"
)

// runnerMachinesCoordinator tracks the status of a specific Machine configuration, ensuring that the maximum number
// of concurrent machines being provisioned are limited.
type runnerMachinesCoordinator struct {
	growing        int
	growthCondLock sync.Mutex
	growthCond     *sync.Cond

	available     uint
	availableLock sync.Mutex

	availableSignal chan struct{}
}

func newRunnerMachinesCoordinator() *runnerMachinesCoordinator {
	coordinator := runnerMachinesCoordinator{}
	coordinator.availableSignal = make(chan struct{})
	coordinator.growthCond = sync.NewCond(&coordinator.growthCondLock)

	return &coordinator
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

// getAvailableMachine returns whether there is a machine available.
// It reduces the internal counter if it can be reduced so next time it might return
// a different value.
func (r *runnerMachinesCoordinator) getAvailableMachine() bool {
	r.availableLock.Lock()
	defer r.availableLock.Unlock()

	if r.available == 0 {
		return false
	}

	r.available--
	return true
}

// addAvailableMachine increments an internal counter which
// is used by getAvailableMachine to check for availability.
func (r *runnerMachinesCoordinator) addAvailableMachine() {
	r.availableLock.Lock()
	defer r.availableLock.Unlock()

	r.available++
	select {
	case r.availableSignal <- struct{}{}:
	default:
	}
}

func (r *runnerMachinesCoordinator) availableMachineSignal() <-chan struct{} {
	return r.availableSignal
}

type runnersDetails map[string]*runnerMachinesCoordinator

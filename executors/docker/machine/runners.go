package machine

import "sync"

// runnerMachinesCoordinator tracks the status of a specific Machine configuration, ensuring that the maximum number
// of concurrent machines being provisioned are limited.
type runnerMachinesCoordinator struct {
	growing        int
	growthCondLock sync.Mutex
	growthCond     *sync.Cond
}

func newRunnerMachinesCoordinator() *runnerMachinesCoordinator {
	result := runnerMachinesCoordinator{}
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

type runnersDetails map[string]*runnerMachinesCoordinator

package scaling

import (
	"sync"
	"time"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"errors"
)

type instanceState string

const (
	instanceUsing        instanceState = "using"
	instanceIdle         instanceState = "idle"
	instanceReinstalling instanceState = "reinstalling"
	instanceFailed       instanceState = "failed"
	instanceRemoving     instanceState = "removing"
	instanceRemoved      instanceState = "removed"
)

type instanceDetails struct {
	lock        sync.Mutex
	i           Instance
	state       instanceState
	created     time.Time
	lastUsed    time.Time
	usedCount   int
	removeTimer *time.Timer
}

func newInstanceDetails(instance Instance) (*instanceDetails) {
	details := &instanceDetails{
		i:       instance,
		state:   instanceIdle,
		created: time.Now(),
	}
	return details
}

func (i *instanceDetails) toState(state instanceState, currents... instanceState) bool {
	i.lock.Lock()
	defer i.lock.Unlock()

	for _, current := range currents {
		if current == i.state {
			i.state = state
			return true
		}
	}

	return false
}

func (i *instanceDetails) Use() bool {
	if !i.toState(instanceUsing, instanceIdle) {
		return false
	}

	i.state = instanceUsing
	i.usedCount++
	i.lastUsed = time.Now()

	if i.removeTimer != nil {
		i.removeTimer.Stop()
		i.removeTimer = nil
	}
	return true
}

func (i *instanceDetails) GetName() string {
	return i.i.GetName()
}

func (i *instanceDetails) GetIP() string {
	return i.i.GetIP()
}

func (i *instanceDetails) Reinstall(timeout time.Duration) error {
	if !i.toState(instanceReinstalling, instanceIdle) {
		return errors.New("instance needs to be used")
	}

	err := i.i.Reinstall(timeout)
	if err == nil {
		i.state = instanceIdle
	} else {
		i.state = instanceFailed
	}
	return err
}

func (i *instanceDetails) Remove(timeout time.Duration) {
	if !i.toState(instanceRemoving, instanceIdle, instanceFailed) {
		return
	}

	i.Remove(timeout)
	i.state = instanceRemoved
}

func (i *instanceDetails) Free(config *common.RunnerConfig) bool {
	if !i.toState(instanceIdle, instanceUsing) {
		return false
	}

	// no need to refresh machine
	if config.Machine.MaxBuilds >= i.usedCount {
		err := i.Reinstall(config.DigitalOcean.ReinstallTimeout)
		if err != nil {
			i.Remove(config.DigitalOcean.RemovalTimeout)
			return false
		}
	}

	if config.Machine.IdleTime != 0 {
		i.removeTimer = time.AfterFunc(
			time.Duration(config.Machine.IdleTime) * time.Second,
			func() {
				i.Remove(config.DigitalOcean.RemovalTimeout)
			},
		)
	}
	return true
}

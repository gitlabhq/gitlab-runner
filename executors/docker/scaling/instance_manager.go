package scaling

import (
	"errors"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

type instanceManager struct {
	provider *provider
	config   *common.RunnerConfig

	pool instancePool

	initialized bool
}

func (r *instanceManager) Create(config *common.RunnerConfig) error {
	r.config = config

	instances, err := r.provider.instanceProvider.List(config)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		details := newInstanceDetails(instance)
		details.usedCount = 1
		r.pool.Put(details)
	}

	r.initialized = true
	return nil
}

func (r *instanceManager) Destroy() {
	for {
		details := r.pool.Get()
		if details == nil {
			break
		}
		go details.Remove(r.config.DigitalOcean.RemovalTimeout)
	}

	r.initialized = false
}

func (r *instanceManager) Allocate() (details *instanceDetails, err error) {
	if !r.initialized {
		return nil, errors.New("not initialized")
	}

	// get machine from pool
	for details == nil {
		details = r.pool.Get()
		if details == nil {
			break
		} else if !details.Use() {
			details = nil
		}
	}

	// create a new machine
	if details == nil {
		instance, err := r.provider.instanceProvider.Create("dsadsa", r.config)
		if err != nil {
			return nil, err
		}

		details = newInstanceDetails(instance)
		if !details.Use() {
			return nil, errors.New("failed to use machine")
		}
	}
	return details, nil
}

func (r *instanceManager) Free(details *instanceDetails) {
	if !r.initialized {
		details.Remove(r.config.DigitalOcean.RemovalTimeout)
		return
	}

	if details.Free(r.config) {
		r.pool.Put(details)
	}
}

func (r *instanceManager) removeInstances() {

}

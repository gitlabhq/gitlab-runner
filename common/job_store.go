package common

import (
	"fmt"
)

type JobStoreProvider interface {
	// Name returns the name of the store the provider creates. The name must match the name set in the store config.
	Name() string
	// Get returns a store instance per runner. Stores are used to store data between separate manager runs. Get will always return a valid store when there's no error.
	Get(config *RunnerConfig) (JobStore, error)
}

type CompoundStoreProvider struct {
	providers []JobStoreProvider
}

func (c *CompoundStoreProvider) Name() string {
	return "unused"
}

func (c *CompoundStoreProvider) Get(config *RunnerConfig) (JobStore, error) {
	if config.Store == nil || config.Store.Name == "" {
		return NoopJobStore{}, nil
	}

	for _, provider := range c.providers {
		if provider.Name() == config.Store.Name {
			return provider.Get(config)
		}
	}

	return nil, fmt.Errorf("store %q is not supported by executor %q", config.Store.Name, config.Executor)
}

func NewCompoundStoreProvider(providers ...JobStoreProvider) *CompoundStoreProvider {
	return &CompoundStoreProvider{
		providers: providers,
	}
}

type JobStore interface {
	Request() (*Job, error)
	List() ([]*Job, error)
	Update(*Job) error
	Remove(*Job) error
}

type JobStoreUpdateType string

const (
	JobStoreUpdateHealth JobStoreUpdateType = "health"
	JobStoreUpdateRemove JobStoreUpdateType = "remove"
	JobStoreUpdateTrace  JobStoreUpdateType = "trace"
	JobStoreUpdateResume JobStoreUpdateType = "resume"
)

type JobStoreUpdate struct {
	ev        JobStoreUpdateType
	sentTrace int64
}

type NoopJobStore struct{}

func (n NoopJobStore) Request() (*Job, error) {
	return nil, nil
}

func (n NoopJobStore) Remove(_ *Job) error {
	return nil
}

func (n NoopJobStore) Update(_ *Job) error {
	return nil
}

func (n NoopJobStore) List() ([]*Job, error) {
	return nil, nil
}

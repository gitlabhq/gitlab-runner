package common

type JobStoreProvider interface {
	// Get returns a store instance per runner. Stores are used to store data between separate manager runs. Get will always return a valid store.
	Get(config *RunnerConfig) (JobStore, error)
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

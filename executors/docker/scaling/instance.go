package scaling

import (
	"time"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

type Instance interface {
	GetName() string
	GetIP() string
	Valid() bool
	Remove(timeout time.Duration) error
	Reinstall(timeout time.Duration) error
}

type InstanceProvider interface {
	Create(name string, config *common.RunnerConfig) (Instance, error)
	List(config *common.RunnerConfig) ([]Instance, error)
}

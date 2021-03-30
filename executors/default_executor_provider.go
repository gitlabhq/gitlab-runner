package executors

import (
	"errors"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type DefaultExecutorProvider struct {
	Creator          func() common.Executor
	FeaturesUpdater  func(features *common.FeaturesInfo)
	ConfigUpdater    func(input *common.RunnerConfig, output *common.ConfigInfo)
	DefaultShellName string
}

func (e DefaultExecutorProvider) CanCreate() bool {
	return e.Creator != nil
}

func (e DefaultExecutorProvider) Create() common.Executor {
	if e.Creator == nil {
		return nil
	}
	return e.Creator()
}

func (e DefaultExecutorProvider) Acquire(config *common.RunnerConfig) (common.ExecutorData, error) {
	return nil, nil
}

func (e DefaultExecutorProvider) Release(config *common.RunnerConfig, data common.ExecutorData) {}

func (e DefaultExecutorProvider) GetFeatures(features *common.FeaturesInfo) error {
	if e.FeaturesUpdater == nil {
		return errors.New("cannot evaluate features")
	}

	e.FeaturesUpdater(features)
	return nil
}

func (e DefaultExecutorProvider) GetConfigInfo(input *common.RunnerConfig, output *common.ConfigInfo) {
	if e.ConfigUpdater == nil {
		return
	}

	e.ConfigUpdater(input, output)
}

func (e DefaultExecutorProvider) GetDefaultShell() string {
	return e.DefaultShellName
}

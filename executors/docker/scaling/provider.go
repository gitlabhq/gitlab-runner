package scaling

import (
	"sync"

	"github.com/Sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

type provider struct {
	name             string
	executorProvider common.ExecutorProvider
	instanceProvider InstanceProvider

	lock             sync.RWMutex
	managers         map[string]*instanceManager
}

func (m *provider) instanceManager(config *common.RunnerConfig) *instanceManager {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.managers[config.UniqueID()]
}

func (m *provider) Acquire(config *common.RunnerConfig) (common.ExecutorData, error) {
	return nil, nil
}

func (m *provider) Release(config *common.RunnerConfig, data common.ExecutorData) error {
	return nil
}

func (m *provider) Reload(runners ...*common.RunnerConfig) {
	m.lock.Lock()
	defer m.lock.Unlock()

	providers := make(map[string]*instanceManager)

	for _, runner := range runners {
		provider := m.instanceManager(runner)
		if provider == nil {
			provider = &instanceManager{provider: m}
			err := provider.Create(runner)
			if err != nil {
				continue
			}
		}
		providers[runner.UniqueID()] = provider
	}

	for id, provider := range m.managers {
		if providers[id] != nil {
			continue
		}
		provider.Destroy()
	}

	m.managers = providers
}

func (m *provider) CanCreate() bool {
	return m.executorProvider.CanCreate()
}

func (m *provider) GetFeatures(features *common.FeaturesInfo) {
	m.executorProvider.GetFeatures(features)
}

func (m *provider) Create() common.Executor {
	return &instanceExecutor{
		provider: m,
	}
}

func NewMachineProvider(name, executor string, instanceProvider InstanceProvider) *provider {
	executorProvider := common.GetExecutor(executor)
	if executorProvider == nil {
		logrus.Panicln("Missing", executor)
	}

	return &provider{
		name:     name,
		executorProvider: executorProvider,
		instanceProvider: instanceProvider,
		managers: make(map[string]*instanceManager),
	}
}

package cache

import (
	"fmt"
	"sync"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

//go:generate mockery --name=CredentialsAdapter --inpackage
type CredentialsAdapter interface {
	GetCredentials() map[string]string
}

var credentialsFactories = &CredentialsFactoriesMap{}

func CredentialsFactories() *CredentialsFactoriesMap {
	return credentialsFactories
}

type CredentialsFactory func(config *common.CacheConfig) (CredentialsAdapter, error)

type CredentialsFactoriesMap struct {
	internal map[string]CredentialsFactory
	lock     sync.Mutex
}

func (m *CredentialsFactoriesMap) Register(typeName string, factory CredentialsFactory) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if len(m.internal) == 0 {
		m.internal = make(map[string]CredentialsFactory)
	}

	_, ok := m.internal[typeName]
	if ok {
		return fmt.Errorf("credentials adapter %q already registered", typeName)
	}

	m.internal[typeName] = factory

	return nil
}

func (m *CredentialsFactoriesMap) Find(typeName string) (CredentialsFactory, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	factory := m.internal[typeName]
	if factory == nil {
		return nil, fmt.Errorf("factory for credentials adapter %q not registered", typeName)
	}

	return factory, nil
}

func CreateCredentialsAdapter(cacheConfig *common.CacheConfig) (CredentialsAdapter, error) {
	create, err := CredentialsFactories().Find(cacheConfig.Type)
	if err != nil {
		return nil, fmt.Errorf("credentials adapter factory not found: %w", err)
	}

	adapter, err := create(cacheConfig)
	if err != nil {
		return nil, fmt.Errorf("credentials adapter could not be initialized: %w", err)
	}

	return adapter, nil
}

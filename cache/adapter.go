package cache

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

type PresignedURL struct {
	URL     *url.URL
	Headers http.Header
}

type GoCloudURL struct {
	URL *url.URL
	// Environment holds the environment variables needed to access the URL.
	Environment map[string]string
}

type Adapter interface {
	GetDownloadURL(context.Context) PresignedURL
	WithMetadata(map[string]string)
	GetUploadURL(context.Context) PresignedURL
	GetGoCloudURL(ctx context.Context, upload bool) (GoCloudURL, error)
}

type Factory func(config *cacheconfig.Config, timeout time.Duration, objectName string) (Adapter, error)

type FactoriesMap struct {
	internal map[string]Factory
	lock     sync.Mutex
}

func (m *FactoriesMap) Register(typeName string, factory Factory) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if len(m.internal) == 0 {
		m.internal = make(map[string]Factory)
	}

	_, ok := m.internal[typeName]
	if ok {
		return fmt.Errorf("adapter %q already registered", typeName)
	}

	m.internal[typeName] = factory

	return nil
}

func (m *FactoriesMap) Find(typeName string) (Factory, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	factory := m.internal[typeName]
	if factory == nil {
		return nil, fmt.Errorf("factory for cache adapter %q was not registered", typeName)
	}

	return factory, nil
}

var factories = &FactoriesMap{}

func Factories() *FactoriesMap {
	return factories
}

func getCreateAdapter(cacheConfig *cacheconfig.Config, timeout time.Duration, objectName string) (Adapter, error) {
	create, err := Factories().Find(cacheConfig.Type)
	if err != nil {
		return nil, fmt.Errorf("cache factory not found: %w", err)
	}

	adapter, err := create(cacheConfig, timeout, objectName)
	if err != nil {
		return nil, fmt.Errorf("cache adapter could not be initialized: %w", err)
	}

	return adapter, nil
}

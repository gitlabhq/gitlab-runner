package executors

import (
	"iter"
	"maps"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type Providers interface {
	// GetByName returns nil if the provider is not found.
	GetByName(name string) common.ExecutorProvider
	All() iter.Seq2[string, common.ExecutorProvider]
}

type ProviderRegistry struct {
	providers map[string]common.ExecutorProvider
}

func NewProviderRegistry(providers map[string]common.ExecutorProvider) *ProviderRegistry {
	return &ProviderRegistry{
		providers: providers,
	}
}

func (r *ProviderRegistry) GetByName(name string) common.ExecutorProvider {
	return r.providers[name]
}

func (r *ProviderRegistry) All() iter.Seq2[string, common.ExecutorProvider] {
	return maps.All(r.providers)
}

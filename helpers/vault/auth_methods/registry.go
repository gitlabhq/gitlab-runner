package auth_methods

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/internal/registry"
)

type Factory func(path string, data Data) (vault.AuthMethod, error)

var factoriesRegistry = registry.New("auth method")

func MustRegisterFactory(authName string, factory Factory) {
	err := factoriesRegistry.Register(authName, factory)
	if err != nil {
		panic(fmt.Sprintf("registering factory: %v", err))
	}
}

func GetFactory(authName string) (Factory, error) {
	factory, err := factoriesRegistry.Get(authName)
	if err != nil {
		return nil, err
	}

	return factory.(Factory), nil
}

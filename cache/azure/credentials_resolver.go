package azure

import (
	"errors"
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

type credentialsResolver interface {
	Resolve() error
	Signer() (sasSigner, error)
}

type defaultCredentialsResolver struct {
	config *cacheconfig.CacheAzureConfig
}

func (cr *defaultCredentialsResolver) Resolve() error {
	return cr.readCredentialsFromConfig()
}

func (cr *defaultCredentialsResolver) Credentials() *cacheconfig.CacheAzureCredentials {
	return &cr.config.CacheAzureCredentials
}

func (cr *defaultCredentialsResolver) Signer() (sasSigner, error) {
	if cr.config.AccountName == "" {
		return nil, errors.New("missing Azure storage account name")
	}
	if cr.config.ContainerName == "" {
		return nil, errors.New("ContainerName can't be empty")
	}
	if cr.config.CacheAzureCredentials.AccountKey != "" {
		return newAccountKeySigner(cr.config)
	}

	return newUserDelegationKeySigner(cr.config)
}

func (cr *defaultCredentialsResolver) readCredentialsFromConfig() error {
	if cr.config.AccountName == "" {
		return fmt.Errorf("config for Azure present, but account name is not configured")
	}

	return nil
}

func newDefaultCredentialsResolver(config *cacheconfig.CacheAzureConfig) (*defaultCredentialsResolver, error) {
	if config == nil {
		return nil, fmt.Errorf("config can't be nil")
	}

	resolver := &defaultCredentialsResolver{
		config: config,
	}

	return resolver, nil
}

var credentialsResolverInitializer = newDefaultCredentialsResolver

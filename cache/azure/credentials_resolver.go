package azure

import (
	"errors"
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

//go:generate mockery --name=credentialsResolver --inpackage
type credentialsResolver interface {
	Resolve() error
	Signer() (sasSigner, error)
}

type defaultCredentialsResolver struct {
	config      *common.CacheAzureConfig
	credentials *common.CacheAzureCredentials
}

func (cr *defaultCredentialsResolver) Resolve() error {
	return cr.readCredentialsFromConfig()
}

func (cr *defaultCredentialsResolver) Credentials() *common.CacheAzureCredentials {
	return cr.credentials
}

func (cr *defaultCredentialsResolver) Signer() (sasSigner, error) {
	if cr.config.AccountName == "" {
		return nil, errors.New("missing Azure storage account name")
	}
	if cr.config.ContainerName == "" {
		return nil, errors.New("ContainerName can't be empty")
	}
	if cr.credentials.AccountKey != "" {
		return newAccountKeySigner(cr.config)
	}

	return newUserDelegationKeySigner(cr.config)
}

func (cr *defaultCredentialsResolver) readCredentialsFromConfig() error {
	if cr.config.AccountName == "" {
		return fmt.Errorf("config for Azure present, but account name is not configured")
	}

	cr.credentials.AccountName = cr.config.AccountName
	cr.credentials.AccountKey = cr.config.AccountKey

	return nil
}

func newDefaultCredentialsResolver(config *common.CacheAzureConfig) (*defaultCredentialsResolver, error) {
	if config == nil {
		return nil, fmt.Errorf("config can't be nil")
	}

	credentials := &defaultCredentialsResolver{
		config:      config,
		credentials: &common.CacheAzureCredentials{},
	}

	return credentials, nil
}

var credentialsResolverInitializer = newDefaultCredentialsResolver

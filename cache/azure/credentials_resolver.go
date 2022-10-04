package azure

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

//go:generate mockery --name=credentialsResolver --inpackage
type credentialsResolver interface {
	Credentials() *common.CacheAzureCredentials
	Resolve() error
}

type defaultCredentialsResolver struct {
	config      *common.CacheAzureConfig
	credentials *common.CacheAzureCredentials
}

func (cr *defaultCredentialsResolver) Credentials() *common.CacheAzureCredentials {
	return cr.credentials
}

func (cr *defaultCredentialsResolver) Resolve() error {
	return cr.readCredentialsFromConfig()
}

func (cr *defaultCredentialsResolver) readCredentialsFromConfig() error {
	if cr.config.AccountName == "" || cr.config.AccountKey == "" {
		return fmt.Errorf("config for Azure present, but credentials are not configured")
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

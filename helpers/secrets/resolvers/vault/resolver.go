package vault

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/service"
)

const (
	resolverName = "vault"
)

var newVaultService = service.NewVault

type resolver struct {
	secret common.Secret
}

func newResolver(secret common.Secret) common.SecretResolver {
	return &resolver{
		secret: secret,
	}
}

func (v *resolver) Name() string {
	return resolverName
}

func (v *resolver) IsSupported() bool {
	return v.secret.Vault != nil
}

func (v *resolver) Resolve(variableKey string) (*common.JobVariable, error) {
	if !v.IsSupported() {
		return nil, secrets.NewResolvingUnsupportedSecretError(resolverName)
	}

	secret := v.secret.Vault

	url := secret.Server.URL

	s, err := newVaultService(url, secret)
	if err != nil {
		return nil, err
	}

	data, err := s.GetField(secret, secret)
	if err != nil {
		return nil, err
	}

	variable := &common.JobVariable{
		Key:   variableKey,
		Value: fmt.Sprintf("%v", data),
		File:  true,
	}

	return variable, nil
}

func init() {
	common.GetSecretResolverRegistry().Register(newResolver)
}

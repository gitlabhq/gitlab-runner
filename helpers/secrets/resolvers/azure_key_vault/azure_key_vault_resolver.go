package azure_key_vault

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/azure_key_vault/service"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
)

const (
	resolverName = "azure-key-vault"
)

var newVaultService = service.NewAzureKeyVault

type azureKeyVaultResolver struct {
	secret common.Secret
}

func newResolver(secret common.Secret) common.SecretResolver {
	return &azureKeyVaultResolver{
		secret: secret,
	}
}

func (v *azureKeyVaultResolver) Name() string {
	return resolverName
}

func (v *azureKeyVaultResolver) IsSupported() bool {
	return v.secret.AzureKeyVault != nil
}

func (v *azureKeyVaultResolver) Resolve() (string, error) {
	if !v.IsSupported() {
		return "", secrets.NewResolvingUnsupportedSecretError(resolverName)
	}

	secret := v.secret.AzureKeyVault
	s, err := newVaultService(secret.Server)
	if err != nil {
		return "", err
	}

	name := secret.Name
	version := secret.Version

	data, err := s.GetSecret(name, version)

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%v", data), nil
}

func init() {
	common.GetSecretResolverRegistry().Register(newResolver)
}

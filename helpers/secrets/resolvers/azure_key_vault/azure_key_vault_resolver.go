package azure_key_vault

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/azure_key_vault/service"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
)

const (
	resolverName = "azure-key-vault"
)

var newVaultService = service.NewAzureKeyVault

type azureKeyVaultResolver struct {
	secret spec.Secret
}

func newResolver(secret spec.Secret) common.SecretResolver {
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
		return "", classifyError(err)
	}

	name := secret.Name
	version := secret.Version

	data, err := s.GetSecret(name, version)

	if err != nil {
		return "", classifyError(err)
	}

	return fmt.Sprintf("%v", data), nil
}

// classifyError reports RBAC/lookup failures on the requested secret as a
// configuration error and 5xx failures as an external dependency error, so
// the caller can report an accurate job failure reason instead of a generic
// runner system failure. 401 keeps the existing classification: it means
// the runner- or admin-configured trust to reach Key Vault is broken, not a
// job misconfiguration. Any other failure is also returned unmodified.
func classifyError(err error) error {
	if err == nil {
		return nil
	}

	var respErr *azcore.ResponseError
	if !errors.As(err, &respErr) {
		return err
	}

	switch {
	case respErr.StatusCode == http.StatusForbidden, respErr.StatusCode == http.StatusNotFound:
		return secrets.NewResolvingConfigurationError(err)
	case respErr.StatusCode >= 500:
		return secrets.NewResolvingExternalDependencyError(err)
	default:
		return err
	}
}

func init() {
	common.GetSecretResolverRegistry().Register(newResolver)
}

package vault

import (
	"errors"
	"fmt"
	"net/http"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/service"
)

const (
	resolverName = "vault"
)

var newVaultService = service.NewVault

type resolver struct {
	secret spec.Secret
}

func newResolver(secret spec.Secret) common.SecretResolver {
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

func (v *resolver) Resolve() (string, error) {
	if !v.IsSupported() {
		return "", secrets.NewResolvingUnsupportedSecretError(resolverName)
	}

	secret := v.secret.Vault

	url := secret.Server.URL
	namespace := secret.Server.Namespace

	s, err := newVaultService(url, namespace, secret)
	if err != nil {
		return "", classifyError(err)
	}

	data, err := s.GetField(secret, secret)
	if err != nil {
		return "", classifyError(err)
	}

	if data == nil {
		return "", common.ErrSecretNotFound
	}

	return fmt.Sprintf("%v", data), nil
}

// apiStatusCoder is implemented by Vault API errors that carry the HTTP
// status code of the API response.
type apiStatusCoder interface {
	StatusCode() int
}

// classifyError inspects a Vault API failure and wraps it with a semantic
// classification, so that the caller can report an accurate job failure
// reason instead of a generic runner system failure:
//
//   - client errors caused by how the job or Vault is configured
//     (e.g. 403 permission denied for the requested path, 400 missing
//     role, 404 unknown path) become a configuration failure
//   - server-side Vault failures (5xx) become an external dependency
//     failure
//
// Any other failure (network errors, timeouts, unexpected statuses such
// as 429) is returned unmodified and keeps the existing classification.
func classifyError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr apiStatusCoder
	if !errors.As(err, &apiErr) {
		return err
	}

	switch code := apiErr.StatusCode(); {
	case code == http.StatusBadRequest,
		code == http.StatusUnauthorized,
		code == http.StatusForbidden,
		code == http.StatusNotFound:
		return secrets.NewResolvingConfigurationError(err)
	case code >= 500:
		return secrets.NewResolvingExternalDependencyError(err)
	default:
		return err
	}
}

func init() {
	common.GetSecretResolverRegistry().Register(newResolver)
}

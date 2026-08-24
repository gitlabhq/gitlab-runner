package gitlab_secrets_manager

import (
	"errors"
	"fmt"
	"net/http"
	"path"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/gitlab_secrets_manager/service"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
)

const resolverName = "gitlab_secrets_manager"

type resolver struct {
	secret spec.Secret
}

func newResolver(secret spec.Secret) common.SecretResolver {
	return &resolver{
		secret: secret,
	}
}

func (r *resolver) Name() string {
	return resolverName
}

func (r *resolver) IsSupported() bool {
	return r.secret.GitLabSecretsManager != nil
}

func (r *resolver) Resolve() (string, error) {
	if !r.IsSupported() {
		return "", secrets.NewResolvingUnsupportedSecretError(resolverName)
	}

	gsmSecret := r.secret.GitLabSecretsManager

	// When path exists, prefer it over templating a fixed path based on
	// AuthMount. Note that AuthMount does not allow control over additional
	// auth paths (e.g., cel/login) or namespaces (which prefix the path,
	// i.e., (<namespace>/auth/<auth_mount>/login).
	//
	// While commonly true, login requests do not necessarily always go to
	// a path called login.
	loginPath := gsmSecret.Server.InlineAuth.Path
	if loginPath == "" {
		loginPath = path.Join("auth", gsmSecret.Server.InlineAuth.AuthMount, "login")
	}

	client, err := vault.NewClient(
		gsmSecret.Server.URL,
		"",
		vault.WithInlineAuth(
			&vault.InlineAuth{
				Path: loginPath,
				JWT:  gsmSecret.Server.InlineAuth.JWT,
				Role: gsmSecret.Server.InlineAuth.Role,
			},
		),
	)
	if err != nil {
		return "", classifyError(fmt.Errorf("creating vault client: %w", err))
	}

	value, err := service.NewGitlabSecretsManager(client).GetSecret(gsmSecret)
	if err != nil {
		return "", classifyError(fmt.Errorf("getting secret: %w", err))
	}

	return value, nil
}

// apiStatusCoder is implemented by the Vault API errors that back this
// resolver, carrying the HTTP status code of the API response.
type apiStatusCoder interface {
	StatusCode() int
}

// classifyError mirrors the Vault resolver's classification, since the auth
// role and secret path here are configured directly by the job: 4xx becomes
// a configuration error, 5xx an external dependency error, so the caller can
// report an accurate job failure reason instead of a generic runner system
// failure. Any other failure is returned unmodified.
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

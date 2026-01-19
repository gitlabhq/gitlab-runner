package gitlab_secrets_manager

import (
	"fmt"
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
		return "", fmt.Errorf("creating vault client: %w", err)
	}

	value, err := service.NewGitlabSecretsManager(client).GetSecret(gsmSecret)
	if err != nil {
		return "", fmt.Errorf("getting secret: %w", err)
	}

	return value, nil
}

func init() {
	common.GetSecretResolverRegistry().Register(newResolver)
}

package service

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/secret_engines"
)

type GitLabSecretsManager struct {
	client vault.Client
}

func NewGitlabSecretsManager(client vault.Client) *GitLabSecretsManager {
	return &GitLabSecretsManager{
		client: client,
	}
}

func (service *GitLabSecretsManager) GetSecret(secret *common.GitLabSecretsManagerSecret) (string, error) {
	engineFactory, err := secret_engines.GetFactory(secret.Engine.Name)
	if err != nil {
		return "", fmt.Errorf("getting secret engine: %w", err)
	}

	engine := engineFactory(service.client, secret.Engine.Path)

	data, err := engine.Get(secret.Path)
	if err != nil {
		return "", fmt.Errorf("get secret data: %w", err)
	}

	if data == nil {
		return "", common.ErrSecretNotFound
	}

	value, exists := data[secret.Field]
	if !exists {
		return "", fmt.Errorf("field %q not found in secret", secret.Field)
	}

	stringValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("field %q has invalid type %T (expected string)", secret.Field, value)
	}

	return stringValue, nil
}

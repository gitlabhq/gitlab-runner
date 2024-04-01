package akeyless

import (
	"context"
	"encoding/json"
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/akeyless/service"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
)

const (
	resolverName = "akeyless"
)

type akeylessResolver struct {
	secret   common.Secret
	akeyless service.Akeyless
}

func newResolver(secret common.Secret) common.SecretResolver {
	return &akeylessResolver{
		secret:   secret,
		akeyless: service.NewAkeyless(),
	}
}

func init() {
	common.GetSecretResolverRegistry().Register(newResolver)
}

func (a *akeylessResolver) Name() string {
	return resolverName
}

func (a *akeylessResolver) IsSupported() bool {
	return a.secret.Akeyless != nil
}

func (a *akeylessResolver) Resolve() (string, error) {
	if !a.IsSupported() {
		return "", secrets.NewResolvingUnsupportedSecretError(resolverName)
	}

	ctx := context.Background()
	secret := a.secret.Akeyless

	data, err := a.akeyless.GetAkeylessSecret(ctx, secret)
	if err != nil {
		return "", err
	}

	if secret.DataKey != "" {
		data, err = extractKeyFromJSON(data, secret.DataKey)
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%v", data), nil
}

func extractKeyFromJSON(data any, key string) (string, error) {
	if data == nil {
		return "", fmt.Errorf("input data is not a valid JSON string")
	}
	dataStr, ok := data.(string)
	if !ok {
		return "", fmt.Errorf("input data is not a valid JSON string")
	}

	var obj map[string]interface{}
	err := json.Unmarshal([]byte(dataStr), &obj)
	if err != nil {
		return "", fmt.Errorf("failed to extract a key out of the json data: %v", err)
	}

	val, ok := obj[key].(string)
	if !ok {
		return "", fmt.Errorf("failed to extract a key out of the json data, the key %v does not exist", key)
	}

	return val, nil
}

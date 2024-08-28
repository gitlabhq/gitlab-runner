package akeyless

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	secret := a.secret.Akeyless

	data, err := a.akeyless.GetAkeylessSecret(ctx, secret)
	if err != nil {
		return "", err
	}

	if secret.DataKey == "" {
		return fmt.Sprintf("%v", data), nil
	}

	value, err := extractKeyFromJSON(data, secret.DataKey)
	if err != nil {
		return "", err
	}

	return value, nil
}

func extractKeyFromJSON(data any, key string) (string, error) {
	if data == nil {
		return "", fmt.Errorf("input data is not a valid JSON string")
	}

	dataStr, ok := data.(string)
	if !ok {
		return "", fmt.Errorf("input data is not a valid string")
	}

	var obj map[string]any
	err := json.Unmarshal([]byte(dataStr), &obj)
	if err != nil {
		return "", fmt.Errorf("failed to extract a key out of the json data: %v", err)
	}

	val, ok := obj[key]
	if !ok {
		return "", fmt.Errorf("failed to extract a key out of the json data, the key %v does not exist", key)
	}

	valStr, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("failed to extract a key out of the json data, the key %v is not a string", key)
	}

	return valStr, nil
}

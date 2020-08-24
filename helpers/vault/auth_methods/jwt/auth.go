package jwt

import (
	"fmt"
	"path"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/auth_methods"
)

const methodName = "jwt"

const (
	jwtKey  = "jwt"
	roleKey = "role"
)

var (
	requiredPayloadFields = []string{
		jwtKey,
	}

	allowedPayloadFields = []string{
		jwtKey,
		roleKey,
	}
)

type method struct {
	path string
	data map[string]interface{}

	token string
}

func NewMethod(path string, data auth_methods.Data) (vault.AuthMethod, error) {
	newData, err := data.Filter(requiredPayloadFields, allowedPayloadFields)
	if err != nil {
		return nil, fmt.Errorf("filtering auth method configuration: %w", err)
	}

	a := &method{
		path: path,
		data: newData,
	}

	return a, nil
}

func (a *method) Name() string {
	return methodName
}

func (a *method) Authenticate(client vault.Client) error {
	authPath := path.Join("auth", a.path, "login")
	authPayload := a.data

	result, err := client.Write(authPath, authPayload)
	if err != nil {
		return fmt.Errorf("writing to Vault: %w", err)
	}

	token, err := result.TokenID()
	if err != nil {
		return fmt.Errorf("getting token from the authentication response: %w", err)
	}

	a.token = token

	return nil
}

func (a *method) Token() string {
	return a.token
}

func init() {
	auth_methods.MustRegisterFactory(methodName, NewMethod)
}

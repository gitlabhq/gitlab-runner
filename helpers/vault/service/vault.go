package service

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/auth_methods"
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/vault/auth_methods/jwt" // register auth method
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/secret_engines"
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/vault/secret_engines/kv_v1" // register secret engine
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/vault/secret_engines/kv_v2" // register secret engine
)

//go:generate mockery --name=Auth --inpackage
type Auth interface {
	AuthName() string
	AuthPath() string
	AuthData() auth_methods.Data
}

//go:generate mockery --name=Engine --inpackage
type Engine interface {
	EngineName() string
	EnginePath() string
}

//go:generate mockery --name=Secret --inpackage
type Secret interface {
	SecretPath() string
	SecretField() string
}

//go:generate mockery --name=Vault --inpackage
type Vault interface {
	GetField(engineDetails Engine, secretDetails Secret) (interface{}, error)
	Put(engineDetails Engine, secretDetails Secret, data map[string]interface{}) error
	Delete(engineDetails Engine, secretDetails Secret) error
}

type defaultVault struct {
	client vault.Client
}

var newVaultClient = vault.NewClient

func NewVault(url string, namespace string, auth Auth) (Vault, error) {
	v := new(defaultVault)

	err := v.initialize(url, namespace, auth)
	if err != nil {
		return nil, fmt.Errorf("initializing Vault service: %w", err)
	}

	return v, nil
}

func (v *defaultVault) initialize(url string, namespace string, auth Auth) error {
	err := v.prepareAuthenticatedClient(url, namespace, auth)
	if err != nil {
		return fmt.Errorf("preparing authenticated client: %w", err)
	}

	return nil
}

func (v *defaultVault) prepareAuthenticatedClient(url string, namespace string, authDetails Auth) error {
	client, err := newVaultClient(url, namespace)
	if err != nil {
		return err
	}

	auth, err := v.prepareAuthMethodAdapter(authDetails)
	if err != nil {
		return err
	}

	err = client.Authenticate(auth)
	if err != nil {
		return err
	}

	v.client = client

	return nil
}

func (v *defaultVault) prepareAuthMethodAdapter(authDetails Auth) (vault.AuthMethod, error) {
	authFactory, err := auth_methods.GetFactory(authDetails.AuthName())
	if err != nil {
		return nil, fmt.Errorf("initializing auth method factory: %w", err)
	}

	auth, err := authFactory(authDetails.AuthPath(), authDetails.AuthData())
	if err != nil {
		return nil, fmt.Errorf("initializing auth method adapter: %w", err)
	}

	return auth, nil
}

func (v *defaultVault) GetField(engineDetails Engine, secretDetails Secret) (interface{}, error) {
	engine, err := v.getSecretEngine(engineDetails)
	if err != nil {
		return nil, err
	}

	secret, err := engine.Get(secretDetails.SecretPath())
	if err != nil {
		return nil, fmt.Errorf("reading secret: %w", err)
	}

	field := secretDetails.SecretField()
	for key, data := range secret {
		if key != field {
			continue
		}

		return data, nil
	}

	return nil, nil
}

func (v *defaultVault) getSecretEngine(engineDetails Engine) (vault.SecretEngine, error) {
	engineFactory, err := secret_engines.GetFactory(engineDetails.EngineName())
	if err != nil {
		return nil, fmt.Errorf("requesting SecretEngine factory: %w", err)
	}

	engine := engineFactory(v.client, engineDetails.EnginePath())

	return engine, nil
}

func (v *defaultVault) Put(engineDetails Engine, secretDetails Secret, data map[string]interface{}) error {
	engine, err := v.getSecretEngine(engineDetails)
	if err != nil {
		return err
	}

	err = engine.Put(secretDetails.SecretPath(), data)
	if err != nil {
		return fmt.Errorf("writing secret: %w", err)
	}

	return nil
}

func (v *defaultVault) Delete(engineDetails Engine, secretDetails Secret) error {
	engine, err := v.getSecretEngine(engineDetails)
	if err != nil {
		return err
	}

	err = engine.Delete(secretDetails.SecretPath())
	if err != nil {
		return fmt.Errorf("deleting secret: %w", err)
	}

	return nil
}

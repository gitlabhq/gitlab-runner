package generic

import (
	"fmt"
	"path"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/secret_engines"
)

type engine struct {
	client vault.Client
	path   string
	name   string
}

func engineForName(engineName string) func(vault.Client, string) vault.SecretEngine {
	return func(client vault.Client, path string) vault.SecretEngine {
		return &engine{
			client: client,
			path:   path,
			name:   engineName,
		}
	}
}

func (e *engine) EngineName() string {
	return e.name
}

func (e *engine) Get(path string) (map[string]interface{}, error) {
	secret, err := e.client.Read(e.fullPath(path))
	if err != nil {
		return nil, fmt.Errorf("reading from Vault: %w", err)
	}

	return secret.Data(), nil
}

func (e *engine) fullPath(p string) string {
	return path.Join(e.path, p)
}

func (e *engine) Put(path string, data map[string]interface{}) error {
	_, err := e.client.Write(e.fullPath(path), data)
	if err != nil {
		return fmt.Errorf("writing to Vault: %w", err)
	}

	return nil
}

func (e *engine) Delete(path string) error {
	err := e.client.Delete(e.fullPath(path))
	if err != nil {
		return fmt.Errorf("deleting from Vault: %w", err)
	}

	return nil
}

func init() {
	secret_engines.MustRegisterFactory("generic", engineForName("generic"))
	secret_engines.MustRegisterFactory("kv-v1", engineForName("kv-v1"))
}

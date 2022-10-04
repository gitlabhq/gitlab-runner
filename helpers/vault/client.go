package vault

import (
	"errors"
	"fmt"

	"github.com/hashicorp/vault/api"
)

//go:generate mockery --name=Client --inpackage
type Client interface {
	Authenticate(auth AuthMethod) error
	Write(path string, data map[string]interface{}) (Result, error)
	Read(path string) (Result, error)
	Delete(path string) error
}

type defaultClient struct {
	internal apiClient
}

//go:generate mockery --name=apiClient --inpackage
type apiClient interface {
	Sys() apiClientSys
	Logical() apiClientLogical
	SetToken(v string)
	SetNamespace(ns string)
}

//go:generate mockery --name=apiClientSys --inpackage
type apiClientSys interface {
	Health() (*api.HealthResponse, error)
}

//go:generate mockery --name=apiClientLogical --inpackage
type apiClientLogical interface {
	Write(path string, data map[string]interface{}) (*api.Secret, error)
	Read(path string) (*api.Secret, error)
	Delete(path string) (*api.Secret, error)
}

type apiClientAdapter struct {
	c *api.Client
}

func (c *apiClientAdapter) Sys() apiClientSys {
	return c.c.Sys()
}

func (c *apiClientAdapter) Logical() apiClientLogical {
	return c.c.Logical()
}

func (c *apiClientAdapter) SetToken(v string) {
	c.c.SetToken(v)
}

func (c *apiClientAdapter) SetNamespace(ns string) {
	c.c.SetNamespace(ns)
}

var (
	ErrVaultServerNotReady = errors.New("not initialized or sealed Vault server")

	newAPIClient = func(config *api.Config) (apiClient, error) {
		c, err := api.NewClient(config)
		if err != nil {
			return nil, err
		}

		return &apiClientAdapter{c: c}, nil
	}
)

func NewClient(URL string, namespace string) (Client, error) {
	config := &api.Config{
		Address: URL,
	}

	client, err := newAPIClient(config)
	if err != nil {
		return nil, fmt.Errorf("creating new Vault client: %w", unwrapAPIResponseError(err))
	}

	healthResp, err := client.Sys().Health()
	if err != nil {
		return nil, fmt.Errorf("checking Vault server health: %w", unwrapAPIResponseError(err))
	}

	if !healthResp.Initialized || healthResp.Sealed {
		return nil, ErrVaultServerNotReady
	}

	client.SetNamespace(namespace)

	c := &defaultClient{
		internal: client,
	}

	return c, nil
}

func (c *defaultClient) Authenticate(auth AuthMethod) error {
	err := auth.Authenticate(c)
	if err != nil {
		return fmt.Errorf("authenticating Vault client: %w", err)
	}

	c.internal.SetToken(auth.Token())

	return nil
}

func (c *defaultClient) Write(path string, data map[string]interface{}) (Result, error) {
	secret, err := c.internal.Logical().Write(path, data)

	return newResult(secret), unwrapAPIResponseError(err)
}

func (c *defaultClient) Read(path string) (Result, error) {
	secret, err := c.internal.Logical().Read(path)

	return newResult(secret), unwrapAPIResponseError(err)
}

func (c *defaultClient) Delete(path string) error {
	_, err := c.internal.Logical().Delete(path)

	return unwrapAPIResponseError(err)
}

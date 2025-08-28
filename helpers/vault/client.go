package vault

import (
	"errors"
	"fmt"

	"github.com/openbao/openbao/api/v2"
)

type Client interface {
	Authenticate(auth AuthMethod) error
	Write(path string, data map[string]interface{}) (Result, error)
	Read(path string) (Result, error)
	Delete(path string) error
}

type defaultClient struct {
	internal *api.Client
}

type InlineAuth struct {
	Path string
	JWT  string
	Role string
}

type ClientOption func(*api.Client) (*api.Client, error)

func WithInlineAuth(auth *InlineAuth) ClientOption {
	return func(c *api.Client) (*api.Client, error) {
		var errs error
		if auth == nil {
			errs = errors.Join(errs, errors.New("inline auth is required"))
		} else {
			if auth.Path == "" {
				errs = errors.Join(errs, errors.New("inline auth path is required"))
			}
			if auth.JWT == "" {
				errs = errors.Join(errs, errors.New("inline auth JWT is required"))
			}
			if auth.Role == "" {
				errs = errors.Join(errs, errors.New("inline auth role is required"))
			}
		}

		if errs != nil {
			return nil, fmt.Errorf("configuring inline auth: %w", errs)
		}

		data := map[string]interface{}{
			"jwt":  auth.JWT,
			"role": auth.Role,
		}

		var err error
		c, err = c.WithInlineAuth(auth.Path, data)
		if err != nil {
			return nil, fmt.Errorf("configuring inline auth: %w", unwrapAPIResponseError(err))
		}

		return c, nil
	}
}

func NewClient(apiURL string, namespace string, opts ...ClientOption) (Client, error) {
	client, err := api.NewClient(&api.Config{Address: apiURL})
	if err != nil {
		return nil, fmt.Errorf("creating new Vault client: %w", unwrapAPIResponseError(err))
	}

	client.SetNamespace(namespace)

	for _, opt := range opts {
		client, err = opt(client)
		if err != nil {
			return nil, err
		}
	}

	return &defaultClient{
		internal: client,
	}, nil
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

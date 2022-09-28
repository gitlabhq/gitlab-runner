package vault

import (
	"errors"

	"github.com/hashicorp/vault/api"
)

//go:generate mockery --name=Result --inpackage
type Result interface {
	Data() map[string]interface{}
	TokenID() (string, error)
}

var ErrNoResult = errors.New("no result from Vault")

type secretResult struct {
	inner *api.Secret
}

func newResult(secret *api.Secret) Result {
	return &secretResult{
		inner: secret,
	}
}

func (r *secretResult) Data() map[string]interface{} {
	if r.inner == nil {
		return nil
	}

	return r.inner.Data
}

func (r *secretResult) TokenID() (string, error) {
	if r.inner == nil {
		return "", ErrNoResult
	}

	return r.inner.TokenID()
}

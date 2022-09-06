//go:build !integration

package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolvingUnsupportedSecretError_Error(t *testing.T) {
	err := NewResolvingUnsupportedSecretError("test")
	assert.Equal(t, "trying to resolve unsupported secret: test", err.Error())
}

func TestResolvingUnsupportedSecretError_Is(t *testing.T) {
	assert.ErrorIs(
		t,
		NewResolvingUnsupportedSecretError("expected"),
		NewResolvingUnsupportedSecretError("expected"),
	)
	assert.NotErrorIs(t, NewResolvingUnsupportedSecretError("expected"), new(ResolvingUnsupportedSecretError))
	assert.NotErrorIs(t, NewResolvingUnsupportedSecretError("expected"), assert.AnError)
}

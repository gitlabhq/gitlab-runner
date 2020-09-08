package secrets

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolvingUnsupportedSecretError_Error(t *testing.T) {
	err := NewResolvingUnsupportedSecretError("test")
	assert.Equal(t, "trying to resolve unsupported secret: test", err.Error())
}

func TestResolvingUnsupportedSecretError_Is(t *testing.T) {
	assert.True(
		t,
		errors.Is(
			NewResolvingUnsupportedSecretError("expected"),
			NewResolvingUnsupportedSecretError("expected"),
		),
	)
	assert.False(t, errors.Is(NewResolvingUnsupportedSecretError("expected"), new(ResolvingUnsupportedSecretError)))
	assert.False(t, errors.Is(NewResolvingUnsupportedSecretError("expected"), assert.AnError))
}

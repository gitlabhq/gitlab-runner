package secret_engines

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
)

func TestOperationNotSupportedError_Error(t *testing.T) {
	e := new(vault.MockSecretEngine)
	e.On("EngineName").
		Return("test-engine").
		Times(3)

	assert.Equal(
		t,
		`operation "get" for secret engine "test-engine" is not supported`,
		NewUnsupportedGetOperationErr(e).Error(),
	)
	assert.Equal(
		t,
		`operation "put" for secret engine "test-engine" is not supported`,
		NewUnsupportedPutOperationErr(e).Error(),
	)
	assert.Equal(
		t,
		`operation "delete" for secret engine "test-engine" is not supported`,
		NewUnsupportedDeleteOperationErr(e).Error(),
	)
}

func TestOperationNotSupportedError_Is(t *testing.T) {
	e := new(vault.MockSecretEngine)
	e.On("EngineName").Return("test-engine")

	assert.True(t, errors.Is(NewUnsupportedGetOperationErr(e), NewUnsupportedGetOperationErr(e)))
	assert.False(t, errors.Is(NewUnsupportedGetOperationErr(e), new(OperationNotSupportedError)))
	assert.False(t, errors.Is(NewUnsupportedGetOperationErr(e), assert.AnError))

	assert.True(t, errors.Is(NewUnsupportedPutOperationErr(e), NewUnsupportedPutOperationErr(e)))
	assert.False(t, errors.Is(NewUnsupportedPutOperationErr(e), new(OperationNotSupportedError)))
	assert.False(t, errors.Is(NewUnsupportedPutOperationErr(e), assert.AnError))

	assert.True(t, errors.Is(NewUnsupportedDeleteOperationErr(e), NewUnsupportedDeleteOperationErr(e)))
	assert.False(t, errors.Is(NewUnsupportedDeleteOperationErr(e), new(OperationNotSupportedError)))
	assert.False(t, errors.Is(NewUnsupportedDeleteOperationErr(e), assert.AnError))
}

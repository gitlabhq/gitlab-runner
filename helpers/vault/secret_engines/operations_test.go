//go:build !integration

package secret_engines

import (
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

	assert.ErrorIs(t, NewUnsupportedGetOperationErr(e), NewUnsupportedGetOperationErr(e))
	assert.NotErrorIs(t, NewUnsupportedGetOperationErr(e), new(OperationNotSupportedError))
	assert.NotErrorIs(t, NewUnsupportedGetOperationErr(e), assert.AnError)

	assert.ErrorIs(t, NewUnsupportedPutOperationErr(e), NewUnsupportedPutOperationErr(e))
	assert.NotErrorIs(t, NewUnsupportedPutOperationErr(e), new(OperationNotSupportedError))
	assert.NotErrorIs(t, NewUnsupportedPutOperationErr(e), assert.AnError)

	assert.ErrorIs(t, NewUnsupportedDeleteOperationErr(e), NewUnsupportedDeleteOperationErr(e))
	assert.NotErrorIs(t, NewUnsupportedDeleteOperationErr(e), new(OperationNotSupportedError))
	assert.NotErrorIs(t, NewUnsupportedDeleteOperationErr(e), assert.AnError)
}

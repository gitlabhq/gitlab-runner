//go:build !integration

package vault

import (
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
)

func TestUnwrapAPIResponseError(t *testing.T) {
	tests := map[string]struct {
		err           error
		expectedError error
	}{
		"nil error": {
			err:           nil,
			expectedError: nil,
		},
		"non-API error": {
			err:           assert.AnError,
			expectedError: assert.AnError,
		},
		"API error": {
			err:           &api.ResponseError{StatusCode: -1, Errors: []string{"test1", "test2"}},
			expectedError: new(unwrappedAPIResponseError),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			err := unwrapAPIResponseError(tt.err)
			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestUnwrappedAPIResponseError_Error(t *testing.T) {
	err := newUnwrappedAPIResponseError(-1, []string{"test1", "test2"})
	assert.Equal(t, "api error: status code -1: test1, test2", err.Error())
}

func TestUnwrappedAPIResponseError_Is(t *testing.T) {
	assert.ErrorIs(
		t,
		newUnwrappedAPIResponseError(-1, []string{"test1", "test2"}),
		newUnwrappedAPIResponseError(-1, []string{"test1", "test2"}),
	)
	assert.NotErrorIs(
		t,
		newUnwrappedAPIResponseError(-1, []string{"test1", "test2"}), new(unwrappedAPIResponseError),
	)
	assert.NotErrorIs(t, newUnwrappedAPIResponseError(-1, []string{"test1", "test2"}), assert.AnError)
}

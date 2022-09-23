//go:build !integration

package vault

import (
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
)

func TestSecretResult_Data(t *testing.T) {
	expectedData := map[string]interface{}{
		"test": "test",
	}

	tests := map[string]struct {
		secret       *api.Secret
		expectedData map[string]interface{}
	}{
		"nil api.Secret": {
			secret:       nil,
			expectedData: nil,
		},
		"non-nil api.Secret": {
			secret:       &api.Secret{Data: expectedData},
			expectedData: expectedData,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			r := newResult(tt.secret)
			data := r.Data()
			assert.Equal(t, tt.expectedData, data)
		})
	}
}

func TestSecretResult_TokenID(t *testing.T) {
	tests := map[string]struct {
		secret        *api.Secret
		expectedToken string
		expectedError error
	}{
		"nil api.Secret": {
			secret:        nil,
			expectedError: ErrNoResult,
		},
		"non-nil api.Secret": {
			secret: &api.Secret{Data: map[string]interface{}{
				"id": "token",
			}},
			expectedToken: "token",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			r := newResult(tt.secret)

			token, err := r.TokenID()
			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}
			assert.Equal(t, tt.expectedToken, token)
		})
	}
}

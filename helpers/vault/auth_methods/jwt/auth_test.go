//go:build !integration

package jwt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/auth_methods"
)

func TestNewMethod(t *testing.T) {
	tests := map[string]struct {
		providedData  map[string]interface{}
		expectedData  map[string]interface{}
		expectedError error
	}{
		"missing required key": {
			providedData: map[string]interface{}{
				roleKey: "role",
			},
			expectedError: new(auth_methods.MissingRequiredConfigurationKeyError),
		},
		"unexpected key provided": {
			providedData: map[string]interface{}{
				jwtKey:        "jwt",
				"unknown-key": "role",
			},
			expectedData: map[string]interface{}{
				jwtKey: "jwt",
			},
		},
		"proper configuration": {
			providedData: map[string]interface{}{
				jwtKey:  "jwt",
				roleKey: "role",
			},
			expectedData: map[string]interface{}{
				jwtKey:  "jwt",
				roleKey: "role",
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			a, err := NewMethod("", tt.providedData)

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}

			jwtAuth, ok := a.(*method)
			require.True(t, ok)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedData, jwtAuth.data)
		})
	}
}

func TestJWTAuth_Name(t *testing.T) {
	a := new(method)
	assert.Equal(t, methodName, a.Name())
}

func TestJWTAuth_Authenticate_Token(t *testing.T) {
	authPath := "some/path/to/jwt"
	expectedPath := "auth/some/path/to/jwt/login"

	jwt := "some.jwt.token"
	testRole := "test_role"
	expectedPayload := map[string]interface{}{
		"jwt":  jwt,
		"role": testRole,
	}

	vaultToken := "some.vault.token"

	tests := map[string]struct {
		setupClientMock func(*testing.T, *vault.MockClient) func()
		expectedError   error
		expectedToken   string
	}{
		"client write failure": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) func() {
				c.On("Write", expectedPath, expectedPayload).
					Return(nil, assert.AnError).
					Once()

				return func() {
					c.AssertExpectations(t)
				}
			},
			expectedError: assert.AnError,
		},
		"client write succeeded but token failure": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) func() {
				result := new(vault.MockResult)
				result.On("TokenID").
					Return("", assert.AnError).
					Once()

				c.On("Write", expectedPath, expectedPayload).
					Return(result, nil).
					Once()

				return func() {
					c.AssertExpectations(t)
					result.AssertExpectations(t)
				}
			},
			expectedError: assert.AnError,
		},
		"authentication succeeded": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) func() {
				result := new(vault.MockResult)
				result.On("TokenID").
					Return(vaultToken, nil).
					Once()

				c.On("Write", expectedPath, expectedPayload).
					Return(result, nil).
					Once()

				return func() {
					c.AssertExpectations(t)
					result.AssertExpectations(t)
				}
			},
			expectedToken: vaultToken,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := new(vault.MockClient)

			assertions := tt.setupClientMock(t, clientMock)
			defer assertions()

			data := map[string]interface{}{
				jwtKey:  jwt,
				roleKey: testRole,
			}

			auth, err := NewMethod(authPath, data)
			require.NoError(t, err)

			err = auth.Authenticate(clientMock)
			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedToken, auth.Token())
		})
	}
}

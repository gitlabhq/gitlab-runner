//go:build !integration

package vault

import (
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	serverURL := "https://vault.example.com"
	namespace := "test_namespace"

	assertAPIClient :=
		func(healthResponse *api.HealthResponse, healthCheckError error) func(t *testing.T, c *mockApiClient) func() {
			return func(t *testing.T, c *mockApiClient) func() {
				sysMock := new(mockApiClientSys)
				sysMock.On("Health").Return(healthResponse, healthCheckError).Once()

				c.On("Sys").Return(sysMock).Once()

				if healthResponse != nil && healthResponse.Initialized {
					c.On("SetNamespace", namespace).Return(nil).Once()
				}

				return func() {
					sysMock.AssertExpectations(t)
				}
			}
		}

	tests := map[string]struct {
		clientCreationError error
		assertAPIClient     func(t *testing.T, c *mockApiClient) func()
		expectedError       error
	}{
		"error on client creation": {
			clientCreationError: assert.AnError,
			assertAPIClient:     func(t *testing.T, c *mockApiClient) func() { return func() {} },
			expectedError:       assert.AnError,
		},
		"vault health check error": {
			assertAPIClient: assertAPIClient(nil, assert.AnError),
			expectedError:   assert.AnError,
		},
		"vault server not initialized": {
			assertAPIClient: assertAPIClient(&api.HealthResponse{Initialized: false}, nil),
			expectedError:   ErrVaultServerNotReady,
		},
		"vault server sealed": {
			assertAPIClient: assertAPIClient(&api.HealthResponse{Sealed: true}, nil),
			expectedError:   ErrVaultServerNotReady,
		},
		"vault client initialized": {
			assertAPIClient: assertAPIClient(&api.HealthResponse{Initialized: true, Sealed: false}, nil),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			apiClientMock := new(mockApiClient)
			defer apiClientMock.AssertExpectations(t)

			oldNewAPIClient := newAPIClient
			defer func() {
				newAPIClient = oldNewAPIClient
			}()
			newAPIClient = func(config *api.Config) (apiClient, error) {
				assert.Equal(t, serverURL, config.Address)

				return apiClientMock, tt.clientCreationError
			}

			defer tt.assertAPIClient(t, apiClientMock)()

			c, err := NewClient(serverURL, namespace)

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, c)

			cc, ok := c.(*defaultClient)
			assert.True(t, ok)
			assert.Equal(t, apiClientMock, cc.internal)
		})
	}
}

func TestDefaultClient_Authenticate(t *testing.T) {
	tests := map[string]struct {
		assertAuthMethodMock func(a *MockAuthMethod, c *defaultClient, ac *mockApiClient)
		expectedError        error
	}{
		"authentication error": {
			assertAuthMethodMock: func(a *MockAuthMethod, c *defaultClient, ac *mockApiClient) {
				a.On("Authenticate", c).Return(assert.AnError).Once()
			},
			expectedError: assert.AnError,
		},
		"authentication succeeded": {
			assertAuthMethodMock: func(a *MockAuthMethod, c *defaultClient, ac *mockApiClient) {
				token := "token"

				a.On("Authenticate", c).Return(nil).Once()
				a.On("Token").Return(token).Once()

				ac.On("SetToken", token).Once()
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			apiClientMock := new(mockApiClient)
			defer apiClientMock.AssertExpectations(t)

			c := &defaultClient{
				internal: apiClientMock,
			}

			authMethodMock := new(MockAuthMethod)
			defer authMethodMock.AssertExpectations(t)

			tt.assertAuthMethodMock(authMethodMock, c, apiClientMock)

			err := c.Authenticate(authMethodMock)
			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestDefaultClient_Write(t *testing.T) {
	secretData := map[string]interface{}{"key1": "value1"}
	secret := &api.Secret{Data: secretData}

	data := map[string]interface{}{"key": "value"}

	tests := map[string]struct {
		result        *api.Secret
		writeError    error
		expectedError error
	}{
		"error on writing": {
			writeError:    assert.AnError,
			expectedError: assert.AnError,
		},
		"api response type error on writing": {
			writeError:    new(api.ResponseError),
			expectedError: new(unwrappedAPIResponseError),
		},
		"successful writing": {
			result: secret,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			apiClientMock := new(mockApiClient)
			defer apiClientMock.AssertExpectations(t)

			apiClientLogicalMock := new(mockApiClientLogical)
			defer apiClientLogicalMock.AssertExpectations(t)

			apiClientMock.On("Logical").Return(apiClientLogicalMock).Once()

			apiClientLogicalMock.On("Write", "path/to/write", data).
				Return(tt.result, tt.writeError).
				Once()

			c := &defaultClient{
				internal: apiClientMock,
			}

			res, err := c.Write("path/to/write", data)

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, res)
			assert.Equal(t, secretData, res.Data())
		})
	}
}

func TestDefaultClient_Read(t *testing.T) {
	secretData := map[string]interface{}{"key1": "value1"}
	secret := &api.Secret{Data: secretData}

	tests := map[string]struct {
		result        *api.Secret
		writeError    error
		expectedError error
	}{
		"error on reading": {
			writeError:    assert.AnError,
			expectedError: assert.AnError,
		},
		"api response type error on reading": {
			writeError:    new(api.ResponseError),
			expectedError: new(unwrappedAPIResponseError),
		},
		"successful reading": {
			result: secret,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			apiClientMock := new(mockApiClient)
			defer apiClientMock.AssertExpectations(t)

			apiClientLogicalMock := new(mockApiClientLogical)
			defer apiClientLogicalMock.AssertExpectations(t)

			apiClientMock.On("Logical").Return(apiClientLogicalMock).Once()

			apiClientLogicalMock.On("Read", "path/to/read").
				Return(tt.result, tt.writeError).
				Once()

			c := &defaultClient{
				internal: apiClientMock,
			}

			res, err := c.Read("path/to/read")

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, res)
			assert.Equal(t, secretData, res.Data())
		})
	}
}

func TestDefaultClient_Delete(t *testing.T) {
	secretData := map[string]interface{}{"key1": "value1"}
	secret := &api.Secret{Data: secretData}

	tests := map[string]struct {
		result        *api.Secret
		writeError    error
		expectedError error
	}{
		"error on reading": {
			writeError:    assert.AnError,
			expectedError: assert.AnError,
		},
		"api response type error on reading": {
			writeError:    new(api.ResponseError),
			expectedError: new(unwrappedAPIResponseError),
		},
		"successful reading": {
			result: secret,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			apiClientMock := new(mockApiClient)
			defer apiClientMock.AssertExpectations(t)

			apiClientLogicalMock := new(mockApiClientLogical)
			defer apiClientLogicalMock.AssertExpectations(t)

			apiClientMock.On("Logical").Return(apiClientLogicalMock).Once()

			apiClientLogicalMock.On("Delete", "path/to/delete").
				Return(tt.result, tt.writeError).
				Once()

			c := &defaultClient{
				internal: apiClientMock,
			}

			err := c.Delete("path/to/delete")

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

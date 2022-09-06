//go:build !integration

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/auth_methods"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/internal/registry"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/secret_engines"
)

func TestNewVault(t *testing.T) {
	testURL := "https://vault.example.com/"
	testNamespace := "test_namespace"

	authPath := "path"
	authData := auth_methods.Data{"key": "value"}
	assertAuthMock := func(authMethodFactoryName string, a *MockAuth) {
		a.On("AuthName").Return(authMethodFactoryName).Once()
		a.On("AuthPath").Return(authPath).Once()
		a.On("AuthData").Return(authData).Once()
	}

	tests := map[string]struct {
		vaultClientCreationError      error
		assertAuthMock                func(authMethodFactoryName string, a *MockAuth)
		authMethodInitializationError error
		assertClientMock              func(c *vault.MockClient, _ vault.AuthMethod)
		expectedError                 error
	}{
		"error on vault client creation": {
			vaultClientCreationError: assert.AnError,
			assertAuthMock:           func(_ string, _ *MockAuth) {},
			assertClientMock:         func(_ *vault.MockClient, _ vault.AuthMethod) {},
			expectedError:            assert.AnError,
		},
		"unknown auth method factory": {
			assertAuthMock: func(_ string, a *MockAuth) {
				a.On("AuthName").Return("unknown factory").Once()
			},
			assertClientMock: func(_ *vault.MockClient, _ vault.AuthMethod) {},
			expectedError:    new(registry.FactoryNotRegisteredError),
		},
		"auth method initialization error": {
			assertAuthMock:                assertAuthMock,
			authMethodInitializationError: assert.AnError,
			assertClientMock:              func(_ *vault.MockClient, _ vault.AuthMethod) {},
			expectedError:                 assert.AnError,
		},
		"client authentication error": {
			assertAuthMock: assertAuthMock,
			assertClientMock: func(c *vault.MockClient, auth vault.AuthMethod) {
				c.On("Authenticate", auth).Return(assert.AnError).Once()
			},
			expectedError: assert.AnError,
		},
		"client initialized properly": {
			assertAuthMock: assertAuthMock,
			assertClientMock: func(c *vault.MockClient, auth vault.AuthMethod) {
				c.On("Authenticate", auth).Return(nil).Once()
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			authMethodMock := new(vault.MockAuthMethod)
			defer authMethodMock.AssertExpectations(t)

			authMethodFactory := func(path string, data auth_methods.Data) (vault.AuthMethod, error) {
				assert.Equal(t, authPath, path)
				assert.Equal(t, authData, data)

				return authMethodMock, tt.authMethodInitializationError
			}
			require.NotPanics(t, func() {
				auth_methods.MustRegisterFactory(t.Name(), authMethodFactory)
			})

			clientMock := new(vault.MockClient)
			defer clientMock.AssertExpectations(t)

			oldNewVaultClient := newVaultClient
			defer func() {
				newVaultClient = oldNewVaultClient
			}()
			newVaultClient = func(URL string, ns string) (vault.Client, error) {
				assert.Equal(t, testURL, URL)
				assert.Equal(t, testNamespace, ns)

				return clientMock, tt.vaultClientCreationError
			}

			authMock := new(MockAuth)
			defer authMock.AssertExpectations(t)

			tt.assertAuthMock(t.Name(), authMock)
			tt.assertClientMock(clientMock, authMethodMock)

			service, err := NewVault(testURL, testNamespace, authMock)

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, service)

			vaultService, ok := service.(*defaultVault)
			require.True(t, ok)

			assert.Equal(t, clientMock, vaultService.client)
		})
	}
}

func TestDefaultVault_GetField(t *testing.T) {
	enginePath := "path"
	assertEngineMock := func(engineFactoryName string, e *MockEngine) {
		e.On("EngineName").Return(engineFactoryName).Once()
		e.On("EnginePath").Return(enginePath).Once()
	}

	secretPath := "path"
	secretField := "field_1"
	secretValue := 1
	secretData := map[string]interface{}{
		secretField: secretValue,
		"field_2":   "test",
	}

	tests := map[string]struct {
		assertEngineMock       func(engineFactoryName string, e *MockEngine)
		assertSecretMock       func(s *MockSecret)
		assertSecretEngineMock func(e *vault.MockSecretEngine)
		expectedError          error
		expectedResult         interface{}
	}{
		"unknown engine factory": {
			assertEngineMock: func(_ string, e *MockEngine) {
				e.On("EngineName").Return("unknown factory").Once()
			},
			assertSecretMock:       func(_ *MockSecret) {},
			assertSecretEngineMock: func(_ *vault.MockSecretEngine) {},
			expectedError:          new(registry.FactoryNotRegisteredError),
		},
		"error on requesting data": {
			assertEngineMock: assertEngineMock,
			assertSecretMock: func(s *MockSecret) {
				s.On("SecretPath").Return(secretPath).Once()
			},
			assertSecretEngineMock: func(e *vault.MockSecretEngine) {
				e.On("Get", secretPath).Return(nil, assert.AnError).Once()
			},
			expectedError: assert.AnError,
		},
		"data requested properly with missing field": {
			assertEngineMock: assertEngineMock,
			assertSecretMock: func(s *MockSecret) {
				s.On("SecretPath").Return(secretPath).Once()
				s.On("SecretField").Return("unknown_field").Once()
			},
			assertSecretEngineMock: func(e *vault.MockSecretEngine) {
				e.On("Get", secretPath).Return(secretData, nil).Once()
			},
			expectedResult: nil,
		},
		"data requested properly with found field": {
			assertEngineMock: assertEngineMock,
			assertSecretMock: func(s *MockSecret) {
				s.On("SecretPath").Return(secretPath).Once()
				s.On("SecretField").Return(secretField).Once()
			},
			assertSecretEngineMock: func(e *vault.MockSecretEngine) {
				e.On("Get", secretPath).Return(secretData, nil).Once()
			},
			expectedResult: secretValue,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := new(vault.MockClient)
			defer clientMock.AssertExpectations(t)

			secretEngineMock := new(vault.MockSecretEngine)
			defer secretEngineMock.AssertExpectations(t)

			tt.assertSecretEngineMock(secretEngineMock)

			secretEngineFactory := func(c vault.Client, path string) vault.SecretEngine {
				assert.Equal(t, clientMock, c)
				assert.Equal(t, enginePath, path)

				return secretEngineMock
			}
			require.NotPanics(t, func() {
				secret_engines.MustRegisterFactory(t.Name(), secretEngineFactory)
			})

			engineMock := new(MockEngine)
			defer engineMock.AssertExpectations(t)

			tt.assertEngineMock(t.Name(), engineMock)

			secretMock := new(MockSecret)
			defer secretMock.AssertExpectations(t)

			tt.assertSecretMock(secretMock)

			service := &defaultVault{
				client: clientMock,
			}

			data, err := service.GetField(engineMock, secretMock)

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, data)
		})
	}
}

func TestDefaultVault_Put(t *testing.T) {
	enginePath := "path"
	assertEngineMock := func(engineFactoryName string, e *MockEngine) {
		e.On("EngineName").Return(engineFactoryName).Once()
		e.On("EnginePath").Return(enginePath).Once()
	}

	secretPath := "path"
	secretData := map[string]interface{}{
		"field_1": 1,
		"field_2": "test",
	}

	tests := map[string]struct {
		assertEngineMock       func(engineFactoryName string, e *MockEngine)
		assertSecretMock       func(s *MockSecret)
		assertSecretEngineMock func(e *vault.MockSecretEngine)
		expectedError          error
	}{
		"unknown engine factory": {
			assertEngineMock: func(_ string, e *MockEngine) {
				e.On("EngineName").Return("unknown factory").Once()
			},
			assertSecretMock:       func(_ *MockSecret) {},
			assertSecretEngineMock: func(_ *vault.MockSecretEngine) {},
			expectedError:          new(registry.FactoryNotRegisteredError),
		},
		"error on saving data": {
			assertEngineMock: assertEngineMock,
			assertSecretMock: func(s *MockSecret) {
				s.On("SecretPath").Return(secretPath).Once()
			},
			assertSecretEngineMock: func(e *vault.MockSecretEngine) {
				e.On("Put", secretPath, secretData).Return(assert.AnError).Once()
			},
			expectedError: assert.AnError,
		},
		"data saved properly": {
			assertEngineMock: assertEngineMock,
			assertSecretMock: func(s *MockSecret) {
				s.On("SecretPath").Return(secretPath).Once()
			},
			assertSecretEngineMock: func(e *vault.MockSecretEngine) {
				e.On("Put", secretPath, secretData).Return(nil).Once()
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := new(vault.MockClient)
			defer clientMock.AssertExpectations(t)

			secretEngineMock := new(vault.MockSecretEngine)
			defer secretEngineMock.AssertExpectations(t)

			tt.assertSecretEngineMock(secretEngineMock)

			secretEngineFactory := func(c vault.Client, path string) vault.SecretEngine {
				assert.Equal(t, clientMock, c)
				assert.Equal(t, enginePath, path)

				return secretEngineMock
			}
			require.NotPanics(t, func() {
				secret_engines.MustRegisterFactory(t.Name(), secretEngineFactory)
			})

			engineMock := new(MockEngine)
			defer engineMock.AssertExpectations(t)

			tt.assertEngineMock(t.Name(), engineMock)

			secretMock := new(MockSecret)
			defer secretMock.AssertExpectations(t)

			tt.assertSecretMock(secretMock)

			service := &defaultVault{
				client: clientMock,
			}

			err := service.Put(engineMock, secretMock, secretData)

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestDefaultVault_Delete(t *testing.T) {
	enginePath := "path"
	assertEngineMock := func(engineFactoryName string, e *MockEngine) {
		e.On("EngineName").Return(engineFactoryName).Once()
		e.On("EnginePath").Return(enginePath).Once()
	}

	secretPath := "path"

	tests := map[string]struct {
		assertEngineMock       func(engineFactoryName string, e *MockEngine)
		assertSecretMock       func(s *MockSecret)
		assertSecretEngineMock func(e *vault.MockSecretEngine)
		expectedError          error
	}{
		"unknown engine factory": {
			assertEngineMock: func(_ string, e *MockEngine) {
				e.On("EngineName").Return("unknown factory").Once()
			},
			assertSecretMock:       func(_ *MockSecret) {},
			assertSecretEngineMock: func(_ *vault.MockSecretEngine) {},
			expectedError:          new(registry.FactoryNotRegisteredError),
		},
		"error on deleting data": {
			assertEngineMock: assertEngineMock,
			assertSecretMock: func(s *MockSecret) {
				s.On("SecretPath").Return(secretPath).Once()
			},
			assertSecretEngineMock: func(e *vault.MockSecretEngine) {
				e.On("Delete", secretPath).Return(assert.AnError).Once()
			},
			expectedError: assert.AnError,
		},
		"data deleted properly": {
			assertEngineMock: assertEngineMock,
			assertSecretMock: func(s *MockSecret) {
				s.On("SecretPath").Return(secretPath).Once()
			},
			assertSecretEngineMock: func(e *vault.MockSecretEngine) {
				e.On("Delete", secretPath).Return(nil).Once()
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := new(vault.MockClient)
			defer clientMock.AssertExpectations(t)

			secretEngineMock := new(vault.MockSecretEngine)
			defer secretEngineMock.AssertExpectations(t)

			tt.assertSecretEngineMock(secretEngineMock)

			secretEngineFactory := func(c vault.Client, path string) vault.SecretEngine {
				assert.Equal(t, clientMock, c)
				assert.Equal(t, enginePath, path)

				return secretEngineMock
			}
			require.NotPanics(t, func() {
				secret_engines.MustRegisterFactory(t.Name(), secretEngineFactory)
			})

			engineMock := new(MockEngine)
			defer engineMock.AssertExpectations(t)

			tt.assertEngineMock(t.Name(), engineMock)

			secretMock := new(MockSecret)
			defer secretMock.AssertExpectations(t)

			tt.assertSecretMock(secretMock)

			service := &defaultVault{
				client: clientMock,
			}

			err := service.Delete(engineMock, secretMock)

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

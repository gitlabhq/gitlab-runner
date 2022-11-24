//go:build !integration

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDefaultResolver_Resolve(t *testing.T) {
	variableKey := "TEST_VARIABLE"
	returnValue := "test"
	secrets := Secrets{
		variableKey: Secret{
			Vault: &VaultSecret{
				Server: VaultServer{
					URL: "url",
					Auth: VaultAuth{
						Name: "name",
						Path: "path",
						Data: VaultAuthData{"data": "data"},
					},
				},
				Engine: VaultEngine{
					Name: "name",
					Path: "path",
				},
				Path:  "path",
				Field: "field",
			},
		},
	}

	composeSecrets := func(file bool) Secrets {
		secret := secrets[variableKey]
		secret.File = &file

		return Secrets{variableKey: secret}
	}

	getLogger := func(t *testing.T) (logger, func()) {
		logger := new(mockLogger)
		logger.On("Println", mock.Anything).Maybe()

		return logger, func() { logger.AssertExpectations(t) }
	}

	tests := map[string]struct {
		getLogger                     func(t *testing.T) (logger, func())
		supportedResolverPresent      bool
		secrets                       Secrets
		resolvedVariable              *JobVariable
		errorOnSecretResolving        error
		expectedResolverCreationError error
		expectedVariables             JobVariables
		expectedError                 error
	}{
		"resolver creation error": {
			getLogger: func(t *testing.T) (logger, func()) {
				return nil, func() {}
			},
			expectedResolverCreationError: ErrMissingLogger,
		},
		"no secrets to resolve": {
			getLogger:                getLogger,
			supportedResolverPresent: true,
			secrets:                  nil,
			expectedVariables:        nil,
			expectedError:            nil,
		},
		"error on secret resolving": {
			getLogger:                getLogger,
			supportedResolverPresent: true,
			secrets:                  secrets,
			errorOnSecretResolving:   assert.AnError,
			expectedVariables:        nil,
			expectedError:            assert.AnError,
		},
		"secret resolved properly - file not defined": {
			getLogger:                getLogger,
			supportedResolverPresent: true,
			secrets:                  secrets,
			expectedVariables: JobVariables{
				{
					Key:    variableKey,
					Value:  returnValue,
					File:   true,
					Masked: true,
					Raw:    true,
				},
			},
			expectedError: nil,
		},
		"secret resolved properly - file set to true": {
			getLogger:                getLogger,
			supportedResolverPresent: true,
			secrets:                  composeSecrets(true),
			expectedVariables: JobVariables{
				{
					Key:    variableKey,
					Value:  returnValue,
					File:   true,
					Masked: true,
					Raw:    true,
				},
			},
			expectedError: nil,
		},
		"secret resolved properly - file set to false": {
			getLogger:                getLogger,
			supportedResolverPresent: true,
			secrets:                  composeSecrets(false),
			expectedVariables: JobVariables{
				{
					Key:    variableKey,
					Value:  returnValue,
					File:   false,
					Masked: true,
					Raw:    true,
				},
			},
			expectedError: nil,
		},
		"no supported resolvers present": {
			getLogger: func(t *testing.T) (logger, func()) {
				logger := new(mockLogger)
				logger.On("Println", mock.Anything).Maybe()
				logger.On("Warningln", mock.Anything).Maybe()

				return logger, func() { logger.AssertExpectations(t) }
			},
			supportedResolverPresent: false,
			secrets:                  secrets,
			expectedVariables:        JobVariables{},
			expectedError:            nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			unsupportedResolver := new(MockSecretResolver)
			defer unsupportedResolver.AssertExpectations(t)
			supportedResolver := new(MockSecretResolver)
			defer supportedResolver.AssertExpectations(t)

			if tt.secrets != nil {
				unsupportedResolver.On("IsSupported").
					Return(false).
					Once()

				supportedResolver.On("IsSupported").
					Return(tt.supportedResolverPresent).
					Once()
				supportedResolver.On("Name").
					Return("supported_resolver").
					Maybe()
				if tt.supportedResolverPresent {
					supportedResolver.On("Resolve").
						Return(returnValue, tt.errorOnSecretResolving).
						Once()
				}
			}

			registry := new(defaultSecretResolverRegistry)
			registry.Register(func(secret Secret) SecretResolver { return unsupportedResolver })
			registry.Register(func(secret Secret) SecretResolver { return supportedResolver })

			logger, loggerCleanup := tt.getLogger(t)
			defer loggerCleanup()

			r, err := newSecretsResolver(logger, registry)
			if tt.expectedResolverCreationError != nil {
				assert.ErrorAs(t, err, &tt.expectedResolverCreationError)
				return
			}
			require.NoError(t, err)

			variables, err := r.Resolve(tt.secrets)

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedVariables, variables)
		})
	}
}

//go:build !integration

package azure_key_vault

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/azure_key_vault/service"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
)

func TestResolver_Name(t *testing.T) {
	r := newResolver(spec.Secret{})
	assert.Equal(t, resolverName, r.Name())
}

func TestResolver_IsSupported(t *testing.T) {
	tests := map[string]struct {
		secret        spec.Secret
		expectedVault bool
	}{
		"supported secret": {
			secret: spec.Secret{
				AzureKeyVault: &spec.AzureKeyVaultSecret{},
			},
			expectedVault: true,
		},
		"unsupported secret": {
			secret:        spec.Secret{},
			expectedVault: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			r := newResolver(tt.secret)
			assert.Equal(t, tt.expectedVault, r.IsSupported())
		})
	}
}

func TestResolver_Resolve(t *testing.T) {
	secret := spec.Secret{
		AzureKeyVault: &spec.AzureKeyVaultSecret{
			Name:    "test",
			Version: "version",
			Server: spec.AzureKeyVaultServer{
				ClientID: "test_url",
				TenantID: "test_namespace",
				JWT:      "jwt",
				URL:      "azure.gitlab.test",
			},
		},
	}

	tests := map[string]struct {
		secret                    spec.Secret
		vaultServiceCreationError error
		assertVaultServiceMock    func(s *service.MockAzureKeyVault)
		expectedValue             string
		expectedError             error
	}{
		"error on support detection": {
			expectedError: new(secrets.ResolvingUnsupportedSecretError),
		},
		"error on vault service creation": {
			secret:                    secret,
			vaultServiceCreationError: assert.AnError,
			expectedError:             assert.AnError,
		},
		"error on field resolving": {
			secret: secret,
			assertVaultServiceMock: func(s *service.MockAzureKeyVault) {
				s.On("GetSecret", secret.AzureKeyVault.Name, secret.AzureKeyVault.Version).
					Return(nil, assert.AnError).
					Once()
			},
			expectedError: assert.AnError,
		},
		"field resolved properly": {
			secret: secret,
			assertVaultServiceMock: func(s *service.MockAzureKeyVault) {
				s.On("GetSecret", secret.AzureKeyVault.Name, secret.AzureKeyVault.Version).
					Return(struct{ Date string }{Date: "2020-08-24"}, nil).
					Once()
			},
			expectedValue: "{2020-08-24}",
			expectedError: nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			serviceMock := service.NewMockAzureKeyVault(t)
			if tt.assertVaultServiceMock != nil {
				tt.assertVaultServiceMock(serviceMock)
			}

			oldNewVaultService := newVaultService
			defer func() {
				newVaultService = oldNewVaultService
			}()
			newVaultService = func(server spec.AzureKeyVaultServer) (service.AzureKeyVault, error) {
				assert.Equal(t, tt.secret.AzureKeyVault.Server, server)
				return serviceMock, tt.vaultServiceCreationError
			}

			r := newResolver(tt.secret)

			value, err := r.Resolve()

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := map[string]struct {
		err                         error
		expectConfigurationError    bool
		expectExternalDependency    bool
		expectUnmodifiedPassThrough bool
	}{
		"nil error": {
			err: nil,
		},
		"permission denied (403) is a configuration error": {
			err:                      fmt.Errorf("getting secret failed: %w", &azcore.ResponseError{StatusCode: http.StatusForbidden}),
			expectConfigurationError: true,
		},
		"unknown secret (404) is a configuration error": {
			err:                      fmt.Errorf("getting secret failed: %w", &azcore.ResponseError{StatusCode: http.StatusNotFound}),
			expectConfigurationError: true,
		},
		"server error (500) is an external dependency error": {
			err:                      fmt.Errorf("getting secret failed: %w", &azcore.ResponseError{StatusCode: http.StatusInternalServerError}),
			expectExternalDependency: true,
		},
		"service unavailable (503) is an external dependency error": {
			err:                      fmt.Errorf("getting secret failed: %w", &azcore.ResponseError{StatusCode: http.StatusServiceUnavailable}),
			expectExternalDependency: true,
		},
		"unauthorized (401) keeps default classification": {
			err:                         fmt.Errorf("getting secret failed: %w", &azcore.ResponseError{StatusCode: http.StatusUnauthorized}),
			expectUnmodifiedPassThrough: true,
		},
		"rate limited (429) keeps default classification": {
			err:                         fmt.Errorf("getting secret failed: %w", &azcore.ResponseError{StatusCode: http.StatusTooManyRequests}),
			expectUnmodifiedPassThrough: true,
		},
		"error without a response error keeps default classification": {
			err:                         assert.AnError,
			expectUnmodifiedPassThrough: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			result := classifyError(tt.err)

			if tt.err == nil {
				assert.NoError(t, result)
				return
			}

			var configurationErr *secrets.ResolvingConfigurationError
			var externalDependencyErr *secrets.ResolvingExternalDependencyError

			assert.Equal(t, tt.expectConfigurationError, errors.As(result, &configurationErr), "configuration error classification")
			assert.Equal(t, tt.expectExternalDependency, errors.As(result, &externalDependencyErr), "external dependency classification")

			if tt.expectUnmodifiedPassThrough {
				assert.Equal(t, tt.err, result)
			} else {
				assert.ErrorIs(t, result, tt.err, "classified error must wrap the original")
			}
		})
	}
}

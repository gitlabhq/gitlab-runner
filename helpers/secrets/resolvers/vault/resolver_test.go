//go:build !integration

package vault

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/service"
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
				Vault: &spec.VaultSecret{},
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
		Vault: &spec.VaultSecret{
			Server: spec.VaultServer{
				URL:       "test_url",
				Namespace: "test_namespace",
			},
		},
	}

	tests := map[string]struct {
		secret                    spec.Secret
		vaultServiceCreationError error
		assertVaultServiceMock    func(s *service.MockVault)
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
			assertVaultServiceMock: func(s *service.MockVault) {
				s.On("GetField", secret.Vault, secret.Vault).
					Return(nil, assert.AnError).
					Once()
			},
			expectedError: assert.AnError,
		},
		"field resolved properly": {
			secret: secret,
			assertVaultServiceMock: func(s *service.MockVault) {
				s.On("GetField", secret.Vault, secret.Vault).
					Return(struct{ Date string }{Date: "2020-08-24"}, nil).
					Once()
			},
			expectedValue: "{2020-08-24}",
			expectedError: nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			serviceMock := service.NewMockVault(t)

			if tt.assertVaultServiceMock != nil {
				tt.assertVaultServiceMock(serviceMock)
			}

			oldNewVaultService := newVaultService
			defer func() {
				newVaultService = oldNewVaultService
			}()
			newVaultService = func(url string, namespace string, auth service.Auth) (service.Vault, error) {
				assert.Equal(t, tt.secret.Vault.Server.URL, url)
				assert.Equal(t, tt.secret.Vault, auth)
				assert.Equal(t, tt.secret.Vault.Server.Namespace, namespace)
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

type testStatusCodeError struct {
	statusCode int
}

func (e *testStatusCodeError) Error() string {
	return fmt.Sprintf("api error: status code %d", e.statusCode)
}

func (e *testStatusCodeError) StatusCode() int {
	return e.statusCode
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
			err:                      fmt.Errorf("reading secret: %w", &testStatusCodeError{statusCode: http.StatusForbidden}),
			expectConfigurationError: true,
		},
		"missing role (400) is a configuration error": {
			err:                      fmt.Errorf("authenticating Vault client: %w", &testStatusCodeError{statusCode: http.StatusBadRequest}),
			expectConfigurationError: true,
		},
		"unauthorized (401) is a configuration error": {
			err:                      fmt.Errorf("reading secret: %w", &testStatusCodeError{statusCode: http.StatusUnauthorized}),
			expectConfigurationError: true,
		},
		"unknown path (404) is a configuration error": {
			err:                      fmt.Errorf("reading secret: %w", &testStatusCodeError{statusCode: http.StatusNotFound}),
			expectConfigurationError: true,
		},
		"server error (500) is an external dependency error": {
			err:                      fmt.Errorf("reading secret: %w", &testStatusCodeError{statusCode: http.StatusInternalServerError}),
			expectExternalDependency: true,
		},
		"service unavailable (503) is an external dependency error": {
			err:                      fmt.Errorf("reading secret: %w", &testStatusCodeError{statusCode: http.StatusServiceUnavailable}),
			expectExternalDependency: true,
		},
		"rate limited (429) keeps default classification": {
			err:                         fmt.Errorf("reading secret: %w", &testStatusCodeError{statusCode: http.StatusTooManyRequests}),
			expectUnmodifiedPassThrough: true,
		},
		"error without status code keeps default classification": {
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

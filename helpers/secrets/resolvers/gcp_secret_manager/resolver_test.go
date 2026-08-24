//go:build !integration

package gcp_secret_manager

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
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
		"supported resolver": {
			secret: spec.Secret{
				GCPSecretManager: &spec.GCPSecretManagerSecret{},
			},
			expectedVault: true,
		},
		"unsupported resolver": {
			secret: spec.Secret{
				Vault: &spec.VaultSecret{},
			},
			expectedVault: false,
		},
		"no resolver": {
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
		GCPSecretManager: &spec.GCPSecretManagerSecret{
			Server: spec.GCPSecretManagerServer{
				WorkloadIdentityFederationPoolId:     "",
				WorkloadIdentityFederationProviderID: "",
				JWT:                                  "",
			},
		},
	}

	tests := map[string]struct {
		secret        spec.Secret
		setupMock     func(c *mockClient)
		expectedValue string
		expectedError error
	}{
		"error on support detection": {
			expectedError: new(secrets.ResolvingUnsupportedSecretError),
		},
		"error on accessing secret": {
			secret: secret,
			setupMock: func(c *mockClient) {
				c.On("GetSecret", mock.Anything, secret.GCPSecretManager).
					Return("", assert.AnError).
					Once()
			},
			expectedError: assert.AnError,
		},
		"secret resolved successfully": {
			secret: secret,
			setupMock: func(c *mockClient) {
				c.On("GetSecret", mock.Anything, secret.GCPSecretManager).
					Return("p@assword", nil).
					Once()
			},
			expectedValue: "p@assword",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := newMockClient(t)
			if tt.setupMock != nil {
				tt.setupMock(clientMock)
			}

			r := &resolver{
				secret: tt.secret,
				client: clientMock,
			}

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
		"permission denied is a configuration error": {
			err:                      fmt.Errorf("failed to get secret: %w", status.Error(codes.PermissionDenied, "permission denied")),
			expectConfigurationError: true,
		},
		"unknown secret is a configuration error": {
			err:                      fmt.Errorf("failed to get secret: %w", status.Error(codes.NotFound, "secret not found")),
			expectConfigurationError: true,
		},
		"unavailable is an external dependency error": {
			err:                      fmt.Errorf("failed to get secret: %w", status.Error(codes.Unavailable, "service unavailable")),
			expectExternalDependency: true,
		},
		"internal is an external dependency error": {
			err:                      fmt.Errorf("failed to get secret: %w", status.Error(codes.Internal, "internal error")),
			expectExternalDependency: true,
		},
		"unauthenticated keeps default classification": {
			err:                         fmt.Errorf("failed to exchange sts token: %w", status.Error(codes.Unauthenticated, "bad credentials")),
			expectUnmodifiedPassThrough: true,
		},
		"error without a grpc status keeps default classification": {
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

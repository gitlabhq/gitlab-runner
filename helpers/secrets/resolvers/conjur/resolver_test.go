//go:build !integration

package conjur

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/conjur/service"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
)

func TestResolver_Name(t *testing.T) {
	r := newResolver(spec.Secret{})
	assert.Equal(t, resolverName, r.Name())
}

func TestResolver_IsSupported(t *testing.T) {
	tests := map[string]struct {
		secret          spec.Secret
		expectSupported bool
	}{
		"supported resolver": {
			secret: spec.Secret{
				Conjur: &spec.ConjurSecret{},
			},
			expectSupported: true,
		},
		"unsupported resolver": {
			secret: spec.Secret{
				Vault: &spec.VaultSecret{},
			},
			expectSupported: false,
		},
		"no resolver": {
			secret:          spec.Secret{},
			expectSupported: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			r := newResolver(tt.secret)
			assert.Equal(t, tt.expectSupported, r.IsSupported())
		})
	}
}

func TestResolver_Resolve(t *testing.T) {
	fullSecret := spec.Secret{
		Conjur: &spec.ConjurSecret{
			Path: "app/prod/db-password",
			Server: spec.ConjurServer{
				ApplianceURL:      "https://conjur.example.com",
				Account:           "myAccount",
				AuthnJWTServiceID: "gitlab",
				JWT:               "the-jwt",
			},
		},
	}

	tests := map[string]struct {
		secret        spec.Secret
		setupMock     func(c *mockClient)
		expectedValue string
		expectedError error
		expectedErrIs error
	}{
		"unsupported secret": {
			secret:        spec.Secret{},
			expectedError: new(secrets.ResolvingUnsupportedSecretError),
		},
		"nil server, feature flag off on rails side": {
			secret: spec.Secret{
				Conjur: &spec.ConjurSecret{
					Path: "app/prod/db-password",
				},
			},
			expectedError: new(secrets.ResolvingConfigurationError),
		},
		"missing appliance_url": {
			secret: spec.Secret{
				Conjur: &spec.ConjurSecret{
					Path: "app/prod/db-password",
					Server: spec.ConjurServer{
						Account:           "myAccount",
						AuthnJWTServiceID: "gitlab",
						JWT:               "the-jwt",
					},
				},
			},
			expectedError: new(secrets.ResolvingConfigurationError),
		},
		"missing account": {
			secret: spec.Secret{
				Conjur: &spec.ConjurSecret{
					Path: "app/prod/db-password",
					Server: spec.ConjurServer{
						ApplianceURL:      "https://conjur.example.com",
						AuthnJWTServiceID: "gitlab",
						JWT:               "the-jwt",
					},
				},
			},
			expectedError: new(secrets.ResolvingConfigurationError),
		},
		"missing authn_jwt_service_id": {
			secret: spec.Secret{
				Conjur: &spec.ConjurSecret{
					Path: "app/prod/db-password",
					Server: spec.ConjurServer{
						ApplianceURL: "https://conjur.example.com",
						Account:      "myAccount",
						JWT:          "the-jwt",
					},
				},
			},
			expectedError: new(secrets.ResolvingConfigurationError),
		},
		"missing jwt": {
			secret: spec.Secret{
				Conjur: &spec.ConjurSecret{
					Path: "app/prod/db-password",
					Server: spec.ConjurServer{
						ApplianceURL:      "https://conjur.example.com",
						Account:           "myAccount",
						AuthnJWTServiceID: "gitlab",
					},
				},
			},
			expectedError: new(secrets.ResolvingConfigurationError),
		},
		"missing path": {
			secret: spec.Secret{
				Conjur: &spec.ConjurSecret{
					Server: spec.ConjurServer{
						ApplianceURL:      "https://conjur.example.com",
						Account:           "myAccount",
						AuthnJWTServiceID: "gitlab",
						JWT:               "the-jwt",
					},
				},
			},
			expectedError: new(secrets.ResolvingConfigurationError),
		},
		"error fetching secret": {
			secret: fullSecret,
			setupMock: func(c *mockClient) {
				c.EXPECT().GetSecret(mock.Anything, fullSecret.Conjur).
					Return("", assert.AnError).
					Once()
			},
			expectedError: assert.AnError,
		},
		"auth failure is classified as a configuration error": {
			secret: fullSecret,
			setupMock: func(c *mockClient) {
				c.EXPECT().GetSecret(mock.Anything, fullSecret.Conjur).
					Return("", &service.APIError{Status: 401}).
					Once()
			},
			expectedError: new(secrets.ResolvingConfigurationError),
		},
		"appliance failure is classified as an external dependency error": {
			secret: fullSecret,
			setupMock: func(c *mockClient) {
				c.EXPECT().GetSecret(mock.Anything, fullSecret.Conjur).
					Return("", &service.APIError{Status: 503}).
					Once()
			},
			expectedError: new(secrets.ResolvingExternalDependencyError),
		},
		"secret not found passes through unchanged": {
			secret: fullSecret,
			setupMock: func(c *mockClient) {
				c.EXPECT().GetSecret(mock.Anything, fullSecret.Conjur).
					Return("", common.ErrSecretNotFound).
					Once()
			},
			expectedErrIs: common.ErrSecretNotFound,
		},
		"secret resolved successfully": {
			secret: fullSecret,
			setupMock: func(c *mockClient) {
				c.EXPECT().GetSecret(mock.Anything, fullSecret.Conjur).
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

			switch {
			case tt.expectedError != nil:
				assert.IsType(t, tt.expectedError, err)
			case tt.expectedErrIs != nil:
				assert.ErrorIs(t, err, tt.expectedErrIs)
				assert.NotErrorAs(t, err, new(*secrets.ResolvingConfigurationError))
				assert.NotErrorAs(t, err, new(*secrets.ResolvingExternalDependencyError))
			default:
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, value)
			}
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
		"302 is a configuration error": {
			err:                      &service.APIError{Status: 302, Body: "found"},
			expectConfigurationError: true,
		},
		"400 is a configuration error": {
			err:                      &service.APIError{Status: 400, Body: "bad request"},
			expectConfigurationError: true,
		},
		"401 is a configuration error": {
			err:                      &service.APIError{Status: 401, Body: "unauthenticated"},
			expectConfigurationError: true,
		},
		"404 is a configuration error": {
			err:                      &service.APIError{Status: 404, Body: "not found"},
			expectConfigurationError: true,
		},
		"500 is an external dependency error": {
			err:                      &service.APIError{Status: 500, Body: "internal error"},
			expectExternalDependency: true,
		},
		"503 is an external dependency error": {
			err:                      &service.APIError{Status: 503, Body: "unavailable"},
			expectExternalDependency: true,
		},
		"error without a status code keeps default classification": {
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

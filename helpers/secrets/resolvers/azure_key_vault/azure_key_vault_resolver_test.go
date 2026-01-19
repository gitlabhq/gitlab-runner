//go:build !integration

package azure_key_vault

import (
	"testing"

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

//go:build !integration

package vault

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/service"
)

func TestResolver_Name(t *testing.T) {
	r := newResolver(common.Secret{})
	assert.Equal(t, resolverName, r.Name())
}

func TestResolver_IsSupported(t *testing.T) {
	tests := map[string]struct {
		secret        common.Secret
		expectedVault bool
	}{
		"supported secret": {
			secret: common.Secret{
				Vault: &common.VaultSecret{},
			},
			expectedVault: true,
		},
		"unsupported secret": {
			secret:        common.Secret{},
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
	secret := common.Secret{
		Vault: &common.VaultSecret{
			Server: common.VaultServer{
				URL:       "test_url",
				Namespace: "test_namespace",
			},
		},
	}

	tests := map[string]struct {
		secret                    common.Secret
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
			serviceMock := new(service.MockVault)
			defer serviceMock.AssertExpectations(t)

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

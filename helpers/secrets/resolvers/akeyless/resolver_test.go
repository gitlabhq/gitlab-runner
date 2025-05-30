//go:build !integration

package akeyless

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/akeyless/service"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
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
		"supported resolver": {
			secret: common.Secret{
				Akeyless: &common.AkeylessSecret{},
			},
			expectedVault: true,
		},
		"unsupported resolver": {
			secret: common.Secret{
				Vault: &common.VaultSecret{},
			},
			expectedVault: false,
		},
		"no resolver": {
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
		Akeyless: &common.AkeylessSecret{Server: common.AkeylessServer{}},
	}

	tests := map[string]struct {
		secret        common.Secret
		setupMock     func(c *service.MockAkeyless)
		expectedValue string
		expectedError error
	}{
		"error on support detection": {
			expectedError: new(secrets.ResolvingUnsupportedSecretError),
		},
		"error on accessing secret": {
			secret: secret,
			setupMock: func(c *service.MockAkeyless) {
				c.On("GetSecret", mock.Anything).
					Return("", assert.AnError).
					Once()
			},
			expectedError: assert.AnError,
		},
		"secret resolved successfully": {
			secret: secret,
			setupMock: func(c *service.MockAkeyless) {
				c.On("GetSecret", mock.Anything).
					Return("p@assword", nil).
					Once()
			},
			expectedValue: "p@assword",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			akeylessMock := service.NewMockAkeyless(t)
			if tt.setupMock != nil {
				tt.setupMock(akeylessMock)
			}

			r := &akeylessResolver{
				secret: tt.secret,
			}

			newAkeyless = func(secret *common.AkeylessSecret) service.Akeyless {
				return akeylessMock
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

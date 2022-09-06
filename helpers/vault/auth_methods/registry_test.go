//go:build !integration

package auth_methods

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/internal/registry"
)

func TestMustRegisterFactory(t *testing.T) {
	factory := func(path string, data Data) (vault.AuthMethod, error) {
		return new(vault.MockAuthMethod), nil
	}

	tests := map[string]struct {
		register      func()
		panicExpected bool
	}{
		"duplicate factory registration": {
			register: func() {
				MustRegisterFactory("test-auth", factory)
				MustRegisterFactory("test-auth", factory)
			},
			panicExpected: true,
		},
		"successful factory registration": {
			register: func() {
				MustRegisterFactory("test-auth", factory)
				MustRegisterFactory("test-auth-2", factory)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			oldFactoriesRegistry := factoriesRegistry
			defer func() {
				factoriesRegistry = oldFactoriesRegistry
			}()
			factoriesRegistry = registry.New("fake registry")

			if tt.panicExpected {
				assert.Panics(t, tt.register)
				return
			}
			assert.NotPanics(t, tt.register)
		})
	}
}

func TestGetFactory(t *testing.T) {
	oldFactoriesRegistry := factoriesRegistry
	defer func() {
		factoriesRegistry = oldFactoriesRegistry
	}()
	factoriesRegistry = registry.New("fake registry")

	require.NotPanics(t, func() {
		newMockedAuthMethodFactory := func(path string, data Data) (vault.AuthMethod, error) {
			return new(vault.MockAuthMethod), nil
		}

		MustRegisterFactory("test-auth", newMockedAuthMethodFactory)
	})

	tests := map[string]struct {
		engineName    string
		expectedError error
	}{
		"factory found": {
			engineName:    "not-existing-auth",
			expectedError: new(registry.FactoryNotRegisteredError),
		},
		"factory not found": {
			engineName: "test-auth",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			factory, err := GetFactory(tt.engineName)
			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				assert.Nil(t, factory)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, factory)
		})
	}
}

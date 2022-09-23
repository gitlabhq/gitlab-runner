//go:build !integration

package secret_engines

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/internal/registry"
)

func TestMustRegisterFactory(t *testing.T) {
	factory := func(client vault.Client, path string) vault.SecretEngine {
		return new(vault.MockSecretEngine)
	}

	tests := map[string]struct {
		register      func()
		panicExpected bool
	}{
		"duplicate factory registration": {
			register: func() {
				MustRegisterFactory("test-engine", factory)
				MustRegisterFactory("test-engine", factory)
			},
			panicExpected: true,
		},
		"successful factory registration": {
			register: func() {
				MustRegisterFactory("test-engine", factory)
				MustRegisterFactory("test-engine-2", factory)
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
		MustRegisterFactory("test-engine", func(client vault.Client, path string) vault.SecretEngine {
			return new(vault.MockSecretEngine)
		})
	})

	tests := map[string]struct {
		engineName    string
		expectedError error
	}{
		"factory found": {
			engineName:    "not-existing-engine",
			expectedError: new(registry.FactoryNotRegisteredError),
		},
		"factory not found": {
			engineName: "test-engine",
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

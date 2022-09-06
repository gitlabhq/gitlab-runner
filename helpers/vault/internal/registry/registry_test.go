//go:build !integration

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFactoryAlreadyRegisteredError_Error(t *testing.T) {
	assert.Equal(
		t,
		`factory for engine "test-engine" already registered`,
		NewFactoryAlreadyRegisteredError("engine", "test-engine").Error(),
	)
}

func TestFactoryAlreadyRegisteredError_Is(t *testing.T) {
	assert.ErrorIs(
		t,
		NewFactoryAlreadyRegisteredError("engine", "test-engine"),
		NewFactoryAlreadyRegisteredError("engine", "test-engine"),
	)
	assert.NotErrorIs(
		t,
		NewFactoryAlreadyRegisteredError("engine", "test-engine"),
		new(FactoryAlreadyRegisteredError),
	)
	assert.NotErrorIs(
		t,
		NewFactoryAlreadyRegisteredError("engine", "test-engine"), assert.AnError,
	)
}

func TestFactoryNotRegisteredError_Error(t *testing.T) {
	assert.Equal(
		t,
		`factory for engine "test-engine" is not registered`,
		NewFactoryNotRegisteredError("engine", "test-engine").Error(),
	)
}

func TestFactoryNotRegisteredError_Is(t *testing.T) {
	assert.ErrorIs(
		t,
		NewFactoryNotRegisteredError("engine", "test-engine"),
		NewFactoryNotRegisteredError("engine", "test-engine"),
	)
	assert.NotErrorIs(
		t,
		NewFactoryNotRegisteredError("engine", "test-engine"),
		new(FactoryNotRegisteredError),
	)
	assert.NotErrorIs(
		t,
		NewFactoryNotRegisteredError("engine", "test-engine"), assert.AnError,
	)
}

type fakeEntry struct{}

func TestFactoryRegistry_Register(t *testing.T) {
	factoryName := "test-entry-1"

	tests := map[string]struct {
		secondFactoryName string
		expectedError     error
	}{
		"duplicate factory registration": {
			secondFactoryName: factoryName,
			expectedError:     new(FactoryAlreadyRegisteredError),
		},
		"successful factory registration": {
			secondFactoryName: "test-entry-2",
			expectedError:     nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			registry := New("fake entries factory")

			err := registry.Register(factoryName, fakeEntry{})
			require.NoError(t, err)

			err = registry.Register(tt.secondFactoryName, fakeEntry{})

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestFactoryRegistry_Get(t *testing.T) {
	factoryName := "test-entry-1"
	entry := &fakeEntry{}

	registry := New("fake entries factory")

	err := registry.Register(factoryName, entry)
	require.NoError(t, err)

	tests := map[string]struct {
		factoryName   string
		expectedEntry *fakeEntry
		expectedError error
	}{
		"factory not found": {
			factoryName:   "test-entry-2",
			expectedError: new(FactoryNotRegisteredError),
		},
		"factory found": {
			factoryName:   factoryName,
			expectedEntry: entry,
			expectedError: nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			factory, err := registry.Get(tt.factoryName)
			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				assert.Nil(t, factory)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedEntry, factory)
		})
	}
}

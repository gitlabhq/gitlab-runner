//go:build !integration

package auth_methods

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMissingRequiredConfigurationKeyError_Error(t *testing.T) {
	assert.Equal(
		t,
		`missing required auth method configuration key "test-key"`,
		NewMissingRequiredConfigurationKeyError("test-key").Error(),
	)
}

func TestMissingRequiredConfigurationKeyError_Is(t *testing.T) {
	assert.ErrorIs(
		t,
		NewMissingRequiredConfigurationKeyError("test-key"),
		NewMissingRequiredConfigurationKeyError("test-key"),
	)
	assert.NotErrorIs(
		t,
		NewMissingRequiredConfigurationKeyError("test-key"), new(MissingRequiredConfigurationKeyError),
	)
	assert.NotErrorIs(
		t,
		NewMissingRequiredConfigurationKeyError("test-key"), assert.AnError,
	)
}

func TestData_Filter(t *testing.T) {
	requiredKeys := []string{"required1", "required2"}
	allowedKeys := []string{"required1", "required2", "allowed1", "allowed2"}

	tests := map[string]struct {
		data          Data
		expectedData  Data
		expectedError error
	}{
		"missing required field": {
			data: Data{
				"required2": "test",
				"allowed1":  "test",
				"allowed2":  "test",
			},
			expectedError: NewMissingRequiredConfigurationKeyError("required1"),
		},
		"missing allowed field": {
			data: Data{
				"required1": "test",
				"required2": "test",
				"allowed1":  "test",
			},
			expectedData: Data{
				"required1": "test",
				"required2": "test",
				"allowed1":  "test",
			},
		},
		"unexpected field used": {
			data: Data{
				"required1":   "test",
				"required2":   "test",
				"allowed1":    "test",
				"allowed2":    "test",
				"unexpected1": "test",
				"unexpected2": "test",
			},
			expectedData: Data{
				"required1": "test",
				"required2": "test",
				"allowed1":  "test",
				"allowed2":  "test",
			},
		},
		"only required and allowed fields": {
			data: Data{
				"required1": "test",
				"required2": "test",
				"allowed1":  "test",
				"allowed2":  "test",
			},
			expectedData: Data{
				"required1": "test",
				"required2": "test",
				"allowed1":  "test",
				"allowed2":  "test",
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			data, err := tt.data.Filter(requiredKeys, allowedKeys)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedData, data)
		})
	}
}

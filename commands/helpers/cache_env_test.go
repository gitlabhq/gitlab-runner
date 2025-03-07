//go:build !integration

package helpers

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadEnvFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test.env")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString("TEST_KEY1=TEST_VALUE1\nTEST_KEY2=TEST_VALUE2")
	assert.NoError(t, err)
	tmpfile.Close()

	tests := map[string]struct {
		envFile     string
		expectError bool
		setup       func()
		check       func(*testing.T)
	}{
		"empty env file": {
			envFile:     "",
			expectError: false,
		},
		"missing env file": {
			envFile:     "non_existent_file.env",
			expectError: true,
		},
		"successful env file load": {
			envFile:     tmpfile.Name(),
			expectError: false,
			check: func(t *testing.T) {
				assert.Equal(t, "TEST_VALUE1", os.Getenv("TEST_KEY1"))
				assert.Equal(t, "TEST_VALUE2", os.Getenv("TEST_KEY2"))
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}

			originalEnv := os.Environ()
			defer func() {
				os.Clearenv()
				for _, envVar := range originalEnv {
					parts := strings.SplitN(envVar, "=", 2)
					os.Setenv(parts[0], parts[1])
				}
			}()

			err := loadEnvFile(tc.envFile)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tc.check != nil {
				tc.check(t)
			}
		})
	}
}

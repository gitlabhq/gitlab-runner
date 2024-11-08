//go:build !integration

package homedir

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFix(t *testing.T) {
	const (
		testHomeDirVar      = "TEST_HOME"
		testUnsetHomeDirVar = "TEST_UNSET_HOME"
	)

	var (
		testDir  = t.TempDir()
		testDir2 = t.TempDir()
	)

	tests := map[string]struct {
		env                  string
		value                string
		assertError          func(t *testing.T, err error)
		expectedHomedirValue string
	}{
		"HOME variable is set": {
			env:                  testHomeDirVar,
			expectedHomedirValue: testDir,
		},
		"HOME variable is not set and homedir value is empty": {
			env: testUnsetHomeDirVar,
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrHomedirVariableNotSet)
			},
		},
		"HOME variable is not set and homedir.Get gives a result": {
			env:                  testUnsetHomeDirVar,
			value:                testDir2,
			expectedHomedirValue: testDir2,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			require.NoError(t, os.Setenv(testHomeDirVar, testDir))
			t.Cleanup(func() {
				require.NoError(t, os.Unsetenv(testHomeDirVar))
			})

			oldEnvGetter := envGetter
			oldHomedirGetter := homedirGetter
			t.Cleanup(func() {
				envGetter = oldEnvGetter
				homedirGetter = oldHomedirGetter
			})

			envGetter = func() string {
				return tc.env
			}

			homedirGetter = func() string {
				return tc.value
			}

			err := Fix()
			if tc.assertError != nil {
				tc.assertError(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedHomedirValue, os.Getenv(tc.env))
		})
	}
}

//go:build !integration

package steps_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/commands/steps"
)

func TestRecoverArgv(t *testing.T) {
	encode := func(t *testing.T, argv []string) string {
		t.Helper()
		out, err := steps.EncodeRecoveryArgv(argv)
		require.NoError(t, err)
		return out
	}

	tests := map[string]struct {
		initialArgs []string
		envValue    string
		envUnset    bool
		wantArgs    []string
		wantUnset   bool
	}{
		"no recovery when argv has subcommand": {
			initialArgs: []string{"helper", "artifacts-uploader"},
			envValue:    encode(t, []string{"steps", "serve", "/bin/sh"}),
			wantArgs:    []string{"helper", "artifacts-uploader"},
			wantUnset:   false,
		},
		"no recovery when env var unset": {
			initialArgs: []string{"helper"},
			envUnset:    true,
			wantArgs:    []string{"helper"},
			wantUnset:   true,
		},
		"recovery when argv mangled and env var present": {
			initialArgs: []string{"helper"},
			envValue:    encode(t, []string{"steps", "serve", "/bin/sh"}),
			wantArgs:    []string{"helper", "steps", "serve", "/bin/sh"},
			wantUnset:   true,
		},
		"recovery preserves arg order and content with spaces": {
			initialArgs: []string{"helper"},
			envValue:    encode(t, []string{"steps", "serve", "echo hello world"}),
			wantArgs:    []string{"helper", "steps", "serve", "echo hello world"},
			wantUnset:   true,
		},
		"malformed base64 is ignored": {
			initialArgs: []string{"helper"},
			envValue:    "not!valid!base64!",
			wantArgs:    []string{"helper"},
			wantUnset:   false,
		},
		"malformed json inside valid base64 is ignored": {
			initialArgs: []string{"helper"},
			envValue:    base64.StdEncoding.EncodeToString([]byte("{not json")),
			wantArgs:    []string{"helper"},
			wantUnset:   false,
		},
		"empty argv inside payload is ignored": {
			initialArgs: []string{"helper"},
			envValue:    encode(t, []string{}),
			wantArgs:    []string{"helper"},
			wantUnset:   false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			savedArgs := os.Args
			t.Cleanup(func() { os.Args = savedArgs })

			os.Args = append([]string{}, tt.initialArgs...)

			if tt.envUnset {
				t.Setenv(steps.RecoveryEnvVar, "")
				require.NoError(t, os.Unsetenv(steps.RecoveryEnvVar))
			} else {
				t.Setenv(steps.RecoveryEnvVar, tt.envValue)
			}

			steps.RecoverArgv()

			assert.Equal(t, tt.wantArgs, os.Args)
			_, present := os.LookupEnv(steps.RecoveryEnvVar)
			assert.Equal(t, !tt.wantUnset, present, "env var presence after recovery")
		})
	}
}

func TestEncodeRecoveryArgv_RoundTrip(t *testing.T) {
	argv := []string{"steps", "serve", "/bin/sh", "echo hello & background"}
	encoded, err := steps.EncodeRecoveryArgv(argv)
	require.NoError(t, err)

	raw, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)

	var decoded []string
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assert.Equal(t, argv, decoded)
}

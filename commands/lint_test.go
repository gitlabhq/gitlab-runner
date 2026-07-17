//go:build !integration

package commands_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/commands"
)

// TestNewLintCommand_DoesNotLeakConfigFileEnv verifies that constructing the
// command does not set the CONFIG_FILE environment variable as a side effect.
// Flag defaults are captured on the struct field, not written back to env.
func TestNewLintCommand_DoesNotLeakConfigFileEnv(t *testing.T) {
	t.Setenv("CONFIG_FILE", "/test/sentinel/path")

	_ = commands.NewLintCommand()

	val, _ := os.LookupEnv("CONFIG_FILE")
	assert.Equal(t, "/test/sentinel/path", val, "command construction must not mutate CONFIG_FILE")
}

func TestLintConfigFile(t *testing.T) {
	tests := map[string]struct {
		fixtureFile string
		wantErr     bool
		errContains string
	}{
		"valid config": {
			fixtureFile: "testdata/lint/valid.toml",
			wantErr:     false,
		},
		"misspelled top-level key": {
			fixtureFile: "testdata/lint/misspelled_key.toml",
			wantErr:     true,
			errContains: "buids_dir",
		},
		"wrong section header": {
			fixtureFile: "testdata/lint/wrong_section.toml",
			wantErr:     true,
			// Assert on the full undecoded path to avoid matching the
			// substring "runner" inside "runners".
			errContains: "runner.kubernetes",
		},
		"invalid TOML syntax": {
			fixtureFile: "testdata/lint/invalid_syntax.toml",
			wantErr:     true,
			errContains: "decoding config",
		},
		"file not found": {
			fixtureFile: "testdata/lint/nonexistent.toml",
			wantErr:     true,
			errContains: "config file not found",
		},
		"invalid shell value": {
			fixtureFile: "testdata/lint/bad_shell.toml",
			wantErr:     true,
			errContains: "config schema validation failed",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := commands.LintConfigFile(tc.fixtureFile)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}
			require.NoError(t, err)
		})
	}
}

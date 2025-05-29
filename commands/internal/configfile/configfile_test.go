//go:build !integration

package configfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func Test_loadConfig(t *testing.T) {
	const expectedSystemIDRegexPattern = "^[sr]_[0-9a-zA-Z]{12}$"

	testCases := map[string]struct {
		runnerSystemID string
		prepareFn      func(t *testing.T, systemIDFile string)
		assertFn       func(t *testing.T, err error, config *common.Config, systemIDFile string)
	}{
		"generates and saves missing system IDs": {
			runnerSystemID: "",
			assertFn: func(t *testing.T, err error, config *common.Config, systemIDFile string) {
				assert.NoError(t, err)
				require.Equal(t, 1, len(config.Runners))
				assert.NotEmpty(t, config.Runners[0].SystemID)
				content, err := os.ReadFile(systemIDFile)
				require.NoError(t, err)
				assert.Contains(t, string(content), config.Runners[0].SystemID)
			},
		},
		"preserves existing unique system IDs": {
			runnerSystemID: "s_c2d22f638c25",
			assertFn: func(t *testing.T, err error, config *common.Config, _ string) {
				assert.NoError(t, err)
				require.Equal(t, 1, len(config.Runners))
				assert.Equal(t, "s_c2d22f638c25", config.Runners[0].SystemID)
			},
		},
		"regenerates system ID if file is invalid": {
			runnerSystemID: "0123456789",
			assertFn: func(t *testing.T, err error, config *common.Config, _ string) {
				assert.NoError(t, err)
				require.Equal(t, 1, len(config.Runners))
				assert.Regexp(t, expectedSystemIDRegexPattern, config.Runners[0].SystemID)
			},
		},
		"succeeds if file cannot be created": {
			runnerSystemID: "",
			prepareFn: func(t *testing.T, systemIDFile string) {
				require.NoError(t, os.Remove(systemIDFile))
				require.NoError(t, os.Chmod(filepath.Dir(systemIDFile), os.ModeDir|0500))
			},
			assertFn: func(t *testing.T, err error, config *common.Config, _ string) {
				require.NoError(t, err)
				require.Equal(t, 1, len(config.Runners))
				assert.Regexp(t, expectedSystemIDRegexPattern, config.Runners[0].SystemID)
			},
		},
	}

	const config = `
[[runners]]
  name = "runner"
  token = "glrt-some-random-token"
  url = "https://some.gitlab.instance.tld/"
`

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			dir := t.TempDir()
			cfgName := filepath.Join(dir, "config.toml")
			systemIDFile := filepath.Join(dir, ".runner_system_id")

			require.NoError(t, os.Chmod(dir, 0777))
			require.NoError(t, os.WriteFile(cfgName, []byte(config), 0777))
			require.NoError(t, os.WriteFile(systemIDFile, []byte(tc.runnerSystemID), 0777))

			if tc.prepareFn != nil {
				tc.prepareFn(t, systemIDFile)
			}

			logGlobal := test.NewGlobal()

			cfg := New(cfgName)
			err := cfg.Load()

			for _, entry := range logGlobal.AllEntries() {
				assert.NotContains(t, entry.Message, "problem with your config based on jsonschema annotations")
			}

			tc.assertFn(t, err, cfg.Config(), systemIDFile)

			// Cleanup
			require.NoError(t, os.Chmod(dir, 0777))
		})
	}
}

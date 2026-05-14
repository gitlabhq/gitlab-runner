//go:build !integration

package commands_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers"
)

// TestCommandConstructorsDoNotMutateProcessEnv asserts that command
// construction must not mutate the process environment. Anything left on
// os.Environ() at this stage can leak into subprocesses or any consumer
// that snapshots the environment, regardless of which binary or subcommand
// the user actually invoked.
func TestCommandConstructorsDoNotMutateProcessEnv(t *testing.T) {
	t.Setenv("CONFIG_FILE", "")
	require.NoError(t, os.Unsetenv("CONFIG_FILE"))

	_ = commands.NewListCommand()
	_ = commands.NewUnregisterCommand(nil)
	_ = commands.NewVerifyCommand(nil)
	_ = commands.NewResetTokenCommand(nil)

	_ = helpers.NewArtifactsDownloaderCommand()
	_ = helpers.NewArtifactsUploaderCommand()
	_ = helpers.NewCacheArchiverCommand()
	_ = helpers.NewCacheExtractorCommand()
	_ = helpers.NewCacheInitCommand()
	_ = helpers.NewHealthCheckCommand()
	_ = helpers.NewProxyExecCommand()
	_ = helpers.NewReadLogsCommand()

	_, set := os.LookupEnv("CONFIG_FILE")
	assert.False(t, set, "command construction must not set CONFIG_FILE; see gitlab-runner#39454")
}

// TestListCommandConfigFilePrecedence documents the precedence chain for
// resolving --config on a manager command: struct default, CONFIG_FILE env
// override, and --config CLI flag override.
func TestListCommandConfigFilePrecedence(t *testing.T) {
	tests := []struct {
		name string
		env  string
		args []string
		want string
	}{
		{"struct default", "", nil, commands.GetDefaultConfigFile()},
		{"CONFIG_FILE env overrides default", "/from/env.toml", nil, "/from/env.toml"},
		{"--config flag overrides env", "/from/env.toml", []string{"--config", "/from/flag.toml"}, "/from/flag.toml"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env == "" {
				t.Setenv("CONFIG_FILE", "")
				require.NoError(t, os.Unsetenv("CONFIG_FILE"))
			} else {
				t.Setenv("CONFIG_FILE", tc.env)
			}

			app := cli.NewApp()
			app.Commands = []cli.Command{commands.NewListCommand()}

			got := ""
			for i, c := range app.Commands {
				if c.Name != "list" {
					continue
				}
				app.Commands[i].Action = func(ctx *cli.Context) error {
					got = ctx.String("config")
					return nil
				}
			}

			require.NoError(t, app.Run(append([]string{"gitlab-runner", "list"}, tc.args...)))
			assert.Equal(t, tc.want, got)
		})
	}
}

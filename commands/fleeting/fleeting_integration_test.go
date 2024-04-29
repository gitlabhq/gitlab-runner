//go:build integration

package fleeting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"gitlab.com/gitlab-org/fleeting/fleeting-artifact/pkg/installer"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func init() {
	osExit = func(code int) {
		if code == 0 {
			return
		}
		panic(code)
	}
}

func TestInstall(t *testing.T) {

	app := cli.NewApp()
	app.Name = "runner"
	app.Commands = common.GetCommands()

	const config = `
[[runners]]
  [runners.autoscaler]
    plugin = "aws:0.5.0"
`

	configPath := filepath.Join(t.TempDir(), "test.toml")

	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o777))

	// no error installing multiple times
	require.NoError(t, app.Run([]string{"runner", "fleeting", "-c", configPath, "install"}))
	require.NoError(t, app.Run([]string{"runner", "fleeting", "-c", configPath, "install"}))

	// ensure plugin installed
	require.DirExists(t, filepath.Join(installer.InstallDir(), "registry.gitlab.com/gitlab-org/fleeting/plugins/aws/0.5.0"))
}

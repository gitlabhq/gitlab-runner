//go:build integration

package helpers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers"
	testHelpers "gitlab.com/gitlab-org/gitlab-runner/helpers"
)

func newCacheInitTestApp() *cli.App {
	cmd := &helpers.CacheInitCommand{}
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Commands = append(app.Commands, cli.Command{
		Name:   "cache-init",
		Action: cmd.Execute,
	})

	return app
}

func TestCacheInit(t *testing.T) {
	dir := t.TempDir()

	// Make sure that the mode is not the expected 0777.
	err := os.Chmod(dir, 0600)
	require.NoError(t, err)

	// Start a new cli with the arguments for the command.
	args := []string{os.Args[0], "cache-init", dir}
	err = newCacheInitTestApp().Run(args)
	require.NoError(t, err)

	info, err := os.Stat(dir)
	require.NoError(t, err)

	assert.Equal(t, os.ModeDir+os.ModePerm, info.Mode())
}

func TestCacheInit_NoArguments(t *testing.T) {
	removeHook := testHelpers.MakeFatalToPanic()
	defer removeHook()

	args := []string{os.Args[0], "cache-init"}

	assert.Panics(t, func() {
		_ = newCacheInitTestApp().Run(args)
	})
}

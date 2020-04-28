package helpers

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// CacheInitCommand will take a single directory/file path and initialize it
// correctly for it to be used for cache. This command tries to support spaces
// in directories name by using the the flags to specify which entries you want
// to initialize.
type CacheInitCommand struct{}

func (c *CacheInitCommand) Execute(ctx *cli.Context) {
	if ctx.NArg() == 0 {
		logrus.Fatal("No arguments passed, at least 1 path is required.")
	}

	for _, path := range ctx.Args() {
		err := os.Chmod(path, os.ModePerm)
		if err != nil {
			logrus.WithError(err).Error("failed to chmod path")
		}
	}
}

func init() {
	common.RegisterCommand2("cache-init", "changed permissions for cache paths (internal)", &CacheInitCommand{})
}

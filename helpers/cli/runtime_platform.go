package cli_helpers

import (
	"os"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func LogRuntimePlatform(app *cli.App) {
	appBefore := app.Before
	app.Before = func(c *cli.Context) error {
		fields := logrus.Fields{
			"os":       runtime.GOOS,
			"arch":     runtime.GOARCH,
			"version":  common.VERSION,
			"revision": common.REVISION,
			"pid":      os.Getpid(),
		}

		logrus.WithFields(fields).Info("Runtime platform")

		if appBefore != nil {
			return appBefore(c)
		}
		return nil
	}
}

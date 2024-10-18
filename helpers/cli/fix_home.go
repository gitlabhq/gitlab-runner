package cli_helpers

import (
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/homedir"
)

func FixHOME(app *cli.App) {
	appBefore := app.Before

	app.Before = func(c *cli.Context) error {
		err := homedir.Fix()
		if err != nil {
			return err
		}

		if appBefore != nil {
			return appBefore(c)
		}
		return nil
	}
}

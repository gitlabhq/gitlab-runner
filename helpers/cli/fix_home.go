package cli_helpers

import (
	"fmt"
	"os"

	"github.com/docker/docker/pkg/homedir"
	"github.com/urfave/cli"
)

func FixHOME(app *cli.App) {
	appBefore := app.Before

	app.Before = func(c *cli.Context) error {
		// Fix home
		if key := homedir.Key(); os.Getenv(key) == "" {
			value := homedir.Get()
			if value == "" {
				return fmt.Errorf("the %q is not set", key)
			}
			_ = os.Setenv(key, value)
		}

		if appBefore != nil {
			return appBefore(c)
		}
		return nil
	}
}

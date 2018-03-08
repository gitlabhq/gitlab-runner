package cli_helpers

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"os"
)

var (
	DefaultLogLevel   = log.InfoLevel
	CustomLogLevelSet = false
)

func SetupLogLevelOptions(app *cli.App) {
	newFlags := []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "debug mode",
			EnvVar: "DEBUG",
		},
		cli.StringFlag{
			Name:  "log-level, l",
			Usage: "Log level (options: debug, info, warn, error, fatal, panic)",
		},
	}
	app.Flags = append(app.Flags, newFlags...)

	appBefore := app.Before
	// logs
	app.Before = func(c *cli.Context) error {
		log.SetOutput(os.Stderr)
		log.SetLevel(DefaultLogLevel)

		if c.IsSet("log-level") || c.IsSet("l") {
			level, err := log.ParseLevel(c.String("log-level"))
			if err != nil {
				log.Fatalf(err.Error())
			}

			setCustomLevel(level)
		} else if c.Bool("debug") {
			setCustomLevel(log.DebugLevel)
			go watchForGoroutinesDump()
		}

		if appBefore != nil {
			return appBefore(c)
		}
		return nil
	}
}

func setCustomLevel(level log.Level) {
	log.SetLevel(level)
	CustomLogLevelSet = true
}

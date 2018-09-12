package cli_helpers

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	LogFormatText = "text"
	LogFormatJSON = "json"
)

var (
	DefaultLogLevel   = logrus.InfoLevel
	CustomLogLevelSet = false
)

func ConfigureLogging(app *cli.App) {
	newFlags := []cli.Flag{
		cli.StringFlag{
			Name:   "log-format",
			Usage:  "Chose log format (options: text, json)",
			EnvVar: "LOG_FORMAT",
			Value:  LogFormatText,
		},
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "debug mode",
			EnvVar: "DEBUG",
		},
		cli.StringFlag{
			Name:   "log-level, l",
			Usage:  "Log level (options: debug, info, warn, error, fatal, panic)",
			EnvVar: "LOG_LEVEL",
		},
	}
	app.Flags = append(app.Flags, newFlags...)

	appBefore := app.Before
	// logs
	app.Before = func(c *cli.Context) error {
		logrus.SetOutput(os.Stderr)
		logrus.SetLevel(DefaultLogLevel)
		setFormat(c)

		if c.IsSet("log-level") || c.IsSet("l") {
			level, err := logrus.ParseLevel(c.String("log-level"))
			if err != nil {
				logrus.Fatalf(err.Error())
			}

			setCustomLevel(level)
		} else if c.Bool("debug") {
			setCustomLevel(logrus.DebugLevel)
			go watchForGoroutinesDump()
		}

		if appBefore != nil {
			return appBefore(c)
		}
		return nil
	}
}

func setFormat(c *cli.Context) {
	if !c.IsSet("log-format") {
		return
	}

	formats := map[string]logrus.Formatter{
		LogFormatText: new(logrus.TextFormatter),
		LogFormatJSON: new(logrus.JSONFormatter),
	}

	format := c.String("log-format")

	formatter, ok := formats[format]
	if !ok {
		formatNames := make([]string, 0)
		for name := range formats {
			formatNames = append(formatNames, name)
		}
		logrus.WithField("format", format).Fatalf("Unknown log format. Expected one of: %v", formatNames)
	}

	logrus.SetFormatter(formatter)
}

func setCustomLevel(level logrus.Level) {
	logrus.SetLevel(level)
	CustomLogLevelSet = true
}

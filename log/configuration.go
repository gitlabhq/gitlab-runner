package log

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/formatter"
)

const (
	FormatRunner = "runner"
	FormatText   = "text"
	FormatJSON   = "json"
)

var (
	defaultLogLevel = logrus.InfoLevel
	customLevelUsed = false

	logFlags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "debug mode",
			EnvVar: "DEBUG",
		},
		cli.StringFlag{
			Name:   "log-format",
			Usage:  "Choose log format (options: runner, text, json)",
			EnvVar: "LOG_FORMAT",
			Value:  FormatRunner,
		},
		cli.StringFlag{
			Name:   "log-level, l",
			Usage:  "Log level (options: debug, info, warn, error, fatal, panic)",
			EnvVar: "LOG_LEVEL",
		},
	}

	formats = map[string]logrus.Formatter{
		FormatRunner: new(formatter.RunnerTextFormatter),
		FormatText:   new(logrus.TextFormatter),
		FormatJSON:   new(logrus.JSONFormatter),
	}
)

func IsCustomLevelUsed() bool {
	return customLevelUsed
}

func ConfigureLogging(app *cli.App) {
	app.Flags = append(app.Flags, logFlags...)

	appBefore := app.Before
	app.Before = func(cliCtx *cli.Context) error {
		logrus.SetOutput(os.Stderr)

		setupFormatter(cliCtx)
		setupLevel(cliCtx)

		if appBefore != nil {
			return appBefore(cliCtx)
		}
		return nil
	}
}

func setupFormatter(cliCtx *cli.Context) {
	format := cliCtx.String("log-format")
	formatter, ok := formats[format]

	if !ok {
		logrus.WithField("format", format).Fatalf("Unknown log format. Expected one of: %v", formatNames())
	}

	logrus.SetFormatter(formatter)
}

func formatNames() []string {
	formatNames := make([]string, 0)
	for name := range formats {
		formatNames = append(formatNames, name)
	}

	return formatNames
}

func setupLevel(cliCtx *cli.Context) {
	if cliCtx.IsSet("log-level") || cliCtx.IsSet("l") {
		level, err := logrus.ParseLevel(cliCtx.String("log-level"))
		if err != nil {
			logrus.WithError(err).Fatal("Failed to parse log level")
		}

		logrus.SetLevel(level)
		customLevelUsed = true

		return
	}

	if cliCtx.Bool("debug") {
		go watchForGoroutinesDump()

		logrus.SetLevel(logrus.DebugLevel)
		customLevelUsed = true

		return
	}

	logrus.SetLevel(defaultLogLevel)
}

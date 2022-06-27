package log

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	FormatRunner = "runner"
	FormatText   = "text"
	FormatJSON   = "json"
)

var (
	configuration = NewConfig(logrus.StandardLogger())

	logFlags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "debug mode",
			EnvVar: "RUNNER_DEBUG",
		},
		cli.StringFlag{
			Name:   "log-format",
			Usage:  "Choose log format (options: runner, text, json)",
			EnvVar: "LOG_FORMAT",
		},
		cli.StringFlag{
			Name:   "log-level, l",
			Usage:  "Log level (options: debug, info, warn, error, fatal, panic)",
			EnvVar: "LOG_LEVEL",
		},
	}

	formats = map[string]logrus.Formatter{
		FormatRunner: new(RunnerTextFormatter),
		FormatText:   new(logrus.TextFormatter),
		FormatJSON:   new(logrus.JSONFormatter),
	}
)

func formatNames() []string {
	formatNames := make([]string, 0)
	for name := range formats {
		formatNames = append(formatNames, name)
	}

	return formatNames
}

type Config struct {
	logger *logrus.Logger
	level  logrus.Level
	format logrus.Formatter

	levelSetWithCli  bool
	formatSetWithCli bool

	goroutinesDumpStopCh chan bool
}

func (l *Config) IsLevelSetWithCli() bool {
	return l.levelSetWithCli
}

func (l *Config) IsFormatSetWithCli() bool {
	return l.formatSetWithCli
}

func (l *Config) handleCliCtx(cliCtx *cli.Context) error {
	if cliCtx.IsSet("log-level") || cliCtx.IsSet("l") {
		err := l.SetLevel(cliCtx.String("log-level"))
		if err != nil {
			return err
		}
		l.levelSetWithCli = true
	}

	if cliCtx.Bool("debug") {
		l.level = logrus.DebugLevel
		l.levelSetWithCli = true
	}

	if cliCtx.IsSet("log-format") {
		err := l.SetFormat(cliCtx.String("log-format"))
		if err != nil {
			return err
		}

		l.formatSetWithCli = true
	}

	l.ReloadConfiguration()

	return nil
}

func (l *Config) SetLevel(levelString string) error {
	level, err := logrus.ParseLevel(levelString)
	if err != nil {
		return fmt.Errorf("failed to parse log level: %w", err)
	}

	l.level = level

	return nil
}

func (l *Config) SetFormat(format string) error {
	formatter, ok := formats[format]
	if !ok {
		return fmt.Errorf("unknown log format %q, expected one of: %v", l.format, formatNames())
	}

	l.format = formatter

	return nil
}

func (l *Config) ReloadConfiguration() {
	l.logger.SetFormatter(l.format)
	l.logger.SetLevel(l.level)

	if l.level == logrus.DebugLevel {
		l.enableGoroutinesDump()
	} else {
		l.disableGoroutinesDump()
	}
}

func (l *Config) enableGoroutinesDump() {
	if l.goroutinesDumpStopCh != nil {
		return
	}

	l.goroutinesDumpStopCh = make(chan bool)

	watchForGoroutinesDump(l.logger, l.goroutinesDumpStopCh)
}

func (l *Config) disableGoroutinesDump() {
	if l.goroutinesDumpStopCh == nil {
		return
	}

	close(l.goroutinesDumpStopCh)
	l.goroutinesDumpStopCh = nil
}

func NewConfig(logger *logrus.Logger) *Config {
	return &Config{
		logger: logger,
		level:  logrus.InfoLevel,
		format: new(RunnerTextFormatter),
	}
}

func Configuration() *Config {
	return configuration
}

func ConfigureLogging(app *cli.App) {
	app.Flags = append(app.Flags, logFlags...)

	appBefore := app.Before
	app.Before = func(cliCtx *cli.Context) error {
		Configuration().logger.SetOutput(os.Stderr)

		err := Configuration().handleCliCtx(cliCtx)
		if err != nil {
			logrus.WithError(err).Fatal("Error while setting up logging configuration")
		}

		if appBefore != nil {
			return appBefore(cliCtx)
		}
		return nil
	}
}

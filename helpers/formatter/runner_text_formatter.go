package formatter

import (
	"bytes"
	"fmt"
	"runtime"
	"sort"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers"
)

type RunnerTextFormatter struct {
	// Force disabling colors.
	DisableColors bool
	// Force coloring output.
	ForceColors bool

	// The fields are sorted by default for a consistent output. For applications
	// that log extremely frequently and don't use the JSON formatter this may not
	// be desired.
	DisableSorting bool
}

func (f *RunnerTextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var keys = make([]string, 0, len(entry.Data))
	for k := range entry.Data {
		keys = append(keys, k)
	}

	if !f.DisableSorting {
		sort.Strings(keys)
	}

	b := bytes.Buffer{}
	f.printColored(&b, entry, keys)
	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *RunnerTextFormatter) printColored(b *bytes.Buffer, entry *logrus.Entry, keys []string) {
	levelColor, levelText, resetColor := f.getLevelColorAndText(entry.Level)
	indentLength := 50 - len(levelText)

	fmt.Fprintf(b, "%s%s%-*s%s ", levelColor, levelText, indentLength, entry.Message, resetColor)
	for _, k := range keys {
		v := entry.Data[k]
		fmt.Fprintf(b, " %s%s%s=%v", levelColor, k, resetColor, v)
	}
}

func (f *RunnerTextFormatter) getLevelColorAndText(level logrus.Level) (lvlColor, lvlText, resetColor string) {
	text := map[logrus.Level]string{
		logrus.WarnLevel:  "WARNING: ",
		logrus.ErrorLevel: "ERROR: ",
		logrus.FatalLevel: "FATAL: ",
		logrus.PanicLevel: "PANIC: ",
	}

	lvlText = text[level]

	if f.DisableColors == true && f.ForceColors != true {
		return
	}

	color := map[logrus.Level]string{
		logrus.DebugLevel: helpers.ANSI_BOLD_WHITE,
		logrus.WarnLevel:  helpers.ANSI_YELLOW,
		logrus.ErrorLevel: helpers.ANSI_BOLD_RED,
		logrus.FatalLevel: helpers.ANSI_BOLD_RED,
		logrus.PanicLevel: helpers.ANSI_BOLD_RED,
	}

	lvlColor = color[level]
	resetColor = helpers.ANSI_RESET
	return
}

func SetRunnerFormatter(app *cli.App) {
	logrus.SetFormatter(&RunnerTextFormatter{})

	newFlags := []cli.Flag{
		cli.BoolFlag{
			Name:   "no-color",
			Usage:  "Run in no-color mode",
			EnvVar: "NO_COLOR",
		},
		cli.BoolFlag{
			Name:   "force-color",
			Usage:  "Run in force-color mode",
			EnvVar: "FORCE_COLOR",
		},
	}
	app.Flags = append(app.Flags, newFlags...)

	appBefore := app.Before
	app.Before = func(c *cli.Context) error {
		logrus.SetFormatter(&RunnerTextFormatter{
			DisableColors: c.Bool("no-color") || runtime.GOOS == "windows",
			ForceColors:   c.Bool("force-color"),
		})

		if appBefore != nil {
			return appBefore(c)
		}

		return nil
	}
}

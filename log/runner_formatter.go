package log

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

type RunnerTextFormatter struct {
	// Force disabling colors.
	DisableColors bool

	// The fields are sorted by default for a consistent output. For applications
	// that log extremely frequently and don't use the JSON formatter this may not
	// be desired.
	DisableSorting bool
}

func (f *RunnerTextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := new(bytes.Buffer)
	f.printColored(b, entry)
	b.WriteByte('\n')

	return b.Bytes(), nil
}

func (f *RunnerTextFormatter) printColored(b *bytes.Buffer, entry *logrus.Entry) {
	levelColor, resetColor, levelPrefix := f.getColorsAndPrefix(entry)
	indentLength := 50 - len(levelPrefix)

	fmt.Fprintf(b, "%s%s%-*s%s ", levelColor, levelPrefix, indentLength, entry.Message, resetColor)
	for _, k := range f.prepareKeys(entry) {
		v := entry.Data[k]
		fmt.Fprintf(b, " %s%s%s=%v", levelColor, k, resetColor, v)
	}
}

func (f *RunnerTextFormatter) getColorsAndPrefix(entry *logrus.Entry) (string, string, string) {
	definitions := map[logrus.Level]struct {
		color  string
		prefix string
	}{
		logrus.DebugLevel: {
			color: helpers.ANSI_BOLD_WHITE,
		},
		logrus.WarnLevel: {
			color:  helpers.ANSI_YELLOW,
			prefix: "WARNING: ",
		},
		logrus.ErrorLevel: {
			color:  helpers.ANSI_BOLD_RED,
			prefix: "ERROR: ",
		},
		logrus.FatalLevel: {
			color:  helpers.ANSI_BOLD_RED,
			prefix: "FATAL: ",
		},
		logrus.PanicLevel: {
			color:  helpers.ANSI_BOLD_RED,
			prefix: "PANIC: ",
		},
	}

	color := ""
	prefix := ""

	definition, ok := definitions[entry.Level]
	if ok {
		if definition.color != "" {
			color = definition.color
		}

		if definition.prefix != "" {
			prefix = definition.prefix
		}
	}

	if f.DisableColors {
		return "", "", prefix
	}

	return color, helpers.ANSI_RESET, prefix
}

func (f *RunnerTextFormatter) prepareKeys(entry *logrus.Entry) []string {
	keys := make([]string, 0, len(entry.Data))

	for k := range entry.Data {
		keys = append(keys, k)
	}

	if !f.DisableSorting {
		sort.Strings(keys)
	}

	return keys
}

func SetRunnerFormatter() {
	logrus.SetFormatter(new(RunnerTextFormatter))
}

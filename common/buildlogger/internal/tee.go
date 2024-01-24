package internal

import (
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

// Tee is a log writer that targets both the job/build log _and_ the runner log,
// writing to both.
type Tee struct {
	logFn func(args ...any)

	entry *logrus.Entry

	// disable stops teeing to the runner log, this is essentially used by
	// runner tests where both the build and runner logs both use the same
	// destination (like stdout)
	disable bool
}

func NewTee(logFn func(args ...any), entry *logrus.Entry, disable bool) Tee {
	return Tee{logFn, entry, disable}
}

func (t *Tee) WithFields(fields logrus.Fields) Tee {
	return Tee{
		logFn:   t.logFn,
		entry:   t.entry.WithFields(fields),
		disable: t.disable,
	}
}

func (t *Tee) WriterLevel(level logrus.Level) *io.PipeWriter {
	return t.entry.WriterLevel(level)
}

func (t *Tee) log(level logrus.Level, logPrefix string, args ...interface{}) {
	if t.entry == nil {
		return
	}

	// log lines have spaces between each argument, followed by an ANSI Reset and *then* a new-line.
	//
	// To achieve this, we use fmt.Sprintln and remove the newline, add the ANSI Reset and then
	// append the newline again. The reason we don't use fmt.Sprint is that there's a greater
	// difference between that and fmt.Sprintln than just the newline character being added
	// (fmt.Sprintln consistently adds a space between arguments).
	logLine := fmt.Sprintln(args...)
	logLine = logLine[:len(logLine)-1]
	logLine += helpers.ANSI_RESET + "\n"

	if t.logFn != nil {
		t.logFn(logPrefix + logLine)
	}

	// don't tee to logrus entry (runner log) when disabled or no args
	if t.disable || len(args) == 0 {
		return
	}

	t.entry.Logln(level, args...)
}

func (t *Tee) Debugln(args ...interface{}) {
	if t.entry == nil {
		return
	}
	t.entry.Debugln(args...)
}

func (t *Tee) Println(args ...interface{}) {
	t.log(logrus.DebugLevel, helpers.ANSI_CLEAR, args...)
}

func (t *Tee) Infoln(args ...interface{}) {
	t.log(logrus.InfoLevel, helpers.ANSI_BOLD_GREEN, args...)
}

func (t *Tee) Warningln(args ...interface{}) {
	t.log(logrus.WarnLevel, helpers.ANSI_YELLOW+"WARNING: ", args...)
}

func (t *Tee) SoftErrorln(args ...interface{}) {
	t.log(logrus.WarnLevel, helpers.ANSI_BOLD_RED+"ERROR: ", args...)
}

func (t *Tee) Errorln(args ...interface{}) {
	t.log(logrus.ErrorLevel, helpers.ANSI_BOLD_RED+"ERROR: ", args...)
}

package common

import (
	"fmt"
	"io"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
)

type BuildLogger struct {
	log   JobTrace
	entry *logrus.Entry
}

type jobTraceIsMaskingURLParams interface {
	IsMaskingURLParams() bool
}

func NewBuildLogger(log JobTrace, entry *logrus.Entry) BuildLogger {
	return BuildLogger{
		log:   log,
		entry: entry,
	}
}

func (e *BuildLogger) WithFields(fields logrus.Fields) BuildLogger {
	return NewBuildLogger(e.log, e.entry.WithFields(fields))
}

func (e *BuildLogger) SendRawLog(args ...interface{}) {
	if e.log != nil {
		_, _ = fmt.Fprint(e.log, args...)
	}
}

func (e *BuildLogger) sendLog(logger func(args ...interface{}), logPrefix string, args ...interface{}) {
	if e.log != nil {
		// log lines have spaces between each argument, followed by an ANSI Reset and *then* a new-line.
		//
		// To achieve this, we use fmt.Sprintln and remove the newline, add the ANSI Reset and then
		// append the newline again. The reason we don't use fmt.Sprint is that there's a greater
		// difference between that and fmt.Sprintln than just the newline character being added
		// (fmt.Sprintln consistently adds a space between arguments).
		logLine := fmt.Sprintln(args...)
		logLine = logLine[:len(logLine)-1]

		if trace, ok := e.log.(jobTraceIsMaskingURLParams); !ok || !trace.IsMaskingURLParams() {
			logLine = url_helpers.ScrubSecrets(logLine)
		}
		logLine += helpers.ANSI_RESET + "\n"

		e.SendRawLog(logPrefix + logLine)

		if e.log.IsStdout() {
			return
		}
	}

	if len(args) == 0 {
		return
	}

	logger(args...)
}

func (e *BuildLogger) WriterLevel(level logrus.Level) *io.PipeWriter {
	return e.entry.WriterLevel(level)
}

func (e *BuildLogger) Debugln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.entry.Debugln(args...)
}

func (e *BuildLogger) Println(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Debugln, helpers.ANSI_CLEAR, args...)
}

func (e *BuildLogger) Infoln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Println, helpers.ANSI_BOLD_GREEN, args...)
}

func (e *BuildLogger) Warningln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Warningln, helpers.ANSI_YELLOW+"WARNING: ", args...)
}

func (e *BuildLogger) SoftErrorln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Warningln, helpers.ANSI_BOLD_RED+"ERROR: ", args...)
}

func (e *BuildLogger) Errorln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Errorln, helpers.ANSI_BOLD_RED+"ERROR: ", args...)
}

type ProcessLoggerAdapter struct {
	buildLogger BuildLogger
}

func NewProcessLoggerAdapter(buildlogger BuildLogger) *ProcessLoggerAdapter {
	return &ProcessLoggerAdapter{
		buildLogger: buildlogger,
	}
}

func (l *ProcessLoggerAdapter) WithFields(fields logrus.Fields) process.Logger {
	l.buildLogger = l.buildLogger.WithFields(fields)

	return l
}

func (l *ProcessLoggerAdapter) Warn(args ...interface{}) {
	l.buildLogger.Warningln(args...)
}

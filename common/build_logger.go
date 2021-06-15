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
		logLine := url_helpers.ScrubSecrets(logPrefix + fmt.Sprintln(args...))
		e.SendRawLog(logLine)
		e.SendRawLog(helpers.ANSI_RESET)

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

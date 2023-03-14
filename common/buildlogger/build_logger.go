package buildlogger

import (
	"fmt"
	"io"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
)

type Trace interface {
	Write([]byte) (int, error)
	IsStdout() bool
}

type Logger struct {
	log      Trace
	entry    *logrus.Entry
	streamID int
}

const (
	// StreamExecutorLevel is the stream number for an executor log line
	StreamExecutorLevel = 0
	// StreamWorkLevel is the stream number for a work log line
	StreamWorkLevel = 1
	// StreamStartingServiceLevel is the starting stream number for a service log line
	StreamStartingServiceLevel = 15
)

type jobTraceIsMaskingURLParams interface {
	IsMaskingURLParams() bool
}

func New(log Trace, entry *logrus.Entry) Logger {
	return Logger{
		log:   log,
		entry: entry,
	}
}

func (e *Logger) Stdout() io.Writer {
	return e.log
}

func (e *Logger) Stderr() io.Writer {
	return e.log
}

func (e *Logger) StreamID(streamID int) Logger {
	return Logger{
		log:      e.log,
		entry:    e.entry,
		streamID: streamID,
	}
}

func (e *Logger) WithFields(fields logrus.Fields) Logger {
	return New(e.log, e.entry.WithFields(fields))
}

func (e *Logger) SendRawLog(args ...interface{}) {
	if e.log != nil {
		_, _ = fmt.Fprint(e.log, args...)
	}
}

func (e *Logger) sendLog(logger func(args ...interface{}), logPrefix string, args ...interface{}) {
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

func (e *Logger) WriterLevel(level logrus.Level) *io.PipeWriter {
	return e.entry.WriterLevel(level)
}

func (e *Logger) Debugln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.entry.Debugln(args...)
}

func (e *Logger) Println(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Debugln, helpers.ANSI_CLEAR, args...)
}

func (e *Logger) Infoln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Println, helpers.ANSI_BOLD_GREEN, args...)
}

func (e *Logger) Warningln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Warningln, helpers.ANSI_YELLOW+"WARNING: ", args...)
}

func (e *Logger) SoftErrorln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Warningln, helpers.ANSI_BOLD_RED+"ERROR: ", args...)
}

func (e *Logger) Errorln(args ...interface{}) {
	if e.entry == nil {
		return
	}
	e.sendLog(e.entry.Errorln, helpers.ANSI_BOLD_RED+"ERROR: ", args...)
}

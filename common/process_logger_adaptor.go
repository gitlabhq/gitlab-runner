package common

import (
	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

type ProcessLoggerAdapter struct {
	buildLogger buildlogger.Logger
}

func NewProcessLoggerAdapter(buildlogger buildlogger.Logger) *ProcessLoggerAdapter {
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

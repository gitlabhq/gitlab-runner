package command

import (
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

// processLogger implements the logger interface defined in
// gitlab.com/gitlab-org/gitlab-runner/helpers/process package.
type processLogger struct {
	buildLogger common.BuildLogger
}

func (l *processLogger) WithFields(fields logrus.Fields) process.Logger {
	l.buildLogger = l.buildLogger.WithFields(fields)

	return l
}

func (l *processLogger) Warn(args ...interface{}) {
	l.buildLogger.Warningln(args...)
}

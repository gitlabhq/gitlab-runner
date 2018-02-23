package commands

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/service/mocks"
)

func TestServiceLogHook(t *testing.T) {
	formatter := new(logrus.TextFormatter)
	formatter.DisableColors = true
	formatter.DisableTimestamp = true

	logger := &logrus.Logger{
		Formatter: formatter,
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.InfoLevel,
		Out:       os.Stderr,
	}

	mockServiceLogger := new(mocks.Logger)
	mockServiceLogger.On("Info", "level=info msg=test\n").Return(nil)

	logHook := &ServiceLogHook{mockServiceLogger, logrus.InfoLevel}

	logger.Hooks.Add(logHook)

	logger.Info("test")

	mockServiceLogger.AssertExpectations(t)
}

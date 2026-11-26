package log

import (
	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
)

type systemLogger interface {
	service.Logger
}

type systemService interface {
	service.Service
}

type SystemServiceLogHook struct {
	systemLogger
}

// Levels returns all levels: logrus only fires hooks for levels the logger
// currently has enabled, so log_level changes on config reload apply here too.
func (s *SystemServiceLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (s *SystemServiceLogHook) Fire(entry *logrus.Entry) error {
	msg, err := entry.String()
	if err != nil {
		return err
	}

	switch entry.Level {
	case logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel:
		return s.Error(msg)
	case logrus.WarnLevel:
		return s.Warning(msg)
	default:
		// service.Logger has no debug or trace method
		return s.Info(msg)
	}
}

func SetSystemLogger(logrusLogger *logrus.Logger, svc systemService) {
	logger, err := svc.SystemLogger(nil)

	if err == nil {
		hook := new(SystemServiceLogHook)
		hook.systemLogger = logger

		logrusLogger.AddHook(hook)
	} else {
		logrusLogger.WithError(err).Error("Error while setting up the system logger")
	}
}

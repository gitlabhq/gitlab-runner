package log

import (
	"github.com/ayufan/golang-kardianos-service"
	"github.com/sirupsen/logrus"
)

type ServiceLogHook struct {
	service.Logger
	Level logrus.Level
}

func (s *ServiceLogHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
	}
}

func (s *ServiceLogHook) Fire(entry *logrus.Entry) error {
	if entry.Level > s.Level {
		return nil
	}

	msg, err := entry.String()
	if err != nil {
		return err
	}

	switch entry.Level {
	case logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel:
		s.Error(msg)
	case logrus.WarnLevel:
		s.Warning(msg)
	case logrus.InfoLevel:
		s.Info(msg)
	}

	return nil
}

func SetSystemLogger(svc service.Service) {
	logrus.SetFormatter(new(logrus.TextFormatter))

	logger, err := svc.SystemLogger(nil)
	if err == nil {
		logrus.AddHook(&ServiceLogHook{logger, logrus.InfoLevel})
	} else {
		logrus.Errorln(err)
	}
}

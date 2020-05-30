package process

import (
	"github.com/sirupsen/logrus"
)

type Logger interface {
	WithFields(fields logrus.Fields) Logger
	Warn(args ...interface{})
}

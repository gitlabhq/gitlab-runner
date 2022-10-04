package process

import (
	"github.com/sirupsen/logrus"
)

//go:generate mockery --name=Logger --inpackage
type Logger interface {
	WithFields(fields logrus.Fields) Logger
	Warn(args ...interface{})
}

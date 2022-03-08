package helpers

import (
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

type warningLogHook struct {
	output io.Writer
}

func (s *warningLogHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.WarnLevel,
	}
}

func (s *warningLogHook) Fire(e *logrus.Entry) error {
	_, _ = fmt.Fprintln(s.output, e.Message)

	panic(e)
}

func MakeWarningToPanic() func() {
	logger := logrus.StandardLogger()
	hooks := make(logrus.LevelHooks)

	hooks.Add(&warningLogHook{output: logger.Out})
	oldHooks := logger.ReplaceHooks(hooks)

	return func() {
		logger.ReplaceHooks(oldHooks)
	}
}

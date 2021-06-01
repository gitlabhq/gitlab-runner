package helpers

import (
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

type fatalLogHook struct {
	output io.Writer
}

func (s *fatalLogHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.FatalLevel,
	}
}

func (s *fatalLogHook) Fire(e *logrus.Entry) error {
	_, _ = fmt.Fprintln(s.output, e.Message)

	panic(e)
}

func MakeFatalToPanic() func() {
	logger := logrus.StandardLogger()
	hooks := make(logrus.LevelHooks)

	hooks.Add(&fatalLogHook{output: logger.Out})
	oldHooks := logger.ReplaceHooks(hooks)

	return func() {
		logger.ReplaceHooks(oldHooks)
	}
}

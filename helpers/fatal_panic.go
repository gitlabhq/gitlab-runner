package helpers

import (
	"fmt"
	"io"

	"github.com/Sirupsen/logrus"
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
	fmt.Fprint(s.output, e.Message)

	panic(e)
}

func MakeFatalToPanic() func() {
	hook := &fatalLogHook{
		output: logrus.StandardLogger().Out,
	}
	logrus.AddHook(hook)

	removeHook := func() {
		for level, levelHooks := range logrus.StandardLogger().Hooks {
			hooks := []logrus.Hook{}
			for _, existingHook := range levelHooks {
				if existingHook != hook {
					hooks = append(hooks, existingHook)
				}
			}
			logrus.StandardLogger().Hooks[level] = hooks
		}
	}

	return removeHook
}

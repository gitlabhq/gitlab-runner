package sentry

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	sentrygo "github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const messageFlushTimeout = 10 * time.Second

type LogHook struct {
	hub *sentrygo.Hub
}

func (s *LogHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
	}
}

func sentryLevelFromLogrusLevel(logrusLevel logrus.Level) sentrygo.Level {
	if logrusLevel == logrus.PanicLevel || logrusLevel == logrus.FatalLevel {
		return sentrygo.LevelFatal
	}
	return sentrygo.LevelError
}

func (s *LogHook) Fire(entry *logrus.Entry) error {
	if s.hub == nil {
		return nil
	}

	tags := make(map[string]string)
	for key, value := range entry.Data {
		tags[key] = fmt.Sprint(value)
	}
	level := sentryLevelFromLogrusLevel(entry.Level)

	scope := s.hub.PushScope()
	defer s.hub.PopScope()

	scope.SetTags(tags)
	scope.SetLevel(level)

	s.hub.CaptureException(errors.New(entry.Message))
	if level == sentrygo.LevelFatal {
		s.hub.Flush(messageFlushTimeout)
	}

	return nil
}

func NewLogHook(dsn string) (lh LogHook, err error) {
	tags := make(map[string]string)
	tags["built"] = common.BUILT
	tags["version"] = common.VERSION
	tags["revision"] = common.REVISION
	tags["branch"] = common.BRANCH
	tags["go-version"] = runtime.Version()
	tags["go-os"] = runtime.GOOS
	tags["go-arch"] = runtime.GOARCH
	tags["hostname"], _ = os.Hostname()

	scope := sentrygo.NewScope()
	client, err := sentrygo.NewClient(sentrygo.ClientOptions{
		Dsn: dsn,
	})

	if err != nil {
		return
	}

	hub := sentrygo.NewHub(client, scope)
	hub.ConfigureScope(func(scope *sentrygo.Scope) {
		scope.SetTags(tags)
	})
	lh.hub = hub

	return
}

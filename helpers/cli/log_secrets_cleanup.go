package cli_helpers

import (
	"github.com/Sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/url"
)

type SecretsCleanupHook struct{}

func (s *SecretsCleanupHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}

func (s *SecretsCleanupHook) Fire(entry *logrus.Entry) error {
	entry.Message = url_helpers.ScrubSecrets(entry.Message)
	return nil
}

func AddSecretsCleanupLogHook() {
	logrus.AddHook(&SecretsCleanupHook{})
}

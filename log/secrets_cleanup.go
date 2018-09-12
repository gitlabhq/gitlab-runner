package log

import (
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/url"
)

type SecretsCleanupHook struct{}

func (s *SecretsCleanupHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (s *SecretsCleanupHook) Fire(entry *logrus.Entry) error {
	entry.Message = url_helpers.ScrubSecrets(entry.Message)
	return nil
}

func AddSecretsCleanupLogHook() {
	logrus.AddHook(new(SecretsCleanupHook))
}

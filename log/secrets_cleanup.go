package log

import (
	"github.com/sirupsen/logrus"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
)

type SecretsCleanupHook struct{}

func (s *SecretsCleanupHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (s *SecretsCleanupHook) Fire(entry *logrus.Entry) error {
	entry.Message = url_helpers.ScrubSecrets(entry.Message)
	return nil
}

func AddSecretsCleanupLogHook(logger *logrus.Logger) {
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	logger.AddHook(new(SecretsCleanupHook))
}

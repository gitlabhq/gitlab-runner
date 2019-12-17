package referees

import (
	"bytes"
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

type Referee interface {
	Execute(
		ctx context.Context,
		startTime time.Time,
		endTime time.Time,
	) (*bytes.Reader, error)
	ArtifactBaseName() string
	ArtifactType() string
	ArtifactFormat() string
}

type Config struct {
	Metrics *MetricsRefereeConfig `toml:"metrics,omitempty" json:"metrics" namespace:"metrics"`
}

func CreateReferees(executor interface{}, config *Config, log *logrus.Entry) []Referee {
	logger := log.WithField("context", "referee")

	if config == nil {
		logger.Info("no referees configured")
		return nil
	}

	var referees []Referee
	metricsReferee := CreateMetricsReferee(executor, config, log)
	if metricsReferee != nil {
		referees = append(referees, metricsReferee)
	}

	return referees
}

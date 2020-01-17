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

type refereeFactory func(executor interface{}, config *Config, log logrus.FieldLogger) Referee

type Config struct {
	Metrics *MetricsRefereeConfig `toml:"metrics,omitempty" json:"metrics" namespace:"metrics"`
}

var refereeFactories = []refereeFactory{
	newMetricsReferee,
}

func CreateReferees(executor interface{}, config *Config, log logrus.FieldLogger) []Referee {
	if config == nil {
		log.Debug("No referees configured")
		return nil
	}

	var referees []Referee
	for _, factory := range refereeFactories {
		referee := factory(executor, config, log)
		if referee != nil {
			referees = append(referees, referee)
		}
	}

	return referees
}

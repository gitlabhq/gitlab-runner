package referees

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func Test_CreateReferees(t *testing.T) {
	testCases := map[string]struct {
		executor         interface{}
		config           *Config
		expectedReferees []Referee
	}{
		"Executor doesn't support any referee": {
			executor:         new(mockExecutor),
			config:           &Config{Metrics: &MetricsRefereeConfig{QueryInterval: 0}},
			expectedReferees: nil,
		},
		"Executor supports metrics referee": {
			executor:         new(mockMetricsExecutor),
			config:           &Config{Metrics: &MetricsRefereeConfig{QueryInterval: 0}},
			expectedReferees: []Referee{&MetricsReferee{}},
		},
		"No config provided": {
			executor:         new(mockMetricsExecutor),
			config:           nil,
			expectedReferees: nil,
		},
	}

	for testName, test := range testCases {
		t.Run(testName, func(t *testing.T) {
			logger := logrus.WithField("test", t.Name())

			referees := CreateReferees(test.executor, test.config, logger)

			if test.expectedReferees == nil {
				assert.Nil(t, referees)
				return
			}

			assert.Len(t, referees, len(test.expectedReferees))
			for i, referee := range test.expectedReferees {
				assert.IsType(t, referee, referees[i])
			}
		})
	}
}

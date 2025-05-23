//go:build !integration

package referees

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func Test_CreateReferees(t *testing.T) {
	fakeMockMetricsExecutor := func(t *testing.T) interface{} {
		return struct{}{}
	}

	mockMetricsExecutor := func(t *testing.T) interface{} {
		m := NewMockMetricsExecutor(t)
		m.On("GetMetricsSelector").Return(`name="value"`).Maybe()
		return m
	}

	testCases := map[string]struct {
		mockExecutor     func(t *testing.T) interface{}
		config           *Config
		expectedReferees []Referee
	}{
		"Executor doesn't support any referee": {
			mockExecutor:     fakeMockMetricsExecutor,
			config:           &Config{Metrics: &MetricsRefereeConfig{QueryInterval: 0}},
			expectedReferees: nil,
		},
		"Executor supports metrics referee": {
			mockExecutor:     mockMetricsExecutor,
			config:           &Config{Metrics: &MetricsRefereeConfig{QueryInterval: 0}},
			expectedReferees: []Referee{&MetricsReferee{}},
		},
		"No config provided": {
			mockExecutor:     mockMetricsExecutor,
			config:           nil,
			expectedReferees: nil,
		},
	}

	for testName, test := range testCases {
		t.Run(testName, func(t *testing.T) {
			logger := logrus.WithField("test", t.Name())

			executor := test.mockExecutor(t)

			referees := CreateReferees(executor, test.config, logger)

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

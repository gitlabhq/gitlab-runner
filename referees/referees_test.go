//go:build !integration

package referees

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_CreateReferees(t *testing.T) {
	fakeMockMetricsExecutor := func(t *testing.T) (interface{}, func(t mock.TestingT) bool) {
		return struct{}{}, func(t mock.TestingT) bool { return false }
	}

	mockMetricsExecutor := func(t *testing.T) (interface{}, func(t mock.TestingT) bool) {
		m := new(MockMetricsExecutor)

		m.On("GetMetricsSelector").Return(`name="value"`).Maybe()

		return m, m.AssertExpectations
	}

	testCases := map[string]struct {
		mockExecutor     func(t *testing.T) (interface{}, func(t mock.TestingT) bool)
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

			executor, assertExpectations := test.mockExecutor(t)
			defer assertExpectations(t)

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

// +build !integration

package machine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestIdleLimitExceeded(t *testing.T) {
	newConfig := func(
		limit int,
		idleCount int,
		idleCountMin int,
		idleScaleFactor float64,
		maxGrowthRate int,
	) *common.RunnerConfig {
		config := &common.RunnerConfig{
			Limit: limit,
		}
		config.Machine = &common.DockerMachine{
			IdleCount:       idleCount,
			IdleCountMin:    idleCountMin,
			IdleScaleFactor: idleScaleFactor,
			MaxGrowthRate:   maxGrowthRate,
		}
		return config
	}

	newData := func(creating int, idle int, used int) *machinesData {
		return &machinesData{
			Creating: creating,
			Idle:     idle,
			Used:     used,
		}
	}

	tests := map[string]struct {
		config                *common.RunnerConfig
		data                  *machinesData
		expectedLimitExceeded bool
	}{
		"IdleCount reached": {
			config:                newConfig(0, 10, 0, 0, 0),
			data:                  newData(0, 10, 0),
			expectedLimitExceeded: true,
		},
		"IdleCount available": {
			config:                newConfig(0, 10, 0, 0, 0),
			data:                  newData(0, 0, 0),
			expectedLimitExceeded: false,
		},
		"IdleScaleFactor reached": {
			config:                newConfig(0, 20, 0, 1.2, 0),
			data:                  newData(0, 12, 10),
			expectedLimitExceeded: true,
		},
		"IdleScaleFactor available": {
			config:                newConfig(0, 20, 0, 1.2, 0),
			data:                  newData(0, 10, 10),
			expectedLimitExceeded: false,
		},
		"limit reached": {
			config:                newConfig(100, 20, 0, 0, 0),
			data:                  newData(0, 10, 90),
			expectedLimitExceeded: true,
		},
		"limit available": {
			config:                newConfig(100, 20, 0, 0, 0),
			data:                  newData(0, 10, 85),
			expectedLimitExceeded: false,
		},
		"MaxGrowthRate reached": {
			config:                newConfig(0, 20, 0, 0, 10),
			data:                  newData(10, 10, 90),
			expectedLimitExceeded: true,
		},
		"MaxGrowthRate available": {
			config:                newConfig(0, 20, 0, 0, 10),
			data:                  newData(5, 10, 90),
			expectedLimitExceeded: false,
		},
		"IdleCountMin not fulfilled": {
			config:                newConfig(0, 10, 5, 1, 0),
			data:                  newData(0, 2, 0),
			expectedLimitExceeded: false,
		},
		"IdleCountMin fulfilled": {
			config:                newConfig(0, 10, 5, 1, 0),
			data:                  newData(0, 5, 0),
			expectedLimitExceeded: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.Equal(t, tt.expectedLimitExceeded, idleLimitExceeded(tt.config, tt.data))
		})
	}
}

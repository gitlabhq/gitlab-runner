//go:build !integration

package machine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type ilsRunnerConfig struct {
	limit           int
	idleCount       int
	idleCountMin    int
	idleScaleFactor float64
	idleTime        int
	maxGrowthRate   int
	maxBuilds       int
}

type ilsMachinesData struct {
	creating int
	idle     int
	used     int
}

type ilsMachineDetails struct {
	state     machineState
	usedCount int
	used      time.Time
}

func ilsNewRunnerConfig(c ilsRunnerConfig) *common.RunnerConfig {
	config := &common.RunnerConfig{
		Limit: c.limit,
	}
	config.Machine = &common.DockerMachine{
		IdleCount:       c.idleCount,
		IdleCountMin:    c.idleCountMin,
		IdleScaleFactor: c.idleScaleFactor,
		IdleTime:        c.idleTime,
		MaxGrowthRate:   c.maxGrowthRate,
		MaxBuilds:       c.maxBuilds,
	}

	return config
}

func ilsNewMachinesData(d ilsMachinesData) *machinesData {
	return &machinesData{
		Creating: d.creating,
		Idle:     d.idle,
		Used:     d.used,
	}
}

func ilsNewMachineDetails(d ilsMachineDetails) *machineDetails {
	return &machineDetails{
		State:     d.state,
		UsedCount: d.usedCount,
		Used:      d.used,
	}
}

func TestCanCreateIdle(t *testing.T) {
	tests := map[string]struct {
		config            ilsRunnerConfig
		data              ilsMachinesData
		expectedCanCreate bool
	}{
		"MaxMachinesGrowth exceeded": {
			config:            ilsRunnerConfig{maxGrowthRate: 10},
			data:              ilsMachinesData{creating: 10},
			expectedCanCreate: false,
		},
		"MaxMachinesGrowth not reached": {
			config:            ilsRunnerConfig{maxGrowthRate: 10},
			data:              ilsMachinesData{creating: 1},
			expectedCanCreate: false,
		},
		"limit exceeded": {
			config:            ilsRunnerConfig{limit: 10},
			data:              ilsMachinesData{creating: 5, idle: 3, used: 2},
			expectedCanCreate: false,
		},
		"limit not reached": {
			config:            ilsRunnerConfig{limit: 10, idleCount: 10},
			data:              ilsMachinesData{idle: 3, used: 2},
			expectedCanCreate: true,
		},
		"IdleCountMin not fulfilled and IdleScaleFactor evaluated to 0": {
			config:            ilsRunnerConfig{idleCount: 100, idleCountMin: 10, idleScaleFactor: 1},
			data:              ilsMachinesData{idle: 0, used: 0},
			expectedCanCreate: true,
		},
		"IdleCountMin not fulfilled and IdleScaleFactor evaluated to non 0 and not reached": {
			config:            ilsRunnerConfig{idleCount: 100, idleCountMin: 10, idleScaleFactor: 1},
			data:              ilsMachinesData{idle: 0, used: 1},
			expectedCanCreate: true,
		},
		"IdleCountMin not fulfilled and IdleScaleFactor evaluated to non 0 and exceeded": {
			config:            ilsRunnerConfig{idleCount: 100, idleCountMin: 10, idleScaleFactor: 1},
			data:              ilsMachinesData{idle: 5, used: 1},
			expectedCanCreate: true,
		},
		"IdleCountMin fulfilled and IdleScaleFactor evaluated to non 0 and not reached": {
			config:            ilsRunnerConfig{idleCount: 100, idleCountMin: 10, idleScaleFactor: 1},
			data:              ilsMachinesData{idle: 10, used: 15},
			expectedCanCreate: true,
		},
		"IdleCountMin fulfilled and IdleScaleFactor evaluated to non 0 and exceeded": {
			config:            ilsRunnerConfig{idleCount: 100, idleCountMin: 10, idleScaleFactor: 1},
			data:              ilsMachinesData{idle: 10, used: 1},
			expectedCanCreate: false,
		},
		"IdleCountMin not set and IdleScaleFactor evaluated to 0 and exceeded": {
			config:            ilsRunnerConfig{idleCount: 100, idleScaleFactor: 1},
			data:              ilsMachinesData{idle: 0, used: 0},
			expectedCanCreate: true,
		},
		"IdleScaleFactor evaluated to non 0 and not reached but IdleCount exceeded": {
			config:            ilsRunnerConfig{idleCount: 10, idleScaleFactor: 1},
			data:              ilsMachinesData{idle: 10, used: 100},
			expectedCanCreate: false,
		},
		"IdleCount exceeded": {
			config:            ilsRunnerConfig{idleCount: 10},
			data:              ilsMachinesData{idle: 10},
			expectedCanCreate: false,
		},
		"IdleCount not reached": {
			config:            ilsRunnerConfig{idleCount: 10},
			data:              ilsMachinesData{idle: 1},
			expectedCanCreate: true,
		},
		// It makes no sense to have the IdleCountMin at the same or higher level than IdleCount.
		// Preparing such configuration would practically remove the functionality added by IdleScaleFactor
		// and revert the IdleCount behavior to be "static number of idle machines to maintain".
		// As preventing that from happening and adding warnings or even errors for such case would
		// complicate the code, I think we can assume that user should understand how IdleCount, IdleCountMin
		// and IdleScaleFactor work together and that this case doesn't make sense.
		// The following two test cases are added to ensure that scaling still works, even when IdleCount and
		// IdleCountMin are messed up.
		"IdleCount exceeded and IdleCountMin not fulfilled": {
			config:            ilsRunnerConfig{idleCount: 5, idleCountMin: 10},
			data:              ilsMachinesData{idle: 8},
			expectedCanCreate: false,
		},
		"IdleCount exceeded and IdleCountMin not fulfilled and IdleScaleFactor evaluated to non 0 and not reached": {
			config:            ilsRunnerConfig{idleCount: 5, idleCountMin: 10, idleScaleFactor: 1},
			data:              ilsMachinesData{idle: 8, used: 9},
			expectedCanCreate: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			result := canCreateIdle(ilsNewRunnerConfig(tt.config), ilsNewMachinesData(tt.data))
			assert.Equal(t, tt.expectedCanCreate, result)
		})
	}
}

func TestShouldRemoveIdle(t *testing.T) {
	stubUsedTime := func(seconds int) time.Time {
		return time.Now().Add(time.Duration(seconds) * time.Second)
	}

	tests := map[string]struct {
		config         ilsRunnerConfig
		data           ilsMachinesData
		details        ilsMachineDetails
		expectedReason removeIdleReason
	}{
		"machine not in Idle state": {
			config:         ilsRunnerConfig{},
			data:           ilsMachinesData{},
			details:        ilsMachineDetails{state: machineStateCreating},
			expectedReason: dontRemoveIdleMachine,
		},
		"MaxBuilds exceeded": {
			config:         ilsRunnerConfig{idleCount: 10, maxBuilds: 1},
			data:           ilsMachinesData{idle: 1},
			details:        ilsMachineDetails{state: machineStateIdle, usedCount: 1},
			expectedReason: removeIdleReasonTooManyBuilds,
		},
		"MaxBuilds not reached": {
			config:         ilsRunnerConfig{idleCount: 10, maxBuilds: 10},
			data:           ilsMachinesData{idle: 1},
			details:        ilsMachineDetails{state: machineStateIdle, usedCount: 1},
			expectedReason: dontRemoveIdleMachine,
		},
		"limit exceeded": {
			config:         ilsRunnerConfig{idleCount: 10, limit: 15},
			data:           ilsMachinesData{creating: 5, idle: 5, used: 5},
			details:        ilsMachineDetails{state: machineStateIdle},
			expectedReason: removeIdleReasonTooManyMachines,
		},
		"IdleTime not exceeded and IdleCount not exceeded": {
			config:         ilsRunnerConfig{idleCount: 10, idleTime: 3600},
			data:           ilsMachinesData{idle: 5},
			details:        ilsMachineDetails{state: machineStateIdle, used: stubUsedTime(-60)},
			expectedReason: dontRemoveIdleMachine,
		},
		"IdleTime not exceeded and IdleCount exceeded": {
			config:         ilsRunnerConfig{idleCount: 10, idleTime: 3600},
			data:           ilsMachinesData{idle: 10},
			details:        ilsMachineDetails{state: machineStateIdle, used: stubUsedTime(-60)},
			expectedReason: dontRemoveIdleMachine,
		},
		"IdleTime exceeded and IdleCount not exceeded": {
			config:         ilsRunnerConfig{idleCount: 10, idleTime: 10},
			data:           ilsMachinesData{idle: 5},
			details:        ilsMachineDetails{state: machineStateIdle, used: stubUsedTime(-60)},
			expectedReason: dontRemoveIdleMachine,
		},
		"IdleTime exceeded and IdleCount exceeded": {
			config:         ilsRunnerConfig{idleCount: 10, idleTime: 10},
			data:           ilsMachinesData{idle: 10},
			details:        ilsMachineDetails{state: machineStateIdle, used: stubUsedTime(-60)},
			expectedReason: removeIdleReasonTooManyIdleMachines,
		},
		"IdleTime exceeded and IdleCountMin not fulfilled and IdleScaleFactor evaluated to 0": {
			config:         ilsRunnerConfig{idleCount: 100, idleCountMin: 10, idleScaleFactor: 1, idleTime: 10},
			data:           ilsMachinesData{idle: 0, used: 0},
			details:        ilsMachineDetails{state: machineStateIdle, used: stubUsedTime(-60)},
			expectedReason: dontRemoveIdleMachine,
		},
		"IdleTime exceeded and IdleCountMin not fulfilled and IdleScaleFactor evaluated to non 0 and not reached": {
			config:         ilsRunnerConfig{idleCount: 100, idleCountMin: 10, idleScaleFactor: 1, idleTime: 10},
			data:           ilsMachinesData{idle: 0, used: 1},
			details:        ilsMachineDetails{state: machineStateIdle, used: stubUsedTime(-60)},
			expectedReason: dontRemoveIdleMachine,
		},
		"IdleTime exceeded and IdleCountMin not fulfilled and IdleScaleFactor evaluated to non 0 and exceeded": {
			config:         ilsRunnerConfig{idleCount: 100, idleCountMin: 10, idleScaleFactor: 1, idleTime: 10},
			data:           ilsMachinesData{idle: 5, used: 1},
			details:        ilsMachineDetails{state: machineStateIdle, used: stubUsedTime(-60)},
			expectedReason: dontRemoveIdleMachine,
		},
		"IdleTime exceeded and IdleCountMin fulfilled and IdleScaleFactor evaluated to non 0 and not reached": {
			config:         ilsRunnerConfig{idleCount: 100, idleCountMin: 10, idleScaleFactor: 1, idleTime: 10},
			data:           ilsMachinesData{idle: 10, used: 15},
			details:        ilsMachineDetails{state: machineStateIdle, used: stubUsedTime(-60)},
			expectedReason: dontRemoveIdleMachine,
		},
		"IdleTime exceeded and IdleCountMin fulfilled and IdleScaleFactor evaluated to non 0 and exceeded": {
			config:         ilsRunnerConfig{idleCount: 100, idleCountMin: 10, idleScaleFactor: 1, idleTime: 10},
			data:           ilsMachinesData{idle: 10, used: 1},
			details:        ilsMachineDetails{state: machineStateIdle, used: stubUsedTime(-60)},
			expectedReason: removeIdleReasonTooManyIdleMachines,
		},
		"IdleTime exceeded and IdleCountMin not set and IdleScaleFactor evaluated to 0 and exceeded": {
			config:         ilsRunnerConfig{idleCount: 100, idleScaleFactor: 1, idleTime: 10},
			data:           ilsMachinesData{idle: 0, used: 0},
			details:        ilsMachineDetails{state: machineStateIdle, used: stubUsedTime(-60)},
			expectedReason: dontRemoveIdleMachine,
		},
		"IdleTime exceeded and IdleScaleFactor evaluated to non 0 and not reached but IdleCount exceeded": {
			config:         ilsRunnerConfig{idleCount: 10, idleScaleFactor: 1, idleTime: 10},
			data:           ilsMachinesData{idle: 10, used: 100},
			details:        ilsMachineDetails{state: machineStateIdle, used: stubUsedTime(-60)},
			expectedReason: removeIdleReasonTooManyIdleMachines,
		},
		// It makes no sense to have the IdleCountMin at the same or higher level than IdleCount.
		// Preparing such configuration would practically remove the functionality added by IdleScaleFactor
		// and revert the IdleCount behavior to be "static number of idle machines to maintain".
		// As preventing that from happening and adding warnings or even errors for such case would
		// complicate the code, I think we can assume that user should understand how IdleCount, IdleCountMin
		// and IdleScaleFactor work together and that this case doesn't make sense.
		// The following two test cases are added to ensure that scaling still works, even when IdleCount and
		// IdleCountMin are messed up.
		"IdleTime exceeded and IdleCount exceeded and IdleCountMin not fulfilled": {
			config:         ilsRunnerConfig{idleCount: 5, idleCountMin: 10, idleTime: 10},
			data:           ilsMachinesData{idle: 8},
			details:        ilsMachineDetails{state: machineStateIdle, used: stubUsedTime(-60)},
			expectedReason: removeIdleReasonTooManyIdleMachines,
		},
		"IdleTime exceeded and IdleCount exceeded and IdleCountMin not fulfilled " +
			"and IdleScaleFactor evaluated to non 0 and not reached": {
			config:         ilsRunnerConfig{idleCount: 5, idleCountMin: 10, idleScaleFactor: 1, idleTime: 10},
			data:           ilsMachinesData{idle: 8, used: 9},
			details:        ilsMachineDetails{state: machineStateIdle, used: stubUsedTime(-60)},
			expectedReason: removeIdleReasonTooManyIdleMachines,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			result := shouldRemoveIdle(
				ilsNewRunnerConfig(tt.config),
				ilsNewMachinesData(tt.data),
				ilsNewMachineDetails(tt.details),
			)
			assert.Equal(t, tt.expectedReason, result)
		})
	}
}

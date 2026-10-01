//go:build !integration

package machine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type ilsRunnerConfig struct {
	limit              int
	idleCount          int
	idleCountMin       int
	idleScaleFactor    float64
	idleTime           int
	maxGrowthRate      int
	maxBuilds          int
	warmupIdleFraction float64
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
		IdleCount:          c.idleCount,
		IdleCountMin:       c.idleCountMin,
		IdleScaleFactor:    c.idleScaleFactor,
		IdleTime:           c.idleTime,
		MaxGrowthRate:      c.maxGrowthRate,
		MaxBuilds:          c.maxBuilds,
		WarmupIdleFraction: c.warmupIdleFraction,
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
			result := canCreateIdle(ilsNewRunnerConfig(tt.config), ilsNewMachinesData(tt.data), nil)
			assert.Equal(t, tt.expectedCanCreate, result)
		})
	}
}

func TestCanCreateOnDemand(t *testing.T) {
	tests := map[string]struct {
		config            ilsRunnerConfig
		data              ilsMachinesData
		expectedCanCreate bool
	}{
		"MaxMachinesGrowth exceeded": {
			config:            ilsRunnerConfig{maxGrowthRate: 10, idleCount: 1},
			data:              ilsMachinesData{creating: 10},
			expectedCanCreate: false,
		},
		"MaxMachinesGrowth not reached": {
			config:            ilsRunnerConfig{maxGrowthRate: 10, idleCount: 1},
			data:              ilsMachinesData{creating: 1},
			expectedCanCreate: false,
		},
		"limit exceeded": {
			config:            ilsRunnerConfig{limit: 10, idleCount: 1},
			data:              ilsMachinesData{creating: 5, idle: 3, used: 2},
			expectedCanCreate: false,
		},
		"limit not reached": {
			config:            ilsRunnerConfig{limit: 10, idleCount: 10},
			data:              ilsMachinesData{idle: 3, used: 2},
			expectedCanCreate: true,
		},
		"IdleCount is 0, no limit": {
			config:            ilsRunnerConfig{idleCount: 0, idleCountMin: 10, idleScaleFactor: 1, limit: 0},
			data:              ilsMachinesData{idle: 0, used: 10},
			expectedCanCreate: true,
		},
		"IdleCount is 0, limit not reached": {
			config:            ilsRunnerConfig{idleCount: 0, idleCountMin: 10, idleScaleFactor: 1, limit: 10},
			data:              ilsMachinesData{idle: 3, used: 2},
			expectedCanCreate: true,
		},
		"IdleCount is 0, limit is reached": {
			config:            ilsRunnerConfig{idleCount: 0, idleCountMin: 10, idleScaleFactor: 1, limit: 10},
			data:              ilsMachinesData{idle: 0, used: 10},
			expectedCanCreate: false,
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
			result := canCreateOnDemand(ilsNewRunnerConfig(tt.config), ilsNewMachinesData(tt.data), nil)
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
				nil,
				ilsNewMachineDetails(tt.details).info(),
			)
			assert.Equal(t, tt.expectedReason, result)
		})
	}
}

func TestEffectiveIdleTarget(t *testing.T) {
	tests := map[string]struct {
		config   ilsRunnerConfig
		data     ilsMachinesData
		expected int
	}{
		"no scale factor returns IdleCount": {
			config:   ilsRunnerConfig{idleCount: 50},
			data:     ilsMachinesData{used: 100},
			expected: 50,
		},
		"scale factor follows in-use machines": {
			config:   ilsRunnerConfig{idleCount: 50, idleCountMin: 5, idleScaleFactor: 0.5},
			data:     ilsMachinesData{used: 40},
			expected: 20,
		},
		"scale factor clamped to IdleCount": {
			config:   ilsRunnerConfig{idleCount: 50, idleCountMin: 5, idleScaleFactor: 0.9},
			data:     ilsMachinesData{used: 100},
			expected: 50,
		},
		"scale factor clamped to IdleCountMin": {
			config:   ilsRunnerConfig{idleCount: 50, idleCountMin: 5, idleScaleFactor: 0.9},
			data:     ilsMachinesData{used: 2},
			expected: 5,
		},
		"zero IdleCountMin forced to one with scale factor": {
			config:   ilsRunnerConfig{idleCount: 50, idleScaleFactor: 0.9},
			data:     ilsMachinesData{used: 0},
			expected: 1,
		},
		"on-demand mode reports zero despite scale factor": {
			config:   ilsRunnerConfig{idleCount: 0, idleScaleFactor: 0.9},
			data:     ilsMachinesData{used: 5},
			expected: 0,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			config := ilsNewRunnerConfig(test.config)
			data := ilsNewMachinesData(test.data)
			assert.Equal(t, test.expected, effectiveIdleTarget(config, data, nil))
		})
	}
}

func TestEffectiveIdleTargetDuringWarmup(t *testing.T) {
	tests := map[string]struct {
		config   ilsRunnerConfig
		warmup   *warmupLimits
		data     ilsMachinesData
		expected int
	}{
		"warm-up target replaces the scale factor value": {
			config:   ilsRunnerConfig{idleCount: 200, idleCountMin: 16, idleScaleFactor: 0.9, warmupIdleFraction: 0.75},
			warmup:   &warmupLimits{burst: true},
			data:     ilsMachinesData{used: 0},
			expected: 150,
		},
		"warm-up target capped at IdleCount": {
			config:   ilsRunnerConfig{idleCount: 200, idleCountMin: 16, idleScaleFactor: 0.9, warmupIdleFraction: 1.5},
			warmup:   &warmupLimits{burst: true},
			expected: 200,
		},
		"scale factor value wins when higher": {
			config:   ilsRunnerConfig{idleCount: 200, idleCountMin: 16, idleScaleFactor: 0.9, warmupIdleFraction: 0.75},
			warmup:   &warmupLimits{burst: true},
			data:     ilsMachinesData{used: 190},
			expected: 171,
		},
		"disabled when WarmupIdleFraction is 0": {
			config:   ilsRunnerConfig{idleCount: 200, idleCountMin: 16, idleScaleFactor: 0.9},
			warmup:   &warmupLimits{burst: true},
			expected: 16,
		},
		"outside the window the scale factor value applies": {
			config:   ilsRunnerConfig{idleCount: 200, idleCountMin: 16, idleScaleFactor: 0.9, warmupIdleFraction: 0.75},
			warmup:   nil,
			data:     ilsMachinesData{used: 20},
			expected: 18,
		},
		"on-demand mode stays at zero": {
			config:   ilsRunnerConfig{idleCount: 0, warmupIdleFraction: 0.75},
			warmup:   &warmupLimits{burst: true},
			expected: 0,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			config := ilsNewRunnerConfig(test.config)
			data := ilsNewMachinesData(test.data)
			assert.Equal(t, test.expected, effectiveIdleTarget(config, data, test.warmup))
		})
	}
}

func TestEffectiveIdleTargetDuringWarmupFollowsActivePeriod(t *testing.T) {
	config := ilsNewRunnerConfig(ilsRunnerConfig{idleCount: 200, idleScaleFactor: 0.9, warmupIdleFraction: 0.5})
	config.Machine.AutoscalingConfigs = []*common.DockerMachineAutoscaling{{
		Periods:         []string{"* * * * * * *"},
		IdleCount:       40,
		IdleScaleFactor: 0.9,
		IdleCountMin:    2,
	}}
	require.NoError(t, config.Machine.CompilePeriods())

	data := ilsNewMachinesData(ilsMachinesData{})
	assert.Equal(t, 20, effectiveIdleTarget(config, data, &warmupLimits{burst: true}))
	assert.Equal(t, 2, effectiveIdleTarget(config, data, nil))
}

func TestMaxGrowthRateDuringWarmup(t *testing.T) {
	tests := map[string]struct {
		config   ilsRunnerConfig
		warmup   *warmupLimits
		expected int
	}{
		"burst raises the cap to the warm-up target": {
			config:   ilsRunnerConfig{idleCount: 200, maxGrowthRate: 90, warmupIdleFraction: 0.75},
			warmup:   &warmupLimits{burst: true},
			expected: 150,
		},
		"MaxGrowthRate wins when higher than the target": {
			config:   ilsRunnerConfig{idleCount: 200, maxGrowthRate: 300, warmupIdleFraction: 0.75},
			warmup:   &warmupLimits{burst: true},
			expected: 300,
		},
		"after a creation error MaxGrowthRate applies": {
			config:   ilsRunnerConfig{idleCount: 200, maxGrowthRate: 90, warmupIdleFraction: 0.75},
			warmup:   &warmupLimits{burst: false},
			expected: 90,
		},
		"outside the window MaxGrowthRate applies": {
			config:   ilsRunnerConfig{idleCount: 200, maxGrowthRate: 90, warmupIdleFraction: 0.75},
			warmup:   nil,
			expected: 90,
		},
		"unlimited stays unlimited": {
			config:   ilsRunnerConfig{idleCount: 200, maxGrowthRate: 0, warmupIdleFraction: 0.75},
			warmup:   &warmupLimits{burst: true},
			expected: 0,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, maxGrowthRate(ilsNewRunnerConfig(test.config), test.warmup))
		})
	}
}

func TestCanCreateIdleDuringWarmup(t *testing.T) {
	config := ilsNewRunnerConfig(ilsRunnerConfig{idleCount: 200, idleCountMin: 16, idleScaleFactor: 0.9, maxGrowthRate: 90, warmupIdleFraction: 0.75})

	t.Run("burst creates past MaxGrowthRate up to the warm-up target", func(t *testing.T) {
		assert.True(t, canCreateIdle(config, ilsNewMachinesData(ilsMachinesData{creating: 149}), &warmupLimits{burst: true}))
		assert.False(t, canCreateIdle(config, ilsNewMachinesData(ilsMachinesData{creating: 150}), &warmupLimits{burst: true}))
	})

	t.Run("without burst MaxGrowthRate caps creation while the target still applies", func(t *testing.T) {
		assert.True(t, canCreateIdle(config, ilsNewMachinesData(ilsMachinesData{creating: 89}), &warmupLimits{burst: false}))
		assert.False(t, canCreateIdle(config, ilsNewMachinesData(ilsMachinesData{creating: 90}), &warmupLimits{burst: false}))
		assert.False(t, canCreateIdle(config, ilsNewMachinesData(ilsMachinesData{creating: 50, idle: 100}), &warmupLimits{burst: false}))
	})

	t.Run("outside the window IdleCountMin applies", func(t *testing.T) {
		assert.False(t, canCreateIdle(config, ilsNewMachinesData(ilsMachinesData{idle: 16}), nil))
	})
}

func TestShouldRemoveIdleDuringWarmup(t *testing.T) {
	config := ilsNewRunnerConfig(ilsRunnerConfig{idleCount: 200, idleCountMin: 16, idleScaleFactor: 0.9, idleTime: 30, warmupIdleFraction: 0.75})
	data := ilsNewMachinesData(ilsMachinesData{idle: 149})
	details := ilsNewMachineDetails(ilsMachineDetails{state: machineStateIdle, used: time.Now().Add(-time.Minute)}).info()

	assert.Equal(t, dontRemoveIdleMachine, shouldRemoveIdle(config, data, &warmupLimits{}, details))
	assert.Equal(t, removeIdleReasonTooManyIdleMachines, shouldRemoveIdle(config, data, nil, details))
}

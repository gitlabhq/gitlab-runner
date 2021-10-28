package machine

import (
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func idleLimitExceeded(config *common.RunnerConfig, data *machinesData) bool {
	ils := &idleLimitStrategy{
		config: config,
		data:   data,
	}

	return ils.exceeded()
}

type idleLimitStrategy struct {
	config *common.RunnerConfig
	data   *machinesData
}

// exceeded checks if any of the defined filters detected
// exceeding one of the tracked limits.
func (ils *idleLimitStrategy) exceeded() bool {
	return ils.idleMachinesExceeded() ||
		ils.idleMachinesScaleFactorExceeded() ||
		ils.totalMachinesExceeded() ||
		ils.machinesGrowthExceeded()
}

// idleMachinesExceeded checks whether runner reached the defined IdleCount
// which is the maximum number of Idle machines that can exist.
func (ils *idleLimitStrategy) idleMachinesExceeded() bool {
	return ils.data.Available() >= ils.config.Machine.GetIdleCount()
}

// idleMachinesScaleFactorExceeded checks whether runner reached the number
// of machines defined as a factor of in-use ones.
// This behavior is optional and depends on the IdleScaleFactor setting.
// When it's set to 0 then it's ignored.
func (ils *idleLimitStrategy) idleMachinesScaleFactorExceeded() bool {
	idleScaleFactor := ils.config.Machine.GetIdleScaleFactor()
	if idleScaleFactor <= 0 {
		return false
	}

	available := ils.data.Available()
	if available < ils.config.Machine.GetIdleCountMin() {
		return false
	}

	desiredCapacity := int(float64(ils.data.InUse()) * idleScaleFactor)

	return available >= desiredCapacity
}

// totalMachinesExceeded checks whether runner reached the maximum number
// of all machines that can be created. It's defined by the limit setting.
// The standard behavior of "limit=0 means no limit" is respected here.
func (ils *idleLimitStrategy) totalMachinesExceeded() bool {
	if ils.config.Limit <= 0 {
		return false
	}

	return ils.data.Total() >= ils.config.Limit
}

// machinesGrowthExceeded checks whether runner reached the maximum number
// of machines that can be in creation state at one moment.
// This behavior is optional and depends on the MaxGrowthRate setting.
// When it's set to 0 then it's ignored.
func (ils *idleLimitStrategy) machinesGrowthExceeded() bool {
	maxGrowthRate := ils.config.Machine.MaxGrowthRate
	if maxGrowthRate <= 0 {
		return false
	}

	return ils.data.Creating >= maxGrowthRate
}

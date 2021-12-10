package machine

import (
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type removeIdleReason string

const (
	dontRemoveIdleMachine               removeIdleReason = "don't remote"
	removeIdleReasonTooManyBuilds       removeIdleReason = "too many builds"
	removeIdleReasonTooManyMachines     removeIdleReason = "too many machines"
	removeIdleReasonTooManyIdleMachines removeIdleReason = "too many idle machines"
)

func canCreateIdle(config *common.RunnerConfig, data *machinesData) bool {
	ils := &idleLimitStrategy{
		config: config,
		data:   data,
	}

	return ils.canCreate()
}

func shouldRemoveIdle(config *common.RunnerConfig, data *machinesData, details *machineDetails) removeIdleReason {
	ils := &idleLimitStrategy{
		config:  config,
		data:    data,
		details: details,
	}

	return ils.shouldRemove()
}

type idleLimitStrategy struct {
	config  *common.RunnerConfig
	data    *machinesData
	details *machineDetails
}

// canCreate checks if any of the defined filters detected
// exceeding one of the tracked limits.
func (ils *idleLimitStrategy) canCreate() bool {
	exceeded := ils.machinesGrowthExceeded() ||
		ils.totalMachinesExceeded() ||
		ils.composedIdleMachinesExceeded()

	return !exceeded
}

// shouldRemove checks if the machine is in Idle state
// and if it's applicable for removal
func (ils *idleLimitStrategy) shouldRemove() removeIdleReason {
	if ils.details.State != machineStateIdle {
		return dontRemoveIdleMachine
	}

	if ils.machineUsageCountExceeded() {
		return removeIdleReasonTooManyBuilds
	}

	if ils.totalMachinesExceeded() {
		return removeIdleReasonTooManyMachines
	}

	if ils.idleTimeExceeded() && ils.composedIdleMachinesExceeded() {
		return removeIdleReasonTooManyIdleMachines
	}

	return dontRemoveIdleMachine
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

// totalMachinesExceeded checks whether runner reached the maximum number
// of all machines that can be created. It's defined by the limit setting.
// The standard behavior of "limit=0 means no limit" is respected here.
func (ils *idleLimitStrategy) totalMachinesExceeded() bool {
	if ils.config.Limit <= 0 {
		return false
	}

	return ils.data.Total() >= ils.config.Limit
}

// composedIdleMachinesExceeded checks several conditions that can evaluate
// as "number of Idle Machines exceeded".
func (ils *idleLimitStrategy) composedIdleMachinesExceeded() bool {
	return ils.idleMachinesExceeded() ||
		(ils.idleCountMinFulfilled() && ils.idleMachinesScaleFactorExceeded())
}

// idleMachinesExceeded checks whether runner reached the defined IdleCount
// which is the maximum number of Idle machines that can exist.
func (ils *idleLimitStrategy) idleMachinesExceeded() bool {
	return ils.data.Available() >= ils.config.Machine.GetIdleCount()
}

// idleCountMinFulfilled checks if the IdleCountMin setting is fulfilled.
// Should be used to ensure that the minimal number of Idle machines is created.
func (ils *idleLimitStrategy) idleCountMinFulfilled() bool {
	min := ils.config.Machine.GetIdleCountMin()

	// When IdleScaleFactor is in use, there is a risk that with no executed jobs
	// the desired number of Idle machines to maintain will also evaluate to 0.
	// This could cause in removing all Idle machines. In that case Runner would
	// stop asking for new jobs (with IdleCount > 0 Runner doesn't ask for jobs
	// if there is no Idle machines awaiting to be used). And without new jobs using
	// some machines, the IdleScaleFactor would be constantly evaluated to 0.
	// This would lock the Runner in a state where no job can't be started because
	// no machines are in Idle, and no machines are in Idle because no jobs are started.
	//
	// Therefore, in case when IdleScaleFactor is greater than 0 and IdleCountMin
	// was not defined or intentionally set to 0, it will be forced to be at least
	// 1. So that there is at least one Idle machine that can handle a job and allow
	// the IdleScaleFactor to bring more of them later.
	if ils.config.Machine.GetIdleScaleFactor() > 0 && min < 1 {
		min = 1
	}

	return ils.data.Available() >= min
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

	desiredCapacity := int(float64(ils.data.InUse()) * idleScaleFactor)

	return ils.data.Available() >= desiredCapacity
}

// machineUsageCountExceeded checks whether the machine was used more times than
// the defined MaxBuilds setting.
// MaxBuild=0 means that there is no limit how many subsequent jobs the machine
// can handle.
func (ils *idleLimitStrategy) machineUsageCountExceeded() bool {
	if ils.config.Machine.MaxBuilds <= 0 {
		return false
	}

	return ils.details.UsedCount >= ils.config.Machine.MaxBuilds
}

// idleTimeExceeded checks whether machine's last usage happened
// more than IdleTime ago.
func (ils *idleLimitStrategy) idleTimeExceeded() bool {
	return time.Since(ils.details.Used) > time.Second*time.Duration(ils.config.Machine.GetIdleTime())
}

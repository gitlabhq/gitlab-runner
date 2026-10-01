package machine

import (
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type removeIdleReason string

const (
	dontRemoveIdleMachine               removeIdleReason = "don't remove"
	removeIdleReasonTooManyBuilds       removeIdleReason = "too many builds"
	removeIdleReasonTooManyMachines     removeIdleReason = "too many machines"
	removeIdleReasonTooManyIdleMachines removeIdleReason = "too many idle machines"
)

// warmupLimits is nil when the runner is not in warm-up.
type warmupLimits struct {
	// burst raises the in-flight creation cap to the warm-up idle target until
	// the first creation error.
	burst bool
}

func canCreateIdle(config *common.RunnerConfig, data *machinesData, warmup *warmupLimits) bool {
	return canCreate(config, data, warmup, false)
}

func canCreateOnDemand(config *common.RunnerConfig, data *machinesData, warmup *warmupLimits) bool {
	return canCreate(config, data, warmup, true)
}

func canCreate(config *common.RunnerConfig, data *machinesData, warmup *warmupLimits, onDemand bool) bool {
	ils := &idleLimitStrategy{
		config: config,
		data:   data,
		warmup: warmup,
	}

	return ils.canCreate(onDemand)
}

func shouldRemoveIdle(
	config *common.RunnerConfig,
	data *machinesData,
	warmup *warmupLimits,
	details machineInfo,
) removeIdleReason {
	ils := &idleLimitStrategy{
		config:  config,
		data:    data,
		warmup:  warmup,
		details: details,
	}

	return ils.shouldRemove()
}

func effectiveIdleTarget(config *common.RunnerConfig, data *machinesData, warmup *warmupLimits) int {
	ils := &idleLimitStrategy{
		config: config,
		data:   data,
		warmup: warmup,
	}

	return ils.effectiveIdleTarget()
}

// maxGrowthRate is the in-flight creation cap. 0 means unlimited.
func maxGrowthRate(config *common.RunnerConfig, warmup *warmupLimits) int {
	rate := config.Machine.MaxGrowthRate
	if rate <= 0 {
		return 0
	}
	if warmup != nil && warmup.burst {
		rate = max(rate, config.Machine.GetWarmupIdleTarget())
	}
	return rate
}

type idleLimitStrategy struct {
	config  *common.RunnerConfig
	data    *machinesData
	warmup  *warmupLimits
	details machineInfo
}

// canCreate checks if any of the defined filters detected
// exceeding one of the tracked limits.
//
// onDemand indicates that the caller wants to create machines as
// they are needed by jobs, as opposed to the pre-scaling phase where idle
// machines are allocated first.
func (ils *idleLimitStrategy) canCreate(onDemand bool) bool {
	if ils.machinesGrowthExceeded() || ils.totalMachinesExceeded() {
		return false
	}

	if onDemand && ils.config.Machine.GetIdleCount() <= 0 {
		return true
	}

	return !ils.composedIdleMachinesExceeded()
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
	maxGrowthRate := maxGrowthRate(ils.config, ils.warmup)
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

func (ils *idleLimitStrategy) composedIdleMachinesExceeded() bool {
	return ils.data.Available() >= ils.effectiveIdleTarget()
}

// effectiveIdleTarget is the number of available machines the creation loop
// aims for.
func (ils *idleLimitStrategy) effectiveIdleTarget() int {
	target := ils.scaledIdleTarget()
	if ils.warmup != nil {
		target = max(target, ils.config.Machine.GetWarmupIdleTarget())
	}
	return target
}

func (ils *idleLimitStrategy) scaledIdleTarget() int {
	idleCount := ils.config.Machine.GetIdleCount()
	factor := ils.config.Machine.GetIdleScaleFactor()
	if factor <= 0 {
		return idleCount
	}

	// With no jobs running the target would evaluate to 0 and all Idle
	// machines would be removed, but with IdleCount > 0 the Runner only asks
	// for jobs when an Idle machine is available. Keep at least one.
	idleCountMin := max(ils.config.Machine.GetIdleCountMin(), 1)

	target := int(float64(ils.data.InUse()) * factor)
	return min(max(target, idleCountMin), idleCount)
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

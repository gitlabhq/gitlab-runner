package machine

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/labkit/v2/fields"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// warmupState is one runner's warm-up.
//
// Warm-up ends only when WarmupDuration has passed, not when the target is
// reached. During a rollout the old pods keep taking jobs for some minutes
// after this pod is Ready, and ending warm-up early would let IdleTime remove
// the pool just built before they are gone.
//
// ready is never cleared once set.
type warmupState struct {
	startedAt  time.Time
	deadline   time.Time
	burst      bool
	ready      bool
	ended      bool
	timedOut   bool
	readyAfter time.Duration
	booted     int

	lastAcquire     time.Time
	lastProgressLog time.Time
}

const warmupProgressLogInterval = time.Minute

func (w *warmupState) active(now time.Time) bool {
	return now.Before(w.deadline)
}

func (w *warmupState) limits() *warmupLimits {
	return &warmupLimits{burst: w.burst}
}

func (w *warmupState) end(config *common.RunnerConfig, now time.Time) {
	if w.ended {
		return
	}
	w.ended = true
	logger := warmupLogger(config).WithFields(logrus.Fields{
		"booted":         w.booted,
		"ready_machines": warmupReadyMachines(config),
		"idle_target":    config.Machine.GetWarmupIdleTarget(),
		fields.DurationS: now.Sub(w.startedAt).Seconds(),
	})
	if w.ready {
		logger.Info("Warm-up ended")
		return
	}
	w.ready = true
	w.timedOut = true
	logger.Warning("Warm-up ended before the pool reached the ready threshold")
}

// Capped at Limit: a pool cannot grow past it.
func warmupReadyMachines(config *common.RunnerConfig) int {
	n := config.Machine.GetWarmupReadyMachines()
	if config.Limit > 0 {
		n = min(n, config.Limit)
	}
	return n
}

func warmupLogger(config *common.RunnerConfig) *logrus.Entry {
	return logrus.WithFields(logrus.Fields{
		"phase":       "warmup",
		"runner":      config.ShortDescription(),
		"runner_name": config.Name,
		"system_id":   config.GetSystemID(),
	})
}

func (m *machineProvider) withWarmup(config *common.RunnerConfig, fn func(*warmupState)) {
	if config.Machine == nil || !config.Machine.WarmupEnabled() {
		return
	}

	m.warmupLock.Lock()
	defer m.warmupLock.Unlock()

	if state, ok := m.warmups[config.ShortDescription()]; ok {
		fn(state)
	}
}

func (m *machineProvider) warmupAcquire(config *common.RunnerConfig, now time.Time) *warmupLimits {
	if !config.Machine.WarmupEnabled() {
		return nil
	}

	m.warmupLock.Lock()
	defer m.warmupLock.Unlock()

	key := config.ShortDescription()
	state, ok := m.warmups[key]
	if !ok {
		// Warm-up is a startup condition. A runner whose first Acquire comes
		// later, or whose pruned entry comes back, does not start one.
		if now.Sub(m.startedAt) >= config.Machine.GetWarmupDuration() {
			return nil
		}
		state = &warmupState{
			startedAt:       now,
			deadline:        now.Add(config.Machine.GetWarmupDuration()),
			lastProgressLog: now,
			burst:           true,
		}
		m.warmups[key] = state

		idleTarget := config.Machine.GetWarmupIdleTarget()
		logger := warmupLogger(config).WithFields(logrus.Fields{
			"idle_target":       idleTarget,
			"ready_machines":    warmupReadyMachines(config),
			"max_growth_rate":   maxGrowthRate(config, state.limits()),
			"warmup_duration_s": config.Machine.GetWarmupDuration().Seconds(),
		})
		logger.Info("Warm-up started")
		if config.Limit > 0 && config.Limit < idleTarget {
			logger.WithField("limit", config.Limit).
				Warning("Warm-up idle target is above the runner limit; the ready threshold is capped at the limit")
		}
	}
	state.lastAcquire = now
	m.pruneWarmups(now)

	if state.active(now) {
		return state.limits()
	}

	state.end(config, now)
	return nil
}

func (m *machineProvider) pruneWarmups(now time.Time) {
	for key, state := range m.warmups {
		if state.ended && now.Sub(state.lastAcquire) > autoscalingLimitsTTL {
			delete(m.warmups, key)
		}
	}
}

func (m *machineProvider) warmupLimitsFor(config *common.RunnerConfig, now time.Time) (limits *warmupLimits) {
	m.withWarmup(config, func(state *warmupState) {
		if state.active(now) {
			limits = state.limits()
		}
	})
	return limits
}

// warmupGauges is what Collect reports for a runner. It is computed from the
// state at scrape time, since the state can change between Acquires.
type warmupGauges struct {
	active   bool
	timedOut bool
	reached  bool
	readyS   float64
}

func (m *machineProvider) warmupGaugesFor(runner string, now time.Time) warmupGauges {
	m.warmupLock.Lock()
	defer m.warmupLock.Unlock()

	state, ok := m.warmups[runner]
	if !ok {
		return warmupGauges{}
	}
	active := state.active(now)
	return warmupGauges{
		active:   active,
		timedOut: state.timedOut || (!active && !state.ready),
		reached:  state.ready && !state.timedOut,
		readyS:   state.readyAfter.Seconds(),
	}
}

func (m *machineProvider) observeWarmup(config *common.RunnerConfig, data *machinesData, now time.Time) {
	m.withWarmup(config, func(state *warmupState) {
		state.booted = data.Booted()
		idleTarget := config.Machine.GetWarmupIdleTarget()
		readyMachines := warmupReadyMachines(config)

		if !state.ready && state.booted >= readyMachines {
			state.ready = true
			state.readyAfter = now.Sub(state.startedAt)
			warmupLogger(config).WithFields(logrus.Fields{
				"booted":         state.booted,
				"ready_machines": readyMachines,
				"idle_target":    idleTarget,
				fields.DurationS: state.readyAfter.Seconds(),
			}).Info("Warm-up ready")
		}

		if state.ended || now.Sub(state.lastProgressLog) < warmupProgressLogInterval {
			return
		}
		state.lastProgressLog = now
		warmupLogger(config).WithFields(logrus.Fields{
			"booted":         state.booted,
			"idle":           data.Idle,
			"used":           data.Used,
			"creating":       data.Creating,
			"ready_machines": readyMachines,
			"idle_target":    idleTarget,
			"ready":          state.ready,
			"burst":          state.burst,
			"elapsed_s":      now.Sub(state.startedAt).Round(time.Second).Seconds(),
			"remaining_s":    max(state.deadline.Sub(now), 0).Round(time.Second).Seconds(),
		}).Info("Warm-up in progress")
	})
}

// A failed create is the only signal the provider gets of a stockout or rate
// limit at the cloud provider.
func (m *machineProvider) warmupCreationFailed(config *common.RunnerConfig, err error) {
	if config.Machine.MaxGrowthRate <= 0 {
		return
	}
	m.withWarmup(config, func(state *warmupState) {
		if !state.burst {
			return
		}
		state.burst = false
		warmupLogger(config).WithField(fields.ErrorMessage, err.Error()).
			WithField("max_growth_rate", config.Machine.MaxGrowthRate).
			Warning("Machine creation failed during warm-up, dropping the creation burst")
	})
}

func (m *machineProvider) WarmupReady(runners []*common.RunnerConfig) (bool, string) {
	now := time.Now()

	m.warmupLock.Lock()
	defer m.warmupLock.Unlock()

	var pending []string
	for _, runner := range runners {
		if runner.Machine == nil || !runner.Machine.WarmupEnabled() {
			continue
		}

		state, ok := m.warmups[runner.ShortDescription()]
		if !ok {
			// Never reached Acquire (no token, for example): the clock is the provider's.
			if now.Sub(m.startedAt) < runner.Machine.GetWarmupDuration() {
				pending = append(pending, fmt.Sprintf("not started (%s)", runner.ShortDescription()))
			}
			continue
		}
		if !state.active(now) {
			state.end(runner, now)
		}
		if state.ready {
			continue
		}
		pending = append(pending, fmt.Sprintf("booted %d/%d (%s)",
			state.booted, warmupReadyMachines(runner), runner.ShortDescription()))
	}

	if len(pending) == 0 {
		return true, ""
	}
	return false, "warm-up: " + strings.Join(pending, ", ")
}

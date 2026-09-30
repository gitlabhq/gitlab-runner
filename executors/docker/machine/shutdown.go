package machine

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// The provider is constructed before the config is loaded; Init runs before
// collector registration and before any machine is created.
func (m *machineProvider) Init(config *common.Config) {
	logrus.WithField("executor", m.name).Debug("Initializing docker+machine executor provider")

	if config != nil {
		m.machineMetrics = newMachineMetrics(m.name, config.Machine)
	}
}

// minRecommendedDrainTimeout is the minimum recommended shutdown_timeout when drain is enabled.
// With default concurrency of 3 and typical machine removal time, this allows draining ~15 machines.
const minRecommendedDrainTimeout = 5 * time.Minute

const drainReason = "shutdown drain"

// shutdownDrain removes the machine pool during shutdown: idle machines in
// passes from BeginShutdown on, everything that is left once Shutdown is called.
type shutdownDrain struct {
	config       *common.DockerMachineShutdownDrain
	logger       *logrus.Entry
	pollInterval time.Duration

	cancel     context.CancelFunc
	finish     chan struct{}
	finishOnce sync.Once
	done       chan struct{}

	mu      sync.Mutex
	removed int
	// Not retried by idle passes; the final pass tries once more.
	failed map[string]struct{}
}

func (d *shutdownDrain) recordRemoved(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.removed++
	delete(d.failed, name)
}

func (d *shutdownDrain) recordFailed(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.failed[name] = struct{}{}
}

func (d *shutdownDrain) hasFailed(name string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.failed[name]
	return ok
}

func (d *shutdownDrain) finishWithin(ctx context.Context) {
	stop := context.AfterFunc(ctx, d.cancel)
	defer stop()

	d.finishOnce.Do(func() { close(d.finish) })
	<-d.done
}

func (m *machineProvider) BeginShutdown(config *common.Config) {
	drainConfig := shutdownDrainConfig(config)
	if drainConfig == nil || !drainConfig.Early {
		return
	}

	m.shuttingDown.Store(true)
	logger := logrus.WithField("executor", m.name)
	logger.Info("Runner is shutting down, draining idle machines while jobs finish")
	m.ensureDrain(drainConfig, logger, false)
}

func (m *machineProvider) Shutdown(ctx context.Context, config *common.Config) {
	m.shuttingDown.Store(true)

	logger := logrus.WithField("executor", m.name)
	logger.Info("Shutting down docker+machine executor provider")

	m.drainLock.Lock()
	d := m.drain
	m.drainLock.Unlock()

	msg := "Finishing machine pool drain"
	if d == nil {
		drainConfig := shutdownDrainConfig(config)
		switch {
		case config == nil || config.Machine == nil || config.Machine.ShutdownDrain == nil:
			logger.Info("No shutdown drain config, skipping machine pool drain")
			return
		case drainConfig == nil:
			logger.Info("Shutdown drain is disabled, skipping machine pool drain")
			return
		}

		d = m.ensureDrain(drainConfig, logger, true)
		msg = "Starting machine pool drain"
	}

	shutdownTimeout := common.DefaultShutdownTimeout
	if config != nil {
		shutdownTimeout = config.GetShutdownTimeout()
	}
	logger.WithField("shutdown_timeout", shutdownTimeout).Info(msg)

	if shutdownTimeout < minRecommendedDrainTimeout {
		logger.WithFields(logrus.Fields{
			"current_timeout":     shutdownTimeout,
			"recommended_minimum": minRecommendedDrainTimeout,
		}).Warn("shutdown_timeout may be too short for draining machines; consider increasing it")
	}

	d.finishWithin(ctx)
}

func shutdownDrainConfig(config *common.Config) *common.DockerMachineShutdownDrain {
	if config == nil || config.Machine == nil || config.Machine.ShutdownDrain == nil ||
		!config.Machine.ShutdownDrain.IsEnabled() {
		return nil
	}

	return config.Machine.ShutdownDrain
}

func (m *machineProvider) ensureDrain(
	config *common.DockerMachineShutdownDrain,
	logger *logrus.Entry,
	final bool,
) *shutdownDrain {
	m.drainLock.Lock()
	defer m.drainLock.Unlock()

	if m.drain != nil {
		return m.drain
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.drain = &shutdownDrain{
		config:       config,
		logger:       logger,
		pollInterval: shutdownDrainPollInterval,
		cancel:       cancel,
		finish:       make(chan struct{}),
		done:         make(chan struct{}),
		failed:       map[string]struct{}{},
	}
	if final {
		m.drain.finishOnce.Do(func() { close(m.drain.finish) })
	}
	go m.runDrain(ctx, m.drain)

	return m.drain
}

func (m *machineProvider) runDrain(ctx context.Context, d *shutdownDrain) {
	defer close(d.done)
	defer m.logDrainResults(d)

	for ctx.Err() == nil {
		select {
		case <-d.finish:
			m.drainPass(ctx, d, m.collectAllMachines(), false)
			return
		default:
		}

		m.drainPass(ctx, d, m.collectIdleMachines(d), true)

		poll := time.NewTimer(d.pollInterval)
		select {
		case <-ctx.Done():
		case <-d.finish:
		case <-poll.C:
		}
		poll.Stop()
	}
}

func (m *machineProvider) drainPass(ctx context.Context, d *shutdownDrain, machines []*machineDetails, idleOnly bool) {
	if idleOnly {
		if len(machines) == 0 {
			return
		}
		d.logger.WithField("count", len(machines)).Info("Draining idle machines")
	} else {
		d.logger.WithField("count", len(machines)).Info("Draining remaining machine pool")
	}

	sem := make(chan struct{}, d.config.GetConcurrency())
	var wg sync.WaitGroup
	defer wg.Wait()

	for _, details := range machines {
		select {
		case <-ctx.Done():
			d.logger.Warn("Drain operation cancelled or timed out")
			return
		case sem <- struct{}{}:
			wg.Go(func() {
				defer func() { <-sem }()
				m.drainMachine(ctx, d, details, idleOnly)
			})
		}
	}
}

func (m *machineProvider) collectAllMachines() []*machineDetails {
	return m.collectMachines(func(*machineDetails) bool { return true })
}

func (m *machineProvider) collectIdleMachines(d *shutdownDrain) []*machineDetails {
	return m.collectMachines(func(details *machineDetails) bool {
		return isClaimable(details) && !d.hasFailed(details.Name)
	})
}

func (m *machineProvider) collectMachines(match func(*machineDetails) bool) []*machineDetails {
	m.lock.RLock()
	defer m.lock.RUnlock()

	machines := make([]*machineDetails, 0, len(m.details))
	for _, details := range m.details {
		details.Lock()
		ok := match(details)
		details.Unlock()

		if ok {
			machines = append(machines, details)
		}
	}

	return machines
}

func isClaimable(details *machineDetails) bool {
	return details.State == machineStateIdle && !details.pendingUse
}

// claimIdle fails when a late Acquire, a Release with MaxBuilds reached, or
// createAndAcquireMachine got to the machine since the pass collected it.
func claimIdle(details *machineDetails) bool {
	details.Lock()
	defer details.Unlock()

	if !isClaimable(details) {
		return false
	}
	details.State = machineStateRemoving
	details.Reason = drainReason
	return true
}

func (m *machineProvider) drainMachine(ctx context.Context, d *shutdownDrain, details *machineDetails, idleOnly bool) {
	logger := d.logger.WithField("machine", details.Name)

	if idleOnly {
		if !claimIdle(details) {
			return
		}
	} else if !m.claimForFinalDrain(ctx, d, details, logger) {
		return
	}

	maxRetries := d.config.GetMaxRetries()
	for attempt := range maxRetries + 1 {
		err := m.removeMachineForDrain(ctx, details)
		if err == nil {
			m.forgetHeartbeat(details.Name)

			m.lock.Lock()
			delete(m.details, details.Name)
			m.lock.Unlock()

			logger.Info("Machine drained successfully")
			m.totalActions.WithLabelValues(actionLabels("removed", details.targets)...).Inc()
			d.recordRemoved(details.Name)
			return
		}

		logger.WithError(err).WithField("attempt", attempt+1).Warn("Failed to drain machine")

		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				logger.Warn("Machine drain cancelled")
				d.recordFailed(details.Name)
				return
			case <-time.After(d.config.GetRetryBackoff() * time.Duration(attempt+1)):
			}
		}
	}

	logger.Error("Failed to drain machine after all retries")
	d.recordFailed(details.Name)
}

func (m *machineProvider) claimForFinalDrain(
	ctx context.Context,
	d *shutdownDrain,
	details *machineDetails,
	logger *logrus.Entry,
) bool {
	if !d.hasFailed(details.Name) && !m.waitForDrainableState(ctx, details, logger) {
		d.recordFailed(details.Name)
		return false
	}

	m.lock.RLock()
	_, stillTracked := m.details[details.Name]
	m.lock.RUnlock()
	if !stillTracked {
		details.Lock()
		gaveUp := details.removalGaveUp
		details.Unlock()
		if gaveUp {
			logger.Warn("In-flight removal gave up; cloud VM may still exist")
			d.recordFailed(details.Name)
			return false
		}
		logger.Info("Machine drained by in-flight operation")
		d.recordRemoved(details.Name)
		return false
	}

	details.Lock()
	details.State = machineStateRemoving
	details.Reason = drainReason
	details.Unlock()
	return true
}

// waitForDrainableState blocks until details is in a state drain can
// safely act on (not Creating, not Removing), or until ctx expires. It
// re-reads details.inFlight after each wake so a create that fails and
// then starts a remove (replacing the channel) is followed through.
// Returns false on timeout, which drain counts as a failed machine.
func (m *machineProvider) waitForDrainableState(
	ctx context.Context,
	details *machineDetails,
	logger *logrus.Entry,
) bool {
	isTransient := func(s machineState) bool {
		return s == machineStateCreating || s == machineStateRemoving
	}

	logged := false
	var waited chan struct{} // the channel we last blocked on
	for {
		m.lock.RLock()
		_, stillTracked := m.details[details.Name]
		m.lock.RUnlock()
		if !stillTracked {
			return true
		}

		details.Lock()
		state := details.State
		done := details.inFlight
		details.Unlock()

		if !isTransient(state) {
			return true
		}

		// Channel closed but the entry is still transient and nothing
		// replaced it. Re-selecting would busy-spin, since a closed
		// channel is always ready, so fall through to drain.
		if done != nil && done == waited {
			logger.WithField("state", state).
				Warn("In-flight operation signalled completion but left machine in a transient state, proceeding with drain")
			return true
		}

		if !logged {
			logger.WithField("state", state).
				Info("Waiting for in-flight operation to settle before draining machine")
			logged = true
		}

		// No completion channel (recovered from disk, or set by an older
		// code path). Fall through rather than block forever.
		if done == nil {
			logger.WithField("state", state).
				Warn("Transient state without completion signal; proceeding with drain")
			return true
		}

		select {
		case <-done:
			waited = done
		case <-ctx.Done():
			logger.WithField("state", state).
				Warn("In-flight operation did not settle before shutdown timeout; cloud VM may still exist")
			return false
		}
	}
}

func (m *machineProvider) removeMachineForDrain(ctx context.Context, details *machineDetails) error {
	existCtx, existCancel := context.WithTimeout(ctx, machineExistCommandTimeout)
	defer existCancel()

	if !m.machine.Exist(existCtx, details.Name) {
		return nil
	}

	removeCtx, removeCancel := context.WithTimeout(ctx, machineRemoveCommandTimeout)
	defer removeCancel()

	return m.machine.ForceRemove(removeCtx, details.Name)
}

func (m *machineProvider) logDrainResults(d *shutdownDrain) {
	m.lock.RLock()
	remaining := len(m.details)
	m.lock.RUnlock()

	d.mu.Lock()
	defer d.mu.Unlock()

	d.logger.WithFields(logrus.Fields{
		"success":   d.removed,
		"failed":    len(d.failed),
		"remaining": remaining,
	}).Info("Machine pool drain completed")
}

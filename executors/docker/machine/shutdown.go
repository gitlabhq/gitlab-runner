package machine

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func (m *machineProvider) Init() {
	logrus.WithField("executor", m.name).Debug("Initializing docker+machine executor provider")
}

// minRecommendedDrainTimeout is the minimum recommended shutdown_timeout when drain is enabled.
// With default concurrency of 3 and typical machine removal time, this allows draining ~15 machines.
const minRecommendedDrainTimeout = 5 * time.Minute

func (m *machineProvider) Shutdown(ctx context.Context, config *common.Config) {
	logger := logrus.WithField("executor", m.name)
	logger.Info("Shutting down docker+machine executor provider")

	if config == nil || config.Machine == nil || config.Machine.ShutdownDrain == nil {
		logger.Info("No shutdown drain config, skipping machine pool drain")
		return
	}

	drainConfig := config.Machine.ShutdownDrain

	if !drainConfig.IsEnabled() {
		logger.Info("Shutdown drain is disabled, skipping machine pool drain")
		return
	}

	shutdownTimeout := config.GetShutdownTimeout()
	logger.WithField("shutdown_timeout", shutdownTimeout).Info("Starting machine pool drain")

	if shutdownTimeout < minRecommendedDrainTimeout {
		logger.WithFields(logrus.Fields{
			"current_timeout":     shutdownTimeout,
			"recommended_minimum": minRecommendedDrainTimeout,
		}).Warn("shutdown_timeout may be too short for draining machines; consider increasing it")
	}

	m.drainMachinePool(ctx, drainConfig, logger)
}

func (m *machineProvider) drainMachinePool(ctx context.Context, config *common.DockerMachineShutdownDrain, logger *logrus.Entry) {
	machines := m.collectAllMachines()

	if len(machines) == 0 {
		logger.Info("No machines to drain")
		return
	}

	logger.WithField("count", len(machines)).Info("Draining machine pool")

	sem := make(chan struct{}, config.GetConcurrency())
	var wg sync.WaitGroup

	var (
		successCount int
		failCount    int
		mu           sync.Mutex
	)

	defer func() {
		wg.Wait()
		m.logDrainResults(logger, successCount, failCount, len(machines))
	}()

	for _, details := range machines {
		select {
		case <-ctx.Done():
			logger.Warn("Drain operation cancelled or timed out")
			return
		case sem <- struct{}{}:
			wg.Go(func() {
				defer func() { <-sem }()

				success := m.drainMachineWithRetry(ctx, details, config, logger)

				mu.Lock()
				if success {
					successCount++
				} else {
					failCount++
				}
				mu.Unlock()
			})
		}
	}
}

func (m *machineProvider) collectAllMachines() []*machineDetails {
	m.lock.RLock()
	defer m.lock.RUnlock()

	machines := make([]*machineDetails, 0, len(m.details))
	for _, details := range m.details {
		machines = append(machines, details)
	}

	return machines
}

func (m *machineProvider) drainMachineWithRetry(
	ctx context.Context,
	details *machineDetails,
	config *common.DockerMachineShutdownDrain,
	logger *logrus.Entry,
) bool {
	machineLogger := logger.WithField("machine", details.Name)
	maxRetries := config.GetMaxRetries()

	// Wait for any in-flight create/remove to finish first. Removing
	// against incomplete Driver state leaks the cloud VM.
	if !m.waitForDrainableState(ctx, details, machineLogger) {
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
			machineLogger.Warn("In-flight removal gave up; cloud VM may still exist")
			return false
		}
		machineLogger.Info("Machine drained by in-flight operation")
		return true
	}

	details.Lock()
	details.State = machineStateRemoving
	details.Reason = "shutdown drain"
	details.Unlock()

	for attempt := range maxRetries + 1 {
		err := m.removeMachineForDrain(ctx, details)
		if err == nil {
			m.lock.Lock()
			delete(m.details, details.Name)
			m.lock.Unlock()

			machineLogger.Info("Machine drained successfully")
			m.totalActions.WithLabelValues(actionLabels("removed", details.targets)...).Inc()
			return true
		}

		machineLogger.WithError(err).WithField("attempt", attempt+1).Warn("Failed to drain machine")

		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				machineLogger.Warn("Machine drain cancelled")
				return false
			case <-time.After(config.GetRetryBackoff() * time.Duration(attempt+1)):
			}
		}
	}

	machineLogger.Error("Failed to drain machine after all retries")
	return false
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

func (m *machineProvider) logDrainResults(logger *logrus.Entry, success, failed, total int) {
	logger.WithFields(logrus.Fields{
		"success": success,
		"failed":  failed,
		"total":   total,
	}).Info("Machine pool drain completed")
}

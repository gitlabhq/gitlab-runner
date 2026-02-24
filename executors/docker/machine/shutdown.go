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

	details.Lock()
	if details.State == machineStateRemoving {
		details.Unlock()
		machineLogger.Debug("Machine already being removed, skipping")
		return true
	}
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
			m.totalActions.WithLabelValues("removed").Inc()
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

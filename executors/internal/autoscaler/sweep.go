package autoscaler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/fleeting/fleeting/connector"
	"gitlab.com/gitlab-org/fleeting/taskscaler"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var _ common.RuntimeEnvironmentSweeper = &provider{}

// expiredReleaseTimeout bounds the release of one expired environment, so a
// hung instance does not use up the whole sweep and block the slots after
// it. It is a variable so tests can shorten it.
var expiredReleaseTimeout = time.Minute

// SweepRuntimeEnvironments expires this runner's suspended slots more than
// common.RuntimeEnvironmentSweepGrace past the expiry stamped when they were
// acquired, removes their workload from running instances and frees them.
// Taskscaler deletes stopped instances whose slots have all expired,
// without resuming them; expired slots on a stopped instance that still
// holds live slots wait for a resume to start it. A slot whose release
// fails stays expired and is retried on the next sweep.
func (p *provider) SweepRuntimeEnvironments(ctx context.Context, config *common.RunnerConfig, now time.Time) error {
	if config.Autoscaler == nil || !config.Autoscaler.StateStorage.Enabled {
		// suspension requires state storage, so there is nothing to sweep
		return nil
	}

	logger := config.Log()

	// Initialize the taskscaler, so that a restarted manager restores and
	// sweeps its suspended slots before any job arrives.
	ts, err := p.scalerFor(config)
	if err != nil {
		// expiring slots doesn't need the policy, so sweep with the
		// taskscaler that is already running
		if ts = p.getRunnerTaskscaler(config); ts == nil {
			return err
		}
		logger.WithError(err).Warningln("Failed to apply the autoscaler policy; sweeping anyway")
	}

	return p.sweepExpired(ctx, config, ts, now)
}

// sweepExpired releases and frees the expired slots in expiry order. It
// stops once ctx is done, and after one release on an instance times out it
// skips that instance's other slots for the rest of the sweep.
func (p *provider) sweepExpired(ctx context.Context, config *common.RunnerConfig, ts taskscaler.Taskscaler, now time.Time) error {
	logger := config.Log()

	var errs []error
	// Instances whose release timed out in this sweep. Their other slots
	// wait for the next sweep, so one hung instance does not use up the
	// sweep for the slots on other instances.
	timedOut := make(map[string]bool)
	for _, slot := range ts.ExpireSuspended(now, common.RuntimeEnvironmentSweepGrace) {
		if ctx.Err() != nil {
			errs = append(errs, fmt.Errorf("sweep ended before every expired environment was released: %w", ctx.Err()))
			break
		}

		log := logger.WithFields(logrus.Fields{
			"key":         slot.Key,
			"instance-id": slot.InstanceID,
			"expires-at":  slot.ExpiresAt,
		})

		if slot.InstanceStopped {
			log.Debugln("Expired suspended environment is on a stopped instance; waiting")
			continue
		}
		if timedOut[slot.InstanceID] {
			log.Debugln("Expired suspended environment's instance timed out in this sweep; waiting")
			continue
		}

		if err := p.sweepSlot(ctx, config, ts, slot, log); err != nil {
			if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
				timedOut[slot.InstanceID] = true
			}
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// sweepSlot releases and frees one expired slot on a running instance. It
// returns nil when the slot is freed or its instance stopped (the slot waits
// for a later sweep), and the release or free error otherwise.
func (p *provider) sweepSlot(ctx context.Context, config *common.RunnerConfig, ts taskscaler.Taskscaler, slot taskscaler.ExpiredSlot, log logrus.FieldLogger) error {
	releaseCtx, cancel := context.WithTimeout(ctx, expiredReleaseTimeout)
	err := p.releaseExpired(releaseCtx, config, ts, slot.Key)
	cancel()
	switch {
	case errors.Is(err, taskscaler.ErrInstanceStopped):
		log.Debugln("Expired suspended environment's instance stopped; waiting")
		return nil
	case errors.Is(err, ErrEnvironmentNotFound):
		log.Warningln("Expired suspended environment not found on instance; freeing it")
	case err != nil:
		return fmt.Errorf("releasing expired environment %q: %w", slot.Key, err)
	}

	if err := ts.FreeExpired(slot.Key); err != nil {
		return fmt.Errorf("freeing expired environment %q: %w", slot.Key, err)
	}

	log.Infoln("Released expired suspended environment")
	return nil
}

// releaseExpired removes an expired environment's workload from its
// instance. Executors without a releaser have no workload to remove. Docker
// refuses to suspend under vm_isolation, so the vm_isolation branch is
// reached only when vm_isolation is turned on (config reload or restart)
// while docker environments are suspended. That is unsupported, and it
// leaves the stopped containers, networks and volumes on the host.
func (p *provider) releaseExpired(ctx context.Context, config *common.RunnerConfig, ts taskscaler.Taskscaler, key string) error {
	releaser, ok := p.ExecutorProvider.(EnvironmentReleaser)
	if !ok || config.Autoscaler.VMIsolation.Enabled {
		return nil
	}

	info, err := ts.ExpiredConnectInfo(ctx, key)
	if err != nil {
		return fmt.Errorf("getting instance connect info: %w", err)
	}

	dialer, err := p.dialInstance(ctx, info, connector.DialOptions{
		UseExternalAddr: config.Autoscaler.ConnectorConfig.UseExternalAddr,
	})
	if err != nil {
		return fmt.Errorf("dialing instance: %w", err)
	}

	conn := &client{client: dialer}
	defer func() { _ = conn.Close() }()

	return releaser.ReleaseEnvironment(ctx, config, conn, key)
}

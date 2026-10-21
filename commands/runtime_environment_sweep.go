package commands

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

const (
	// runtimeEnvironmentSweepInterval is how often the manager looks for
	// suspended environments past their expiry. It is small relative to the
	// TTL, so an expired environment is removed soon after the grace period.
	runtimeEnvironmentSweepInterval = 10 * time.Minute

	// runtimeEnvironmentSweepTimeout bounds a single runner's sweep, so one
	// unresponsive backend cannot stall cleanup for the other runners.
	runtimeEnvironmentSweepTimeout = 5 * time.Minute
)

// runtimeEnvironmentSweepLoop removes each runner's suspended environments
// past the expiry stamped on them when they were created. It is the safety net for environments the orchestrator never
// releases. Every manager of a registration runs it; providers must tolerate
// another manager removing the same environment concurrently.
type runtimeEnvironmentSweepLoop struct {
	providers executors.Providers
	config    func() *common.Config
	now       func() time.Time
	log       logrus.FieldLogger
}

// run sweeps once immediately and then every interval until ctx is done.
func (l *runtimeEnvironmentSweepLoop) run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		l.sweep(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (l *runtimeEnvironmentSweepLoop) sweep(ctx context.Context) {
	cfg := l.config()
	if cfg == nil {
		return
	}

	for _, runner := range cfg.Runners {
		if ctx.Err() != nil {
			return
		}
		l.sweepRunner(ctx, runner)
	}
}

func (l *runtimeEnvironmentSweepLoop) sweepRunner(ctx context.Context, runner *common.RunnerConfig) {
	// Gated on the runner-level flag: sweeping needs extra permissions on the
	// executor's backend, which runners that never suspend don't have.
	if !runner.IsFeatureFlagOn(featureflags.SuspendableEnvironments) {
		return
	}
	// Environments are attributed to the runner ID; without one there is
	// nothing this runner could have created.
	if runner.ID <= 0 {
		return
	}

	sweeper, ok := l.providers.GetByName(runner.Executor).(common.RuntimeEnvironmentSweeper)
	if !ok {
		return
	}

	now := l.now()

	ctx, cancel := context.WithTimeout(ctx, runtimeEnvironmentSweepTimeout)
	defer cancel()

	logger := l.log.WithFields(logrus.Fields{
		"runner":   runner.ShortDescription(),
		"executor": runner.Executor,
	})
	logger.Debugln("Sweeping expired suspended environments")

	if err := sweeper.SweepRuntimeEnvironments(ctx, runner, now); err != nil {
		logger.WithError(err).Errorln("Failed to sweep expired suspended environments")
	}
}

package machine

import (
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/circuitbreaker"
)

type creationBreakerSettings struct {
	policy      circuitbreaker.FailureRatePolicy
	openInitial time.Duration
	openMax     time.Duration
	probes      int
}

func creationBreakerSettingsFor(config *common.DockerMachine) creationBreakerSettings {
	cb := config.CreationBreaker
	return creationBreakerSettings{
		policy: circuitbreaker.FailureRatePolicy{
			Rate:        cb.GetFailureRate(),
			Window:      cb.GetWindow(),
			Settle:      cb.GetSettle(),
			MinRequests: cb.GetMinCreations(),
		},
		openInitial: cb.GetOpenInitial(),
		openMax:     cb.GetOpenMax(),
		probes:      cb.GetProbes(config.MaxGrowthRate),
	}
}

// creationBreaker returns the runner's machine creation breaker, or nil when
// disabled. Disabling it or changing its settings in a config reload releases
// the creations waiting on it and starts over closed.
func (r *runnerMachinesCoordinator) creationBreaker(config *common.RunnerConfig) *circuitbreaker.Breaker {
	r.breakerLock.Lock()
	defer r.breakerLock.Unlock()

	if !config.Machine.CreationBreakerEnabled() {
		if r.breaker != nil {
			r.breaker.Reset()
			r.breaker = nil
		}
		return nil
	}

	settings := creationBreakerSettingsFor(config.Machine)
	if r.breaker != nil && r.breakerSettings == settings {
		return r.breaker
	}
	if r.breaker != nil {
		r.breaker.Reset()
	}

	logger := breakerLogger(config)
	r.breaker = circuitbreaker.New(0, settings.openInitial,
		circuitbreaker.WithFailureRate(settings.policy),
		circuitbreaker.WithProbes(settings.probes),
		circuitbreaker.WithOpenBackoff(settings.openMax),
		circuitbreaker.WithStateChange(func(from, to circuitbreaker.State) {
			logCreationBreakerChange(logger, from, to)
		}),
	)
	r.breakerSettings = settings
	return r.breaker
}

func logCreationBreakerChange(logger *logrus.Entry, from, to circuitbreaker.State) {
	switch {
	case to == circuitbreaker.Closed:
		logger.Info("Machine creation circuit breaker closed, resuming machine creation")
	case to == circuitbreaker.HalfOpen:
		logger.Info("Machine creation circuit breaker half-open, probing")
	case from == circuitbreaker.HalfOpen:
		logger.Warning("Machine creation probes failed, pausing machine creation")
	default:
		logger.Warning("Machine creation circuit breaker tripped, pausing machine creation")
	}
}

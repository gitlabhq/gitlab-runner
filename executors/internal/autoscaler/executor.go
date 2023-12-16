package autoscaler

import (
	"context"
	"fmt"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type executor struct {
	common.Executor

	provider *provider
	build    *common.Build
	config   common.RunnerConfig
}

func (e *executor) Prepare(options common.ExecutorPrepareOptions) (err error) {
	e.build = options.Build
	e.config = *options.Config

	e.build.Log().Infoln("Preparing instance...")

	acqRef, ok := options.Build.ExecutorData.(*acquisitionRef)
	if !ok {
		return fmt.Errorf("no acquisition ref data")
	}

	// if we already have an acquisition just retry preparing it
	if acqRef.acq != nil {
		return e.Executor.Prepare(options)
	}

	// todo: allow configuration of how long we're willing to wait for.
	// This is currently set to 15minutes knowing that cloud providers
	// can sometimes exceed 6 minutes provisioning a Windows instance.
	// This is far more relevant when we're provisioning instances on demand,
	// rather than waiting for idle instances.
	ctx, cancel := context.WithTimeout(options.Context, 15*time.Minute)
	defer cancel()

	acq, err := e.provider.getRunnerTaskscaler(options.Config).Acquire(ctx, acqRef.key)
	if err != nil {
		return fmt.Errorf("unable to acquire instance: %w", err)
	}

	e.build.Log().WithField("key", acqRef.key).Trace("Acquired capacity...")

	acqRef.acq = acq

	return e.Executor.Prepare(options)
}

func (e *executor) Cleanup() {
	e.Executor.Cleanup()
}

package autoscaler

import (
	"context"
	"fmt"
	"strconv"
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

	acqRef, ok := options.Build.ExecutorData.(*acqusitionRef)
	if !ok {
		return fmt.Errorf("no acqusition ref data")
	}

	// generate key for acqusition
	key := options.Build.Token + strconv.FormatInt(options.Build.ID, 10)

	// todo: allow configuration of how long we're willing to wait for.
	// Or is this already handled by the option's context?
	ctx, cancel := context.WithTimeout(options.Context, 5*time.Minute)
	defer cancel()

	acq, err := e.provider.getRunnerTaskscaler(options.Config).Acquire(ctx, key)
	if err != nil {
		return fmt.Errorf("unable to acquire instance: %w", err)
	}

	acqRef.set(key, acq)

	return e.Executor.Prepare(options)
}

func (e *executor) Cleanup() {
	e.Executor.Cleanup()
	if e.build.ExecutorData == nil {
		return
	}

	e.provider.Release(&e.config, e.build.ExecutorData)
	e.build.ExecutorData = nil
}

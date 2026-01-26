package autoscaler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
	"gitlab.com/gitlab-org/gitlab-runner/steps"
)

var (
	_ terminal.InteractiveTerminal = (*executor)(nil)
	_ steps.Connector              = (*executor)(nil)
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

	// The acqTimeout defines how long we are willing to wait for an instance to be acquired.
	// It defaults to 15 minutes, as cloud providers can take several minutes to provision instances,
	// especially for certain operating systems like Windows. This value can be configured
	// through the Autoscaler configuration (InstanceAcquireTimeout) to better suit the user's environment.
	acqTimeout := 15 * time.Minute
	if options.Config.Autoscaler != nil && options.Config.Autoscaler.InstanceAcquireTimeout > 0 {
		acqTimeout = options.Config.Autoscaler.InstanceAcquireTimeout
	}

	ctx, cancel := context.WithTimeout(options.Context, acqTimeout)
	defer cancel()

	acq, err := e.provider.getRunnerTaskscaler(options.Config).Acquire(ctx, acqRef.key)
	if err != nil {
		// Check if the error is due to the context timeout
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("unable to acquire instance within the configured timeout of %s: %w", acqTimeout, err)
		}

		return fmt.Errorf("unable to acquire instance: %w", err)
	}

	e.build.Log().WithField("key", acqRef.key).Trace("Acquired capacity...")

	acqRef.acq = acq

	return e.Executor.Prepare(options)
}

func (e *executor) Cleanup() {
	e.Executor.Cleanup()
}

func (s *executor) Connect(ctx context.Context) (func() (io.ReadWriteCloser, error), error) {
	if connector, ok := s.Executor.(steps.Connector); ok {
		return connector.Connect(ctx)
	}

	return nil, common.ExecutorStepRunnerConnectNotSupported
}

func (e *executor) TerminalConnect() (terminal.Conn, error) {
	if connector, ok := e.Executor.(terminal.InteractiveTerminal); ok {
		return connector.TerminalConnect()
	}

	return nil, errors.New("executor does not have terminal")
}

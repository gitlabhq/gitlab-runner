package autoscaler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"gitlab.com/gitlab-org/fleeting/taskscaler"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
	"gitlab.com/gitlab-org/gitlab-runner/steps"
)

var (
	_ terminal.InteractiveTerminal = (*executor)(nil)
	_ steps.Connector              = (*executor)(nil)
	_ common.SuspendableExecutor   = (*executor)(nil)
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
		return errors.New("no acquisition ref data")
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

	scaler := e.provider.getRunnerTaskscaler(options.Config)

	// Check for resume path
	envKey := options.Build.EnvironmentKey()
	if envKey != "" {
		return e.prepareResume(ctx, scaler, options, acqRef, envKey)
	}

	// If we already have an acquisition just retry preparing it
	if acqRef.acq != nil {
		return e.Executor.Prepare(options)
	}

	acq, err := scaler.Acquire(ctx, acqRef.key)
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

func (e *executor) prepareResume(ctx context.Context, scaler taskscaler.Taskscaler, options common.ExecutorPrepareOptions, acqRef *acquisitionRef, envKey string) error {
	acqKey, executorFields, err := validateEnvKey(envKey, options.Config.ID, options.Config.GetSystemID())
	if err != nil {
		return err
	}

	if !scaler.HasCapability(fleetingprovider.CapabilitySuspendResume) {
		return &common.BuildError{
			Inner:         errors.New("cloud plugin does not support suspend/resume"),
			FailureReason: common.ConfigurationError,
		}
	}

	acq, err := scaler.Resume(ctx, acqKey)
	if err != nil {
		return fmt.Errorf("resuming acquisition: %w", err)
	}

	// Resume reuses the suspended acquisition embedded in the env key;
	// the fresh one Acquire() just reserved is unused - unreserve it.
	scaler.Unreserve(acqRef.key)
	acqRef.acq = acq
	acqRef.key = acqKey

	// Inner Prepare first: establishes connections (e.g., Docker client)
	if err := e.Executor.Prepare(options); err != nil {
		return err
	}

	// Then let inner executor restore its workload from its own fields.
	if err := e.Resume(ctx, executorFields); err != nil {
		return err
	}

	options.BuildLogger.Infoln("Job environment resumed:", envKey)
	return nil
}

func (e *executor) Suspend(ctx context.Context) (url.Values, error) {
	se, ok := e.Executor.(common.SuspendableExecutor)
	if !ok {
		return nil, errors.New("executor does not support suspend")
	}

	acqRef, ok := e.build.ExecutorData.(*acquisitionRef)
	if !ok {
		return nil, errors.New("no acquisition ref data")
	}

	fields, err := se.Suspend(ctx)
	if err != nil {
		return nil, err
	}

	if _, collision := fields[acqKeyField]; collision {
		return nil, fmt.Errorf("inner executor returned reserved field %q", acqKeyField)
	}

	scaler := e.provider.getRunnerTaskscaler(&e.config)

	if err := scaler.Suspend(acqRef.key); err != nil {
		return nil, err
	}

	af := envKeyFields{acqKey: acqRef.key}.toFields()
	for k, v := range af {
		fields[k] = v
	}
	return fields, nil
}

// Resume restores only the inner executor's workload state. It satisfies the
// common.SuspendableExecutor interface but is not a complete resume of the
// autoscaler-managed environment - it does not touch the taskscaler slot,
// run inner Prepare, or update the acquisition ref. The full resume flow is
// driven from (*executor).Prepare when the build supplies an EnvironmentKey;
// do not call this method directly from outside that flow.
func (e *executor) Resume(ctx context.Context, fields url.Values) error {
	if se, ok := e.Executor.(common.SuspendableExecutor); ok {
		return se.Resume(ctx, fields)
	}
	return errors.New("executor does not support resume")
}

func (e *executor) Cleanup() {
	e.Executor.Cleanup()
}

func (e *executor) Connect(ctx context.Context) (func() (io.ReadWriteCloser, error), error) {
	if connector, ok := e.Executor.(steps.Connector); ok {
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

func validateEnvKey(envKey string, runnerID int64, systemID string) (string, url.Values, error) {
	key, err := common.ParseEnvironmentKey(envKey)
	if err != nil {
		return "", nil, err
	}

	if key.RunnerID != runnerID {
		return "", nil, errors.New("environment key was not issued by this runner")
	}
	if key.SystemID != systemID {
		return "", nil, errors.New("environment key was not issued by this machine")
	}

	data, executorFields, err := parseEnvKeyFields(key.Fields)
	if err != nil {
		return "", nil, fmt.Errorf("environment key fields: %w", err)
	}

	return data.acqKey, executorFields, nil
}

package autoscaler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"time"

	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"gitlab.com/gitlab-org/fleeting/taskscaler"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
	"gitlab.com/gitlab-org/gitlab-runner/steps"
)

// EnvironmentReleaser is implemented by executor providers that can remove a
// suspended environment's workload from a running instance without a job
// attached, for the TTL sweep. Cleanup releases by label directly.
//
// It is provider-level because docker has no connection to the instance
// without a job.
type EnvironmentReleaser interface {
	// ReleaseEnvironment removes all docker objects belonging to the environment
	// identified by acquisitionKey. An environment already gone counts as
	// released (callers should treat ErrEnvironmentNotFound as success).
	ReleaseEnvironment(ctx context.Context, cfg *common.RunnerConfig, conn executors.Client, acquisitionKey string) error
}

// ErrEnvironmentNotFound is returned (wrapped) by ReleaseEnvironment when no
// containers for the environment exist on the instance. The environment is
// considered lost rather than active.
var ErrEnvironmentNotFound = errors.New("environment not found on instance")

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

	// This runs before getRunnerTaskscaler so nothing is set up first and a
	// guard failure on resume leaves the slot suspended.
	if err := e.checkSuspendGuards(options); err != nil {
		return err
	}

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
	envKey := options.Build.RuntimeEnvironmentKey()
	if envKey != "" {
		return e.prepareResume(ctx, scaler, options, acqRef, envKey)
	}

	// If we already have an acquisition just retry preparing it
	if acqRef.acq != nil {
		return e.Executor.Prepare(options)
	}

	// The slot's expiry is stamped here, once, so a resume or sweep by any
	// manager uses it rather than that manager's current TTL. Without
	// FF_SUSPENDABLE_ENVIRONMENTS at the runner level the slot can't be
	// suspended, so it gets no expiry.
	//
	// Slots suspended before expiries were stamped have none, and taskscaler
	// never expires a slot without one, so the sweep never frees them.
	// Their containers have no expires-at label either, so a resume is
	// refused. This is accepted for the experimental
	// FF_SUSPENDABLE_ENVIRONMENTS rollout.
	var acqOpts []taskscaler.AcquireOption
	if options.Config.IsFeatureFlagOn(featureflags.SuspendableEnvironments) {
		acqOpts = append(acqOpts, taskscaler.WithAcquisitionTTL(options.Config.GetRuntimeEnvironmentTTL()))
	}

	acq, err := scaler.Acquire(ctx, acqRef.key, acqOpts...)
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

// checkSuspendGuards rejects a job that asks to suspend when its environment
// could not be resumed or swept later.
func (e *executor) checkSuspendGuards(options common.ExecutorPrepareOptions) error {
	if !options.Build.WillSuspend() {
		return nil
	}

	// Without state_storage a manager restart loses the suspended slot — the
	// environment can be neither resumed nor swept.
	if options.Config.Autoscaler == nil || !options.Config.Autoscaler.StateStorage.Enabled {
		return &common.BuildError{
			Inner:         errors.New("suspendable environments require autoscaler state_storage to be enabled"),
			FailureReason: common.ConfigurationError,
		}
	}

	if e.provider.cfg.SuspendRequiresDedicatedInstance {
		if err := checkDedicatedInstance(options.Config); err != nil {
			return &common.BuildError{Inner: err, FailureReason: common.ConfigurationError}
		}
	}

	return nil
}

// checkDedicatedInstance enforces what suspending a whole instance needs:
// the shared sweep must visit it, and the instance must hold only this job.
func checkDedicatedInstance(cfg *common.RunnerConfig) error {
	switch {
	// a CI variable alone would create environments the sweep never visits:
	// it gates on the runner-level flag and skips runners without an ID
	case !cfg.IsFeatureFlagOn(featureflags.SuspendableEnvironments):
		return errors.New("suspendable environments require FF_SUSPENDABLE_ENVIRONMENTS to be enabled at the runner level")
	case cfg.ID <= 0:
		return errors.New("suspendable environments require a positive runner registration ID")
	// zero means taskscaler's default of one
	case cfg.Autoscaler.CapacityPerInstance > 1:
		return errors.New("suspendable environments require capacity_per_instance = 1")
	// zero means unlimited
	case cfg.Autoscaler.MaxUseCount != 1:
		return errors.New("suspendable environments require max_use_count = 1")
	case cfg.Instance == nil || !cfg.Instance.UseCommonBuildDir:
		return errors.New("suspendable environments require use_common_build_dir = true")
	// the nested VM, and the job's state with it, is gone after the job
	case cfg.Autoscaler.VMIsolation.Enabled:
		return errors.New("suspendable environments are not supported with vm_isolation")
	}

	return nil
}

func (e *executor) prepareResume(ctx context.Context, scaler taskscaler.Taskscaler, options common.ExecutorPrepareOptions, acqRef *acquisitionRef, envKey string) error {
	acqKey, issuerSystemID, executorFields, err := validateEnvKey(envKey, options.Config.ID)
	if err != nil {
		return err
	}

	if !scaler.HasCapability(fleetingprovider.CapabilitySuspendResume) {
		return &common.BuildError{
			Inner:         errors.New("cloud plugin does not support suspend/resume"),
			FailureReason: common.ConfigurationError,
		}
	}

	// A retried prepare already resumed the slot, and Resume would now refuse
	// it as not suspended: reuse the acquisition and restore the workload again.
	if acqRef.acq == nil {
		acq, err := scaler.Resume(ctx, acqKey)
		// Once the sweep frees an expired slot, Resume reports it as not found
		// rather than expired; either way the environment is gone. Slot state is
		// per manager, so not found means gone only for a key this manager
		// issued. A key from another manager keeps the retryable error: its
		// environment may still exist there.
		if errors.Is(err, taskscaler.ErrSlotExpired) ||
			(errors.Is(err, taskscaler.ErrAcquisitionNotFound) && issuerSystemID == options.Config.GetSystemID()) {
			return common.NewRuntimeEnvironmentUnavailableError(fmt.Errorf("resuming acquisition: %w", err))
		}
		if err != nil {
			return fmt.Errorf("resuming acquisition: %w", err)
		}

		// Resume reuses the suspended acquisition embedded in the env key;
		// the fresh one Acquire() just reserved is unused - unreserve it.
		scaler.Unreserve(acqRef.key)
		acqRef.acq = acq
		acqRef.key = acqKey
	}

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
	maps.Copy(fields, af)
	return fields, nil
}

// Resume restores only the inner executor's workload state. It satisfies the
// common.SuspendableExecutor interface but is not a complete resume of the
// autoscaler-managed environment - it does not touch the taskscaler slot,
// run inner Prepare, or update the acquisition ref. The full resume flow is
// driven from (*executor).Prepare when the build supplies an RuntimeEnvironmentKey;
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

	return nil, common.ErrExecutorStepRunnerConnectNotSupported
}

func (e *executor) TerminalConnect() (terminal.Conn, error) {
	if connector, ok := e.Executor.(terminal.InteractiveTerminal); ok {
		return connector.TerminalConnect()
	}

	return nil, errors.New("executor does not have terminal")
}

// validateEnvKey accepts a key issued by any manager of this runner's
// registration. It does not check the system ID: that changes when a manager
// moves to another machine, and the runner ID is what scopes an environment.
// It returns the issuing manager's system ID so the caller can tell whether
// this manager's slot state should hold the key.
func validateEnvKey(envKey string, runnerID int64) (string, string, url.Values, error) {
	key, err := common.ParseRuntimeEnvironmentKey(envKey)
	if err != nil {
		return "", "", nil, err
	}

	if key.RunnerID != runnerID {
		return "", "", nil, errors.New("environment key was not issued by this runner")
	}

	data, executorFields, err := parseEnvKeyFields(key.Fields)
	if err != nil {
		return "", "", nil, fmt.Errorf("environment key fields: %w", err)
	}

	return data.acqKey, key.SystemID, executorFields, nil
}

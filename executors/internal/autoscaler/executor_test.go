//go:build !integration

package autoscaler

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"testing"
	"time"

	"gitlab.com/gitlab-org/fleeting/taskscaler"
	"gitlab.com/gitlab-org/fleeting/taskscaler/mocks"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
	"gitlab.com/gitlab-org/gitlab-runner/steps"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPrepare(t *testing.T) {
	const (
		runnerToken            = "abcdefgh"
		acqRefKey              = "foobar"
		instanceAcquireTimeout = 3 * time.Second
	)

	tests := map[string]struct {
		executorData any
		retry        bool
		setupFn      func(t *testing.T, cfg *common.RunnerConfig)
		assertFn     func(t *testing.T, ts *mocks.Taskscaler, me *common.MockExecutor)
		checkErrFn   func(t *testing.T, err error)
	}{
		"no acquisition ref": {
			executorData: nil,
			retry:        false,
			setupFn:      nil,
			assertFn:     func(t *testing.T, ts *mocks.Taskscaler, me *common.MockExecutor) {},
			checkErrFn: func(t *testing.T, err error) {
				require.Error(t, err, "no acquisition data")
			},
		},
		"new acquisition": {
			executorData: &acquisitionRef{key: acqRefKey},
			retry:        false,
			setupFn: func(t *testing.T, cfg *common.RunnerConfig) {
				cfg.FeatureFlags = map[string]bool{featureflags.SuspendableEnvironments: true}
			},
			assertFn: func(t *testing.T, ts *mocks.Taskscaler, me *common.MockExecutor) {
				// the TTL option stamps the slot's expiry; without it slots never expire
				withTTL := mock.MatchedBy(func(opts []taskscaler.AcquireOption) bool { return len(opts) == 1 })
				ts.EXPECT().Acquire(mock.Anything, acqRefKey, withTTL).Return(mocks.NewAcquisition(t), nil).Once()
				me.On("Prepare", mock.Anything).Return(nil).Once()
			},
			checkErrFn: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		"retry acquisition should Prepare twice": {
			executorData: &acquisitionRef{key: acqRefKey},
			retry:        true,
			setupFn:      nil,
			assertFn: func(t *testing.T, ts *mocks.Taskscaler, me *common.MockExecutor) {
				ts.EXPECT().Acquire(mock.Anything, acqRefKey, mock.Anything).Return(mocks.NewAcquisition(t), nil).Once()
				me.On("Prepare", mock.Anything).Return(nil).Twice()
			},
			checkErrFn: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		"acquire failed due to timeout": {
			executorData: &acquisitionRef{key: acqRefKey},
			retry:        false,
			setupFn: func(t *testing.T, cfg *common.RunnerConfig) {
				cfg.Autoscaler = &common.AutoscalerConfig{InstanceAcquireTimeout: instanceAcquireTimeout}
			},
			assertFn: func(t *testing.T, ts *mocks.Taskscaler, me *common.MockExecutor) {
				ts.EXPECT().Acquire(mock.Anything, acqRefKey, mock.Anything).Return(mocks.NewAcquisition(t), context.DeadlineExceeded).Once()
			},
			checkErrFn: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), fmt.Sprintf("unable to acquire instance within the configured timeout of %s", instanceAcquireTimeout))
			},
		},
		"acquire failed": {
			executorData: &acquisitionRef{key: acqRefKey},
			retry:        false,
			setupFn:      nil,
			assertFn: func(t *testing.T, ts *mocks.Taskscaler, me *common.MockExecutor) {
				ts.EXPECT().Acquire(mock.Anything, acqRefKey, mock.Anything).Return(mocks.NewAcquisition(t), assert.AnError).Once()
			},
			checkErrFn: func(t *testing.T, err error) {
				require.ErrorIs(t, err, assert.AnError)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			runnerCfg := &common.RunnerConfig{}
			runnerCfg.Token = runnerToken
			runnerCfg.ID = 101
			runnerCfg.SystemID = "sys-1"
			if tc.setupFn != nil {
				tc.setupFn(t, runnerCfg)
			}

			ts := mocks.NewTaskscaler(t)
			ep := common.NewMockExecutorProvider(t)
			me := common.NewMockExecutor(t)

			p := New(ep, Config{}).(*provider)
			p.taskscalerNew = mockTaskscalerNew(ts, false)
			p.fleetingRunPlugin = mockFleetingRunPlugin(false)

			p.scalers = map[string]scaler{
				runnerScalerKey(runnerCfg): {internal: ts, shutdown: func(_ context.Context) {}},
			}

			tc.assertFn(t, ts, me)

			e := &executor{
				Executor: me,
				provider: p,
				build: &common.Build{
					Runner:       runnerCfg,
					ExecutorData: tc.executorData,
				},
				config: *runnerCfg,
			}

			err := e.Prepare(common.ExecutorPrepareOptions{
				Config:  runnerCfg,
				Context: t.Context(),
				Build:   e.build,
			})

			if !tc.retry {
				tc.checkErrFn(t, err)
			} else {
				err := e.Prepare(common.ExecutorPrepareOptions{
					Config:  runnerCfg,
					Context: t.Context(),
					Build:   e.build,
				})

				tc.checkErrFn(t, err)
			}
		})
	}
}

// mockDockerExecutor: composes MockExecutor + InteractiveTerminal + Connector.
type mockDockerExecutor struct {
	*common.MockExecutor
	*terminal.MockInteractiveTerminal
	*steps.MockConnector
}

// mockSuspendableExecutor: composes MockExecutor + MockSuspendableExecutor so
// the value satisfies common.SuspendableExecutor.
type mockSuspendableExecutor struct {
	*common.MockExecutor
	*common.MockSuspendableExecutor
}

func TestMachineExecutor_WithoutInteractiveTerminal(t *testing.T) {
	e := executor{
		Executor: common.NewMockExecutor(t),
	}

	conn, err := e.TerminalConnect()
	assert.Error(t, err)
	assert.Nil(t, conn)
}

func TestMachineExecutor_WithoutConnector(t *testing.T) {
	e := executor{
		Executor: common.NewMockExecutor(t),
	}

	conn, err := e.Connect(t.Context())
	assert.ErrorIs(t, err, common.ErrExecutorStepRunnerConnectNotSupported)
	assert.Nil(t, conn)
}

func TestMachineExecutor_WithInteractiveTerminal(t *testing.T) {
	mock := mockDockerExecutor{
		MockExecutor:            common.NewMockExecutor(t),
		MockInteractiveTerminal: terminal.NewMockInteractiveTerminal(t),
	}
	e := executor{
		Executor: &mock,
	}

	mock.MockInteractiveTerminal.EXPECT().TerminalConnect().Return(terminal.NewMockConn(t), nil).Once()

	conn, err := e.TerminalConnect()
	assert.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestMachineExecutor_Connect(t *testing.T) {
	mock := mockDockerExecutor{
		MockExecutor:  common.NewMockExecutor(t),
		MockConnector: steps.NewMockConnector(t),
	}
	e := executor{
		Executor: &mock,
	}

	mock.MockConnector.EXPECT().Connect(t.Context()).Return(nil, nil).Once()

	_, err := e.Connect(t.Context())
	assert.NoError(t, err)
}

func newSuspendableBuild(opts spec.SuspendOptions) *common.Build {
	build := &common.Build{
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				FeatureFlags: map[string]bool{
					featureflags.SuspendableEnvironments: true,
				},
			},
		},
	}
	build.Job.SuspendOptions = opts
	return build
}

// setupResumeTest wires a provider, mocked Taskscaler, SuspendableExecutor inner,
// and a build whose RuntimeEnvironmentKey triggers the resume branch in Prepare.
// runnerToken/newKey must be unique per test (they key into the scalers map).
func setupResumeTest(t *testing.T, runnerToken, newKey, suspendedKey string) (
	*executor, *mocks.Taskscaler, *mockSuspendableExecutor, common.ExecutorPrepareOptions,
) {
	const (
		runnerID = int64(101)
		systemID = "sys-1"
	)
	envKeyValue := fmt.Sprintf("%d/%s/acquisition-key=%s", runnerID, systemID, suspendedKey)

	ts := mocks.NewTaskscaler(t)
	ep := common.NewMockExecutorProvider(t)
	me := &mockSuspendableExecutor{
		MockExecutor:            common.NewMockExecutor(t),
		MockSuspendableExecutor: common.NewMockSuspendableExecutor(t),
	}

	p := New(ep, Config{}).(*provider)
	p.taskscalerNew = mockTaskscalerNew(ts, false)
	p.fleetingRunPlugin = mockFleetingRunPlugin(false)

	build := newSuspendableBuild(spec.SuspendOptions{RuntimeEnvironmentKey: envKeyValue})
	runnerCfg := build.Runner
	runnerCfg.Token = runnerToken
	runnerCfg.ID = runnerID
	runnerCfg.SystemID = systemID

	p.scalers = map[string]scaler{
		runnerScalerKey(runnerCfg): {internal: ts, shutdown: func(_ context.Context) {}},
	}
	build.ExecutorData = &acquisitionRef{key: newKey}

	e := &executor{
		Executor: me,
		provider: p,
		build:    build,
		config:   *runnerCfg,
	}

	opts := common.ExecutorPrepareOptions{
		Config:  runnerCfg,
		Context: t.Context(),
		Build:   build,
	}

	return e, ts, me, opts
}

// setupSuspendTest wires a provider, mocked Taskscaler, and a SuspendableExecutor
// inner with build.ExecutorData already populated as if Acquire had run.
// runnerToken must be unique per test.
func setupSuspendTest(t *testing.T, runnerToken, acqKey string) (
	*executor, *mocks.Taskscaler, *mockSuspendableExecutor,
) {
	ts := mocks.NewTaskscaler(t)
	ep := common.NewMockExecutorProvider(t)
	p := New(ep, Config{}).(*provider)
	p.taskscalerNew = mockTaskscalerNew(ts, false)
	p.fleetingRunPlugin = mockFleetingRunPlugin(false)
	p.scalers = map[string]scaler{
		runnerToken: {internal: ts, shutdown: func(_ context.Context) {}},
	}

	runnerCfg := &common.RunnerConfig{}
	runnerCfg.Token = runnerToken

	acq := mocks.NewAcquisition(t)
	ref := &acquisitionRef{key: acqKey, acq: acq}

	build := newSuspendableBuild(spec.SuspendOptions{SuspendOnSuccess: true})
	build.Runner.Token = runnerToken
	build.ExecutorData = ref

	inner := &mockSuspendableExecutor{
		MockExecutor:            common.NewMockExecutor(t),
		MockSuspendableExecutor: common.NewMockSuspendableExecutor(t),
	}

	e := &executor{
		Executor: inner,
		provider: p,
		build:    build,
		config:   *runnerCfg,
	}

	return e, ts, inner
}

func TestExecutor_Suspend_HappyPath(t *testing.T) {
	const acqKey = "acq-key-exec-suspend-ok"

	e, ts, me := setupSuspendTest(t, "exec-suspend-ok", acqKey)
	ts.EXPECT().Suspend(acqKey).Return(nil).Once()
	me.MockSuspendableExecutor.On("Suspend", mock.Anything).
		Return(url.Values{"foo": []string{"bar"}}, nil).Once()

	fields, err := e.Suspend(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "bar", fields.Get("foo"))
	assert.Equal(t, acqKey, fields.Get("acquisition-key"))
}

func TestExecutor_Suspend_InnerExecutorError(t *testing.T) {
	e, _, me := setupSuspendTest(t, "exec-suspend-inner-err", "acq-key-inner-err")
	// Deliberately no ts.EXPECT().Suspend: a call would fail the test.
	innerErr := fmt.Errorf("boom")
	me.MockSuspendableExecutor.On("Suspend", mock.Anything).Return(nil, innerErr).Once()

	fields, err := e.Suspend(t.Context())
	require.Error(t, err)
	assert.Nil(t, fields)
	assert.ErrorIs(t, err, innerErr)
}

func TestExecutor_Suspend_TaskscalerError(t *testing.T) {
	const acqKey = "acq-key-ts-err"

	e, ts, me := setupSuspendTest(t, "exec-suspend-ts-err", acqKey)
	tsErr := fmt.Errorf("ts boom")
	ts.EXPECT().Suspend(acqKey).Return(tsErr).Once()
	me.MockSuspendableExecutor.On("Suspend", mock.Anything).Return(url.Values{}, nil).Once()

	fields, err := e.Suspend(t.Context())
	require.Error(t, err)
	assert.Nil(t, fields)
	assert.ErrorIs(t, err, tsErr)
}

func TestExecutor_Suspend_InnerReturnsReservedField(t *testing.T) {
	e, ts, me := setupSuspendTest(t, "exec-suspend-collision", "acq-key-collision")
	me.MockSuspendableExecutor.On("Suspend", mock.Anything).
		Return(url.Values{"acquisition-key": []string{"inner-thinks-this-is-mine"}}, nil).Once()
	// Deliberately no ts.EXPECT().Suspend: a call would fail the test.
	_ = ts

	fields, err := e.Suspend(t.Context())
	require.Error(t, err)
	assert.Nil(t, fields)
	assert.Contains(t, err.Error(), "inner executor returned reserved field")
}

func TestExecutor_Suspend_InnerNotSuspendable(t *testing.T) {
	e, _, _ := setupSuspendTest(t, "exec-suspend-not-suspendable", "acq-key-not-suspendable")
	// Override with a plain MockExecutor that does NOT implement SuspendableExecutor.
	e.Executor = common.NewMockExecutor(t)

	fields, err := e.Suspend(t.Context())
	require.Error(t, err)
	assert.Nil(t, fields)
	assert.Contains(t, err.Error(), "executor does not support suspend")
}

func TestExecutor_Resume_InnerNotSuspendable(t *testing.T) {
	const (
		newKey       = "new-key-not-resumable"
		suspendedKey = "suspended-key-not-resumable"
	)

	e, ts, _, opts := setupResumeTest(t, "test-token-not-resumable", newKey, suspendedKey)
	// Override with a plain MockExecutor that does NOT implement SuspendableExecutor.
	plainInner := common.NewMockExecutor(t)
	plainInner.On("Prepare", mock.Anything).Return(nil).Once()
	e.Executor = plainInner

	ts.EXPECT().Unreserve(newKey).Once()
	ts.EXPECT().HasCapability(mock.Anything).Return(true).Once()
	ts.EXPECT().Resume(mock.Anything, suspendedKey).Return(mocks.NewAcquisition(t), nil).Once()

	err := e.Prepare(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "executor does not support resume")
}

func TestReleaseCallsSuspend_NonSuspendPath(t *testing.T) {
	const runnerToken = "release-sus"

	ts := mocks.NewTaskscaler(t)
	ep := common.NewMockExecutorProvider(t)
	p := New(ep, Config{}).(*provider)
	p.taskscalerNew = mockTaskscalerNew(ts, false)
	p.fleetingRunPlugin = mockFleetingRunPlugin(false)
	p.scalers = map[string]scaler{
		runnerToken: {internal: ts, shutdown: func(_ context.Context) {}},
	}

	runnerCfg := &common.RunnerConfig{}
	runnerCfg.Token = runnerToken

	acqKey := "acq-key-release"

	build := newSuspendableBuild(spec.SuspendOptions{
		SuspendOnSuccess: true,
	})
	build.Runner.Token = runnerToken

	acq := mocks.NewAcquisition(t)
	ref := &acquisitionRef{
		key: acqKey,
		acq: acq,
	}

	// Release() now always calls scaler.Release when acq is set
	ts.EXPECT().Release(acqKey).Return().Once()

	p.Release(runnerCfg, ref)
}

func TestExecutor_Resume_UnreservesNewKey(t *testing.T) {
	const (
		newKey       = "test-token-new-key-123"
		suspendedKey = "test-token-suspended-key-456"
	)

	e, ts, me, opts := setupResumeTest(t, "test-token-resume", newKey, suspendedKey)

	// Unreserve targets newKey (the freshly-Acquired slot we don't need),
	// not suspendedKey (the slot we're resuming). Otherwise the env-key's
	// suspended slot would be released and we'd lose the workload.
	ts.EXPECT().Unreserve(newKey).Once()

	mockAcq := mocks.NewAcquisition(t)
	mockAcq.EXPECT().InstanceID().Return("i-123").Maybe()
	ts.EXPECT().HasCapability(mock.Anything).Return(true).Once()
	ts.EXPECT().Resume(mock.Anything, suspendedKey).Return(mockAcq, nil).Once()
	me.MockExecutor.On("Prepare", mock.Anything).Return(nil).Once()
	me.MockSuspendableExecutor.On("Resume", mock.Anything, mock.Anything).Return(nil).Once()

	err := e.Prepare(opts)
	require.NoError(t, err)
}

// A prepare retried after a successful Resume must reuse the resumed
// acquisition: Resume would now refuse the slot as not suspended.
func TestExecutor_Resume_RetriedPrepareReusesAcquisition(t *testing.T) {
	const (
		newKey       = "new-key-retry"
		suspendedKey = "suspended-key-retry"
	)

	e, ts, me, opts := setupResumeTest(t, "test-token-retry", newKey, suspendedKey)

	mockAcq := mocks.NewAcquisition(t)
	ts.EXPECT().HasCapability(mock.Anything).Return(true).Twice()
	ts.EXPECT().Resume(mock.Anything, suspendedKey).Return(mockAcq, nil).Once()
	ts.EXPECT().Unreserve(newKey).Once()
	me.MockExecutor.On("Prepare", mock.Anything).Return(fmt.Errorf("dial failed")).Once()
	me.MockExecutor.On("Prepare", mock.Anything).Return(nil).Once()
	me.MockSuspendableExecutor.On("Resume", mock.Anything, mock.Anything).Return(nil).Once()

	require.Error(t, e.Prepare(opts))
	require.NoError(t, e.Prepare(opts))

	acqRef := opts.Build.ExecutorData.(*acquisitionRef)
	assert.Equal(t, suspendedKey, acqRef.key)
	assert.Same(t, mockAcq, acqRef.acq)
}

func TestExecutor_Resume_InnerPrepareFails_ReturnsError(t *testing.T) {
	const (
		newKey       = "new-key-prep-fail"
		suspendedKey = "suspended-key-prep-fail"
	)

	prepareErr := fmt.Errorf("inner prepare boom")

	e, ts, me, opts := setupResumeTest(t, "test-token-resume-prep-fail", newKey, suspendedKey)

	mockAcq := mocks.NewAcquisition(t)
	mockAcq.EXPECT().InstanceID().Return("i-prep-fail").Maybe()
	ts.EXPECT().Unreserve(newKey).Once()
	ts.EXPECT().HasCapability(mock.Anything).Return(true).Once()
	ts.EXPECT().Resume(mock.Anything, suspendedKey).Return(mockAcq, nil).Once()
	me.MockExecutor.On("Prepare", mock.Anything).Return(prepareErr).Once()

	err := e.Prepare(opts)
	require.ErrorIs(t, err, prepareErr)
}

func TestExecutor_Resume_ExecutorResumeFails_ReturnsError(t *testing.T) {
	const (
		newKey       = "new-key-resume-fail"
		suspendedKey = "suspended-key-resume-fail"
	)

	resumeErr := fmt.Errorf("inner resume boom")

	e, ts, me, opts := setupResumeTest(t, "test-token-resume-resume-fail", newKey, suspendedKey)

	mockAcq := mocks.NewAcquisition(t)
	mockAcq.EXPECT().InstanceID().Return("i-resume-fail").Maybe()
	ts.EXPECT().Unreserve(newKey).Once()
	ts.EXPECT().HasCapability(mock.Anything).Return(true).Once()
	ts.EXPECT().Resume(mock.Anything, suspendedKey).Return(mockAcq, nil).Once()
	me.MockExecutor.On("Prepare", mock.Anything).Return(nil).Once()
	me.MockSuspendableExecutor.On("Resume", mock.Anything, mock.Anything).Return(resumeErr).Once()

	err := e.Prepare(opts)
	require.ErrorIs(t, err, resumeErr)
}

func TestExecutor_Resume_ScalerResumeFails(t *testing.T) {
	const (
		newKey       = "new-key-scaler-resume-err"
		suspendedKey = "suspended-key-resume-err"
	)

	resumeErr := fmt.Errorf("ts resume boom")

	// No Prepare expectation on me.MockExecutor and no Resume expectation on
	// the SuspendableExecutor; their absence means a call fails the test.
	e, ts, _, opts := setupResumeTest(t, "test-token-scaler-resume-err", newKey, suspendedKey)

	ts.EXPECT().HasCapability(mock.Anything).Return(true).Once()
	ts.EXPECT().Resume(mock.Anything, suspendedKey).Return(nil, resumeErr).Once()

	err := e.Prepare(opts)
	require.ErrorIs(t, err, resumeErr)
	assert.NotErrorIs(t, err, common.ErrRuntimeEnvironmentUnavailable)
}

func TestExecutor_Resume_UnavailableClassification(t *testing.T) {
	const (
		newKey       = "new-key-unavailable"
		suspendedKey = "suspended-key-unavailable"
	)

	// setupResumeTest runs this manager as sys-1. ErrAcquisitionNotFound is
	// what Resume returns once the sweep has freed the slot, ErrSlotExpired
	// while it is expired but not yet freed. Slot state is per manager, so a
	// key from another manager is never found here.
	tests := map[string]struct {
		issuer          string
		resumeErr       error
		wantUnavailable bool
	}{
		"own key, slot expired, not yet freed": {
			issuer:          "sys-1",
			resumeErr:       taskscaler.ErrSlotExpired,
			wantUnavailable: true,
		},
		"own key, slot freed by the sweep": {
			issuer:          "sys-1",
			resumeErr:       taskscaler.ErrAcquisitionNotFound,
			wantUnavailable: true,
		},
		"other manager's key, not found": {
			issuer:          "other-sys",
			resumeErr:       taskscaler.ErrAcquisitionNotFound,
			wantUnavailable: false,
		},
		"own key, generic resume error": {
			issuer:          "sys-1",
			resumeErr:       assert.AnError,
			wantUnavailable: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// No Prepare expectation on the inner executor; a call fails the test.
			e, ts, _, opts := setupResumeTest(t, "test-token-unavailable", newKey, suspendedKey)
			opts.Build.Job.SuspendOptions.RuntimeEnvironmentKey = "101/" + tc.issuer + "/acquisition-key=" + suspendedKey

			ts.EXPECT().HasCapability(mock.Anything).Return(true).Once()
			ts.EXPECT().Resume(mock.Anything, suspendedKey).Return(nil, fmt.Errorf("wrapped: %w", tc.resumeErr)).Once()

			err := e.Prepare(opts)
			require.ErrorIs(t, err, tc.resumeErr)

			if !tc.wantUnavailable {
				assert.NotErrorIs(t, err, common.ErrRuntimeEnvironmentUnavailable)
				return
			}

			assert.ErrorIs(t, err, common.ErrRuntimeEnvironmentUnavailable)
			var buildErr *common.BuildError
			require.ErrorAs(t, err, &buildErr)
			assert.Equal(t, common.RuntimeEnvironmentUnavailable, buildErr.FailureReason)
		})
	}
}

func TestExecutor_Resume_ValidateEnvKeyRejections(t *testing.T) {
	const newKey = "new-key-validate"

	tests := map[string]struct {
		runnerToken string
		envKey      string
		wantSubstr  string
	}{
		"wrong runner ID": {
			runnerToken: "test-token-validate-env-key-runner",
			envKey:      "99/sys-1/acquisition-key=k",
			wantSubstr:  "environment key was not issued by this runner",
		},
		"missing acquisition-key": {
			runnerToken: "test-token-validate-env-key-missing",
			envKey:      "101/sys-1/x=y",
			wantSubstr:  "acquisition-key is required",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			// Build a rig and override the env-key for this case. No
			// HasCapability/Resume expectations: validation must short-circuit.
			e, _, _, opts := setupResumeTest(t, tc.runnerToken, newKey, "ignored")
			opts.Build.Job.SuspendOptions.RuntimeEnvironmentKey = tc.envKey

			err := e.Prepare(opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantSubstr)
		})
	}
}

// A key from another manager of the registration (another system ID) is not
// refused by validation: it reaches the scaler's Resume.
func TestExecutor_Resume_AcceptsKeyFromAnotherSystem(t *testing.T) {
	const (
		newKey       = "new-key-other-system"
		suspendedKey = "suspended-key-other-system"
	)
	resumeErr := assert.AnError

	e, ts, _, opts := setupResumeTest(t, "test-token-other-system", newKey, suspendedKey)
	opts.Build.Job.SuspendOptions.RuntimeEnvironmentKey = "101/other-sys/acquisition-key=" + suspendedKey

	ts.EXPECT().HasCapability(mock.Anything).Return(true).Once()
	ts.EXPECT().Resume(mock.Anything, suspendedKey).Return(nil, resumeErr).Once()

	err := e.Prepare(opts)
	require.ErrorIs(t, err, resumeErr)
}

func TestExecutor_Resume_NoSuspendResumeCapability(t *testing.T) {
	const (
		newKey       = "new-key-no-capability"
		suspendedKey = "suspended-key-no-capability"
	)

	// No Resume expectation on ts; no Prepare/Resume expectation on the inner.
	e, ts, _, opts := setupResumeTest(t, "test-token-no-capability", newKey, suspendedKey)

	ts.EXPECT().HasCapability(mock.Anything).Return(false).Once()

	err := e.Prepare(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cloud plugin does not support suspend/resume")

	var buildErr *common.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Equal(t, common.ConfigurationError, buildErr.FailureReason)
}

func TestExecutor_StateStorageGuard_WillSuspend_StateStorageOff(t *testing.T) {
	// WillSuspend() + state storage off → ConfigurationError without calling
	// Acquire or Resume.
	const (
		newKey       = "new-key-storage-off"
		suspendedKey = "ignored-key-storage-off"
	)

	e, _, me, opts := setupResumeTest(t, "test-token-storage-off", newKey, suspendedKey)

	// Override the build to have SuspendOnSuccess without a runtime key
	// so it's a fresh suspend job (not a resume), and the state storage guard fires.
	opts.Build.Job.SuspendOptions.RuntimeEnvironmentKey = ""
	opts.Build.Job.SuspendOptions.SuspendOnSuccess = true

	if opts.Config.Autoscaler == nil {
		opts.Config.Autoscaler = &common.AutoscalerConfig{}
	}
	opts.Config.Autoscaler.StateStorage.Enabled = false

	// No Acquire or Prepare expectations — guard must short-circuit before
	// calling getRunnerTaskscaler or Acquire.
	_ = me

	err := e.Prepare(opts)
	require.Error(t, err)
	var buildErr *common.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Equal(t, common.ConfigurationError, buildErr.FailureReason)
	assert.Contains(t, err.Error(), "state_storage")
}

func TestExecutor_StateStorageGuard_WillSuspend_StateStorageOn_Proceeds(t *testing.T) {
	// WillSuspend() + state storage on → proceeds to Acquire and Prepare.
	const (
		newKey       = "new-key-storage-on"
		suspendedKey = "ignored-key-storage-on"
	)

	e, ts, me, opts := setupResumeTest(t, "test-token-storage-on", newKey, suspendedKey)

	opts.Build.Job.SuspendOptions.RuntimeEnvironmentKey = ""
	opts.Build.Job.SuspendOptions.SuspendOnSuccess = true
	opts.Build.ExecutorData = &acquisitionRef{key: newKey}

	if opts.Config.Autoscaler == nil {
		opts.Config.Autoscaler = &common.AutoscalerConfig{}
	}
	opts.Config.Autoscaler.StateStorage.Enabled = true

	ts.EXPECT().Acquire(mock.Anything, newKey, mock.Anything).Return(mocks.NewAcquisition(t), nil).Once()
	me.MockExecutor.On("Prepare", mock.Anything).Return(nil).Once()

	err := e.Prepare(opts)
	require.NoError(t, err)
}

func TestExecutor_StateStorageGuard_FFOff_Proceeds(t *testing.T) {
	// FF off → WillSuspend() returns false → guard not checked → proceeds normally.
	const newKey = "new-key-ff-off"

	runnerCfg := &common.RunnerConfig{}
	runnerCfg.Token = "test-token-ff-off"
	runnerCfg.ID = 101
	// SuspendableEnvironments NOT set → WillSuspend() returns false.

	ts := mocks.NewTaskscaler(t)
	ep := common.NewMockExecutorProvider(t)
	me := common.NewMockExecutor(t)

	p := New(ep, Config{}).(*provider)
	p.taskscalerNew = mockTaskscalerNew(ts, false)
	p.fleetingRunPlugin = mockFleetingRunPlugin(false)
	p.scalers = map[string]scaler{
		runnerScalerKey(runnerCfg): {internal: ts, shutdown: func(_ context.Context) {}},
	}

	build := &common.Build{
		Runner:       runnerCfg,
		ExecutorData: &acquisitionRef{key: newKey},
	}
	// SuspendOnSuccess set but FF off → WillSuspend() returns false.
	build.Job.SuspendOptions.SuspendOnSuccess = true

	e := &executor{
		Executor: me,
		provider: p,
		build:    build,
		config:   *runnerCfg,
	}

	ts.EXPECT().Acquire(mock.Anything, newKey, mock.Anything).Return(mocks.NewAcquisition(t), nil).Once()
	me.On("Prepare", mock.Anything).Return(nil).Once()

	err := e.Prepare(common.ExecutorPrepareOptions{
		Config:  runnerCfg,
		Context: t.Context(),
		Build:   build,
	})
	require.NoError(t, err)
}

func TestPrepare_AcquisitionTTL(t *testing.T) {
	const acqRefKey = "foobar"

	tests := []struct {
		name    string
		ffOff   bool
		setupFn func(*common.RunnerConfig)
		wantTTL time.Duration
	}{
		{
			name: "explicit 2h TTL is forwarded",
			setupFn: func(cfg *common.RunnerConfig) {
				ttl := 2 * time.Hour
				cfg.RuntimeEnvironmentTTL = &ttl
			},
			wantTTL: 2 * time.Hour,
		},
		{
			name:    "unset TTL uses DefaultRuntimeEnvironmentTTL",
			setupFn: nil,
			wantTTL: common.DefaultRuntimeEnvironmentTTL,
		},
		{
			name:  "FF off stamps no TTL",
			ffOff: true,
			setupFn: func(cfg *common.RunnerConfig) {
				ttl := 2 * time.Hour
				cfg.RuntimeEnvironmentTTL = &ttl
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runnerCfg := &common.RunnerConfig{}
			runnerCfg.Token = "token"
			runnerCfg.ID = 101
			runnerCfg.SystemID = "sys-1"
			runnerCfg.FeatureFlags = map[string]bool{featureflags.SuspendableEnvironments: !tc.ffOff}
			if tc.setupFn != nil {
				tc.setupFn(runnerCfg)
			}

			ts := mocks.NewTaskscaler(t)
			ep := common.NewMockExecutorProvider(t)
			me := common.NewMockExecutor(t)

			p := New(ep, Config{}).(*provider)
			p.taskscalerNew = mockTaskscalerNew(ts, false)
			p.fleetingRunPlugin = mockFleetingRunPlugin(false)
			p.scalers = map[string]scaler{
				runnerScalerKey(runnerCfg): {internal: ts, shutdown: func(_ context.Context) {}},
			}

			var capturedOpts []taskscaler.AcquireOption
			if tc.ffOff {
				// The mock drops an empty variadic, so any option fails to match.
				ts.EXPECT().Acquire(mock.Anything, acqRefKey).Return(mocks.NewAcquisition(t), nil).Once()
			} else {
				ts.EXPECT().Acquire(mock.Anything, acqRefKey, mock.MatchedBy(func(opts []taskscaler.AcquireOption) bool {
					capturedOpts = opts
					return len(opts) == 1
				})).Return(mocks.NewAcquisition(t), nil).Once()
			}
			me.On("Prepare", mock.Anything).Return(nil).Once()

			e := &executor{
				Executor: me,
				provider: p,
				build: &common.Build{
					Runner:       runnerCfg,
					ExecutorData: &acquisitionRef{key: acqRefKey},
				},
				config: *runnerCfg,
			}

			err := e.Prepare(common.ExecutorPrepareOptions{
				Config:  runnerCfg,
				Context: t.Context(),
				Build:   e.build,
			})
			require.NoError(t, err)

			if tc.ffOff {
				return
			}

			require.Len(t, capturedOpts, 1, "expected exactly one AcquireOption (WithAcquisitionTTL)")
			want := acquireOptionState(t, taskscaler.WithAcquisitionTTL(tc.wantTTL))
			got := acquireOptionState(t, capturedOpts[0])
			assert.Equal(t, want, got, "AcquisitionTTL mismatch: want %v", tc.wantTTL)
		})
	}
}

// acquireOptionState applies opt to a freshly allocated acquireOptions value
// (the struct is unexported in taskscaler) so two AcquireOptions can be
// compared semantically via reflect.DeepEqual rather than as function pointers,
// without naming the unexported fields.
func acquireOptionState(t *testing.T, opt taskscaler.AcquireOption) any {
	t.Helper()
	fn := reflect.ValueOf(opt)
	o := reflect.New(fn.Type().In(0).Elem())
	require.Nil(t, fn.Call([]reflect.Value{o})[0].Interface(), "AcquireOption must not error")
	return o.Interface()
}

func TestExecutor_DedicatedInstanceGuard(t *testing.T) {
	const (
		newKey       = "new-key-dedicated"
		suspendedKey = "suspended-key-dedicated"
	)

	tests := map[string]struct {
		notDedicated bool // the executor doesn't require a dedicated instance
		resume       bool // the job resumes and asks to suspend again
		noSuspend    bool // the job doesn't ask to suspend
		modify       func(cfg *common.RunnerConfig)
		wantErr      string
	}{
		"valid":                         {},
		"default capacity_per_instance": {modify: func(cfg *common.RunnerConfig) { cfg.Autoscaler.CapacityPerInstance = 0 }},
		"runner-level FF off":           {modify: func(cfg *common.RunnerConfig) { cfg.FeatureFlags = nil }, wantErr: "FF_SUSPENDABLE_ENVIRONMENTS"},
		"no runner ID":                  {modify: func(cfg *common.RunnerConfig) { cfg.ID = 0 }, wantErr: "positive runner registration ID"},
		"capacity_per_instance above 1": {modify: func(cfg *common.RunnerConfig) { cfg.Autoscaler.CapacityPerInstance = 2 }, wantErr: "capacity_per_instance = 1"},
		"unlimited max_use_count":       {modify: func(cfg *common.RunnerConfig) { cfg.Autoscaler.MaxUseCount = 0 }, wantErr: "max_use_count = 1"},
		"max_use_count above 1":         {modify: func(cfg *common.RunnerConfig) { cfg.Autoscaler.MaxUseCount = 2 }, wantErr: "max_use_count = 1"},
		"use_common_build_dir off":      {modify: func(cfg *common.RunnerConfig) { cfg.Instance.UseCommonBuildDir = false }, wantErr: "use_common_build_dir = true"},
		"no instance section":           {modify: func(cfg *common.RunnerConfig) { cfg.Instance = nil }, wantErr: "use_common_build_dir = true"},
		"vm_isolation":                  {modify: func(cfg *common.RunnerConfig) { cfg.Autoscaler.VMIsolation.Enabled = true }, wantErr: "vm_isolation"},
		"resume fails before Resume":    {resume: true, modify: func(cfg *common.RunnerConfig) { cfg.Autoscaler.MaxUseCount = 0 }, wantErr: "max_use_count = 1"},
		"job not suspending is exempt":  {noSuspend: true, modify: func(cfg *common.RunnerConfig) { cfg.Autoscaler.MaxUseCount = 0 }},
		"other executors are exempt":    {notDedicated: true, modify: func(cfg *common.RunnerConfig) { cfg.Autoscaler.MaxUseCount = 0 }},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e, ts, me, opts := setupResumeTest(t, "test-token-dedicated-"+name, newKey, suspendedKey)
			e.provider.cfg.SuspendRequiresDedicatedInstance = !tt.notDedicated

			if !tt.resume {
				opts.Build.Job.SuspendOptions.RuntimeEnvironmentKey = ""
			}
			opts.Build.Job.SuspendOptions.SuspendOnSuccess = !tt.noSuspend

			// a copy, so the job keeps its feature flag when a row turns off
			// the runner-level one, as a CI variable would
			cfg := *opts.Config
			opts.Config = &cfg
			opts.Config.FeatureFlags = map[string]bool{featureflags.SuspendableEnvironments: true}
			opts.Config.Autoscaler = &common.AutoscalerConfig{MaxUseCount: 1, CapacityPerInstance: 1}
			opts.Config.Autoscaler.StateStorage.Enabled = true
			opts.Config.Instance = &common.InstanceConfig{UseCommonBuildDir: true}
			if tt.modify != nil {
				tt.modify(opts.Config)
			}

			if tt.wantErr != "" {
				// no Acquire or Resume expectations: the guard must fail first
				err := e.Prepare(opts)
				var buildErr *common.BuildError
				require.ErrorAs(t, err, &buildErr)
				assert.Equal(t, common.ConfigurationError, buildErr.FailureReason)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			ts.EXPECT().Acquire(mock.Anything, newKey, mock.Anything).Return(mocks.NewAcquisition(t), nil).Once()
			me.MockExecutor.On("Prepare", mock.Anything).Return(nil).Once()

			require.NoError(t, e.Prepare(opts))
		})
	}
}

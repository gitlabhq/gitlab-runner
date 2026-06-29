//go:build !integration

package autoscaler

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

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
		executorData interface{}
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
			setupFn:      nil,
			assertFn: func(t *testing.T, ts *mocks.Taskscaler, me *common.MockExecutor) {
				ts.EXPECT().Acquire(mock.Anything, acqRefKey).Return(mocks.NewAcquisition(t), nil).Once()
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
				ts.EXPECT().Acquire(mock.Anything, acqRefKey).Return(mocks.NewAcquisition(t), nil).Once()
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
				ts.EXPECT().Acquire(mock.Anything, acqRefKey).Return(mocks.NewAcquisition(t), context.DeadlineExceeded).Once()
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
				ts.EXPECT().Acquire(mock.Anything, acqRefKey).Return(mocks.NewAcquisition(t), assert.AnError).Once()
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
	assert.ErrorIs(t, err, common.ExecutorStepRunnerConnectNotSupported)
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
// and a build whose EnvironmentKey triggers the resume branch in Prepare.
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

	build := newSuspendableBuild(spec.SuspendOptions{EnvironmentKey: envKeyValue})
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
		"wrong system ID": {
			runnerToken: "test-token-validate-env-key-system",
			envKey:      "101/other-sys/acquisition-key=k",
			wantSubstr:  "environment key was not issued by this machine",
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
			opts.Build.Job.SuspendOptions.EnvironmentKey = tc.envKey

			err := e.Prepare(opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantSubstr)
		})
	}
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
}

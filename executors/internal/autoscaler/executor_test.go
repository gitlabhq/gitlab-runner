//go:build !integration

package autoscaler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gitlab.com/gitlab-org/fleeting/taskscaler/mocks"
	"gitlab.com/gitlab-org/gitlab-runner/common"
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
				runnerToken: {internal: ts, shutdown: func(_ context.Context) {}},
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

// mockDockerExecutor implements InteractiveTerminal and Connector.
type mockDockerExecutor struct {
	*common.MockExecutor
	*terminal.MockInteractiveTerminal
	*steps.MockConnector
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

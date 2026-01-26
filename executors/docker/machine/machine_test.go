//go:build !integration

package machine

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
	"gitlab.com/gitlab-org/gitlab-runner/steps"
)

func getRunnerConfig() *common.RunnerConfig {
	return &common.RunnerConfig{
		Name: "runner",
		RunnerSettings: common.RunnerSettings{
			Executor: "docker+machine",
			Docker: &common.DockerConfig{
				Credentials: docker.Credentials{},
				Image:       "alpine",
			},
		},
	}
}

func getRunnerConfigWithoutDockerConfig() *common.RunnerConfig {
	return &common.RunnerConfig{
		Name: "runner",
		RunnerSettings: common.RunnerSettings{
			Executor: "docker+machine",
		},
	}
}

type machineCredentialsUsageFakeExecutor struct {
	t *testing.T

	expectedMachineCredentials docker.Credentials
	expectedRunnerConfig       *common.RunnerConfig
}

func (e *machineCredentialsUsageFakeExecutor) assertRunnerConfiguration(runnerConfig *common.RunnerConfig) {
	assert.Equal(e.t, e.expectedRunnerConfig.Name, runnerConfig.Name)
	assert.Equal(e.t, e.expectedRunnerConfig.RunnerSettings.Executor, runnerConfig.RunnerSettings.Executor)
	if e.expectedRunnerConfig.Docker != nil {
		assert.Equal(e.t, e.expectedRunnerConfig.Docker.Image, runnerConfig.Docker.Image)
	}
	assert.Equal(
		e.t,
		e.expectedMachineCredentials,
		runnerConfig.Docker.Credentials,
		"Credentials should be filled with machine's credentials",
	)
}

func (e *machineCredentialsUsageFakeExecutor) Prepare(options common.ExecutorPrepareOptions) error {
	e.assertRunnerConfiguration(options.Config)
	e.assertRunnerConfiguration(options.Build.Runner)
	return nil
}

func (e *machineCredentialsUsageFakeExecutor) Shell() *common.ShellScriptInfo             { return nil }
func (e *machineCredentialsUsageFakeExecutor) Run(cmd common.ExecutorCommand) error       { return nil }
func (e *machineCredentialsUsageFakeExecutor) Finish(err error)                           {}
func (e *machineCredentialsUsageFakeExecutor) Cleanup()                                   {}
func (e *machineCredentialsUsageFakeExecutor) SetCurrentStage(stage common.ExecutorStage) {}
func (e *machineCredentialsUsageFakeExecutor) GetCurrentStage() common.ExecutorStage {
	return common.ExecutorStageCreated
}

func testMachineCredentialsUsage(t *testing.T, name string, runnerConfigSource func() *common.RunnerConfig) {
	t.Run(name, func(t *testing.T) {
		machineName := "expected-machine"
		machineCredentials := docker.Credentials{
			Host: "tcp://expected-host:1234",
		}

		runnerConfig := runnerConfigSource()
		options := common.ExecutorPrepareOptions{
			Config: runnerConfig,
			Build: &common.Build{
				Runner: runnerConfig,
				ExecutorData: &machineDetails{
					Name:  machineName,
					State: machineStateAcquired,
				},
			},
		}

		machine := docker.NewMockMachine(t)

		machine.On("CanConnect", mock.Anything, machineName, true).
			Return(true).Once()
		machine.On("Credentials", mock.Anything, machineName).
			Return(machineCredentials, nil).Once()

		executorProvider := common.NewMockExecutorProvider(t)

		fakeExecutor := &machineCredentialsUsageFakeExecutor{
			t:                          t,
			expectedMachineCredentials: machineCredentials,
			expectedRunnerConfig:       runnerConfigSource(),
		}
		executorProvider.On("Create").
			Return(fakeExecutor).Once()

		e := &machineExecutor{
			provider: &machineProvider{
				machine:  machine,
				provider: executorProvider,
				totalActions: prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "actions_total",
						Help: "actions_total",
					},
					[]string{"action"},
				),
			},
		}
		err := e.Prepare(options)
		assert.NoError(t, err)
	})
}

func TestMachineCredentialsUsage(t *testing.T) {
	testMachineCredentialsUsage(t, "config-with-docker-section", getRunnerConfig)
	testMachineCredentialsUsage(t, "config-without-docker-section", getRunnerConfigWithoutDockerConfig)
}

// mockDockerExecutor implements InteractiveTerminal and Connector.
type mockDockerExecutor struct {
	*common.MockExecutor
	*terminal.MockInteractiveTerminal
	*steps.MockConnector
}

func TestMachineExecutor_WithoutInteractiveTerminal(t *testing.T) {
	e := machineExecutor{
		executor: common.NewMockExecutor(t),
	}

	conn, err := e.TerminalConnect()
	assert.Error(t, err)
	assert.Nil(t, conn)
}

func TestMachineExecutor_WithoutConnector(t *testing.T) {
	e := machineExecutor{
		executor: common.NewMockExecutor(t),
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
	e := machineExecutor{
		executor: &mock,
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
	e := machineExecutor{
		executor: &mock,
	}

	mock.MockConnector.EXPECT().Connect(t.Context()).Return(nil, nil).Once()

	_, err := e.Connect(t.Context())
	assert.NoError(t, err)
}

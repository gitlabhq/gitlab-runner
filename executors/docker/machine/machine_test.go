//go:build !integration

package machine

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
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

func (e *machineCredentialsUsageFakeExecutor) Name() string {
	return "fake"
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

		machine := new(docker.MockMachine)
		defer machine.AssertExpectations(t)

		machine.On("CanConnect", machineName, true).
			Return(true).Once()
		machine.On("Credentials", machineName).
			Return(machineCredentials, nil).Once()

		executorProvider := &common.MockExecutorProvider{}
		defer executorProvider.AssertExpectations(t)

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

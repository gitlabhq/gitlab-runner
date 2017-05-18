package machine

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers/docker"
)

func getRunnerConfig() *common.RunnerConfig {
	return &common.RunnerConfig{
		Name: "runner",
		RunnerSettings: common.RunnerSettings{
			Executor: "docker+machine",
			Docker: &common.DockerConfig{
				DockerCredentials: docker_helpers.DockerCredentials{},
				Image:             "alpine",
			},
		},
	}
}

type machineCredentialsUsageFakeExecutor struct {
	t                  *testing.T
	machineCredentials docker_helpers.DockerCredentials
}

func (e *machineCredentialsUsageFakeExecutor) assertRunnerConfiguration(runnerConfig *common.RunnerConfig) {
	expectedRunnerConfig := getRunnerConfig()

	assert.Equal(e.t, expectedRunnerConfig.Name, runnerConfig.Name)
	assert.Equal(e.t, expectedRunnerConfig.RunnerSettings.Executor, runnerConfig.RunnerSettings.Executor)
	assert.Equal(e.t, expectedRunnerConfig.Docker.Image, runnerConfig.Docker.Image)
	assert.Equal(e.t, e.machineCredentials, runnerConfig.Docker.DockerCredentials, "DockerCredentials should be filled with machine's credentials")

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

func TestMachineCredentialsUsage(t *testing.T) {
	machineName := "expected-machine"
	machineCredentials := docker_helpers.DockerCredentials{
		Host: "tcp://expected-host:1234",
	}

	runnerConfig := getRunnerConfig()
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

	machine := &docker_helpers.MockMachine{}
	defer machine.AssertExpectations(t)

	machine.On("CanConnect", machineName).
		Return(true).Once()
	machine.On("Credentials", machineName).
		Return(machineCredentials, nil).Once()

	executorProvider := &common.MockExecutorProvider{}
	defer executorProvider.AssertExpectations(t)

	fakeExecutor := &machineCredentialsUsageFakeExecutor{t, machineCredentials}
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
}

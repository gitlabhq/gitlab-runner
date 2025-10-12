//go:build !integration

package commands

import (
	"context"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/commands/internal/configfile"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func init() {
	s := common.MockShell{}
	s.On("GetName").Return("script-shell")
	s.On("GenerateScript", mock.Anything, mock.Anything, mock.Anything).Return("script", nil)
	common.RegisterShell(&s)
}

type jobSimulation func(mock.Arguments)

func TestSingleRunnerSigquit(t *testing.T) {
	var sendQuitSignal func()

	job := func(_ mock.Arguments) {
		sendQuitSignal()
		// simulate some real work while while sigquit get handled
		time.Sleep(time.Second)
	}

	single := mockingExecutionStack(t, "test-sigquit", 1, job)

	sendQuitSignal = func() {
		single.interruptSignals <- syscall.SIGQUIT
	}

	single.Execute(nil)
}

func TestSingleRunnerMaxBuilds(t *testing.T) {
	maxBuilds := 7

	single := mockingExecutionStack(t, "test-max-build", maxBuilds, nil)

	single.Execute(nil)
}

func TestConfigFile(t *testing.T) {
	// create config file
	config_file, err := os.CreateTemp("", "gitlab-runner-test")
	require.NoError(t, err)
	filename := config_file.Name()
	defer os.Remove(filename)
	// fill config file with multiple runners
	_, err = config_file.WriteString(`[[runners]]
	name = "runner"
	token= "t1"
	url = "https://example.com/"
	executor = "shell"
	[[runners]]
	name = "runner2"
	token = "t2"
	url = "https://example.com/"
	executor = "shell"`)
	require.NoError(t, err)
	err = config_file.Close()
	require.NoError(t, err)
	// create command config for runner2
	config := RunSingleCommand{ConfigFile: filename, RunnerName: "runner2"}

	config.HandleArgs()

	assert.Equal(t, "t2", config.Token)
}

func newRunSingleCommand(executorName string, network common.Network) *RunSingleCommand {
	systemID, _ := configfile.GenerateUniqueSystemID()

	return &RunSingleCommand{
		network: network,
		RunnerConfig: common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: executorName,
			},
			RunnerCredentials: common.RunnerCredentials{
				URL:   "http://example.com",
				Token: "_test_token_",
			},
			SystemID: systemID,
		},
		interruptSignals: make(chan os.Signal),
	}
}

func mockingExecutionStack(
	t *testing.T,
	executorName string,
	maxBuilds int,
	job jobSimulation,
) *RunSingleCommand {
	// mocking the whole stack
	e := common.NewMockExecutor(t)
	p := common.NewMockExecutorProvider(t)
	mockNetwork := common.NewMockNetwork(t)

	// Network
	jobData := common.JobResponse{}
	_, cancel := context.WithCancel(t.Context())
	jobTrace := common.Trace{Writer: io.Discard}
	jobTrace.SetCancelFunc(cancel)
	jobTrace.SetAbortFunc(cancel)
	mockNetwork.On("RequestJob", mock.Anything, mock.Anything, mock.Anything).Return(&jobData, true).Times(maxBuilds)
	// Mock UpdateJob to return success for existing tests
	mockNetwork.On("UpdateJob", mock.Anything, mock.Anything, mock.Anything).Return(common.UpdateJobResult{State: common.UpdateSucceeded}).Times(maxBuilds)
	processJob := mockNetwork.On("ProcessJob", mock.Anything, mock.Anything).Return(&jobTrace, nil).Times(maxBuilds)
	if job != nil {
		processJob.Run(job)
	}

	// ExecutorProvider
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Times(maxBuilds + 1)

	p.On("Create").Return(e).Times(maxBuilds)
	p.On("Acquire", mock.Anything).Return(common.NewMockExecutorData(t), nil).Times(maxBuilds)
	p.On("Release", mock.Anything, mock.Anything).Return(nil).Times(maxBuilds)

	// Executor
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(maxBuilds)
	e.On("Finish", nil).Times(maxBuilds)
	e.On("Cleanup").Times(maxBuilds)

	// Run script successfully
	e.On("Shell").Return(&common.ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", mock.Anything).Return(nil)

	common.RegisterExecutorProviderForTest(t, executorName, p)

	single := newRunSingleCommand(executorName, mockNetwork)
	single.MaxBuilds = maxBuilds

	t.Cleanup(cancel)

	return single
}

func TestRunSingleCommand_processBuild_HandlesUpdateAbort(t *testing.T) {
	runner := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-token",
		},
	}

	jobData := &common.JobResponse{
		ID:    123,
		Token: "job-token",
	}

	network := common.NewMockNetwork(t)
	mockTrace := common.NewMockJobTrace(t)
	mockTrace.On("SetDebugModeEnabled", false).Return()
	mockTrace.On("Finish").Return()

	// Mock RequestJob to return a job
	network.On("RequestJob", mock.Anything, *runner, mock.Anything).Return(jobData, true)
	// Mock ProcessJob to return a trace
	network.On("ProcessJob", *runner, mock.AnythingOfType("*common.JobCredentials")).Return(mockTrace, nil)
	// Mock UpdateJob to return UpdateAbort
	network.On("UpdateJob", *runner, mock.AnythingOfType("*common.JobCredentials"), mock.AnythingOfType("common.UpdateJobInfo")).
		Return(common.UpdateJobResult{State: common.UpdateAbort})

	cmd := &RunSingleCommand{
		RunnerConfig: *runner,
		network:      network,
	}

	err := cmd.processBuild(common.NewMockExecutorData(t), make(chan os.Signal))

	// When UpdateJob returns UpdateAbort, processBuild should return nil (no error)
	assert.Nil(t, err, "Should return no error when update is aborted")

	network.AssertExpectations(t)
	mockTrace.AssertExpectations(t)
}

func TestRunSingleCommand_processBuild_HandlesCancelRequested(t *testing.T) {
	runner := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-token",
		},
	}

	jobData := &common.JobResponse{
		ID:    123,
		Token: "job-token",
	}

	network := common.NewMockNetwork(t)
	mockTrace := common.NewMockJobTrace(t)
	mockTrace.On("SetDebugModeEnabled", false).Return()
	mockTrace.On("Finish").Return()

	// Mock RequestJob to return a job
	network.On("RequestJob", mock.Anything, *runner, mock.Anything).Return(jobData, true)
	// Mock ProcessJob to return a trace
	network.On("ProcessJob", *runner, mock.AnythingOfType("*common.JobCredentials")).Return(mockTrace, nil)
	// Mock UpdateJob to return success but with CancelRequested=true
	network.On("UpdateJob", *runner, mock.AnythingOfType("*common.JobCredentials"), mock.AnythingOfType("common.UpdateJobInfo")).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded, CancelRequested: true})

	cmd := &RunSingleCommand{
		RunnerConfig: *runner,
		network:      network,
	}

	err := cmd.processBuild(common.NewMockExecutorData(t), make(chan os.Signal))

	// When UpdateJob has CancelRequested=true, processBuild should return nil (no error)
	assert.Nil(t, err, "Should return no error when job is being canceled")

	network.AssertExpectations(t)
	mockTrace.AssertExpectations(t)
}

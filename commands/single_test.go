package commands

import (
	"context"
	"io/ioutil"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

func init() {
	s := common.MockShell{}
	s.On("GetName").Return("script-shell")
	s.On("GenerateScript", mock.Anything, mock.Anything).Return("script", nil)
	common.RegisterShell(&s)
}

func TestSingleRunnerSigquit(t *testing.T) {
	assert := assert.New(t)
	executorName := "test-sigquit"

	// mocking the whole stack
	e := common.MockExecutor{}
	defer e.AssertExpectations(t)
	p := common.MockExecutorProvider{}
	defer p.AssertExpectations(t)
	mockNetwork := common.MockNetwork{}
	defer mockNetwork.AssertExpectations(t)

	//Network
	jobData := common.JobResponse{}
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobTrace := common.Trace{Writer: ioutil.Discard, CancelFunc: cancel}
	mockNetwork.On("RequestJob", mock.Anything).Return(&jobData, true)
	mockNetwork.On("ProcessJob", mock.Anything, mock.Anything).Return(&jobTrace).Run(func(_ mock.Arguments) {
		err := syscall.Kill(syscall.Getpid(), syscall.SIGQUIT)
		assert.NoError(err)
		// simulate some real work while while sigquit get handled
		time.Sleep(time.Second)
	})

	// Create executor only once
	p.On("Create").Return(&e).Once()
	p.On("Acquire", mock.Anything).Return(&common.MockExecutorData{}, nil).Once()
	p.On("Release", mock.Anything, mock.Anything).Return(nil).Once()

	// We run everything once
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	e.On("Finish", nil).Return().Once()
	e.On("Cleanup").Return().Once()

	// Run script successfully
	e.On("Shell").Return(&common.ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", mock.Anything).Return(nil)

	common.RegisterExecutor(executorName, &p)

	single := newRunSingleCommand(executorName, &mockNetwork)
	single.Execute(nil)
}

func TestSingleRunnerMaxBuilds(t *testing.T) {
	executorName := "test-max-build"
	maxBuilds := 7

	// mocking the whole stack
	e := common.MockExecutor{}
	defer e.AssertExpectations(t)
	p := common.MockExecutorProvider{}
	defer p.AssertExpectations(t)
	mockNetwork := common.MockNetwork{}
	defer mockNetwork.AssertExpectations(t)

	//Network
	jobData := common.JobResponse{}
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobTrace := common.Trace{Writer: ioutil.Discard, CancelFunc: cancel}
	mockNetwork.On("RequestJob", mock.Anything).Return(&jobData, true).Times(maxBuilds)
	mockNetwork.On("ProcessJob", mock.Anything, mock.Anything).Return(&jobTrace).Times(maxBuilds)

	//ExecutorProvider
	p.On("Create").Return(&e).Times(maxBuilds)
	p.On("Acquire", mock.Anything).Return(&common.MockExecutorData{}, nil).Times(maxBuilds)
	p.On("Release", mock.Anything, mock.Anything).Return(nil).Times(maxBuilds)

	//Executor
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(maxBuilds)
	e.On("Finish", nil).Return().Times(maxBuilds)
	e.On("Cleanup").Return().Times(maxBuilds)

	// Run script successfully
	e.On("Shell").Return(&common.ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", mock.Anything).Return(nil)

	common.RegisterExecutor(executorName, &p)

	single := newRunSingleCommand(executorName, &mockNetwork)
	single.MaxBuilds = maxBuilds
	single.Execute(nil)
}

func newRunSingleCommand(executorName string, network common.Network) *RunSingleCommand {
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
		},
	}
}

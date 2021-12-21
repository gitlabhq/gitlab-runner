//go:build !integration
// +build !integration

package commands

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/log/test"
)

func TestProcessRunner_BuildLimit(t *testing.T) {
	hook, cleanup := test.NewHook()
	defer cleanup()

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(ioutil.Discard)

	cfg := common.RunnerConfig{
		Limit:              2,
		RequestConcurrency: 10,
		RunnerSettings: common.RunnerSettings{
			Executor: "multi-runner-build-limit",
		},
	}

	jobData := common.JobResponse{
		ID: 1,
		Steps: []common.Step{
			{
				Name:         "sleep",
				Script:       common.StepScript{"sleep 10"},
				Timeout:      15,
				When:         "",
				AllowFailure: false,
			},
		},
	}

	mJobTrace := common.MockJobTrace{}
	defer mJobTrace.AssertExpectations(t)
	mJobTrace.On("SetFailuresCollector", mock.Anything)
	mJobTrace.On("Write", mock.Anything).Return(0, nil)
	mJobTrace.On("IsStdout").Return(false)
	mJobTrace.On("SetCancelFunc", mock.Anything)
	mJobTrace.On("SetAbortFunc", mock.Anything)
	mJobTrace.On("SetMasked", mock.Anything)
	mJobTrace.On("Success")

	mNetwork := common.MockNetwork{}
	defer mNetwork.AssertExpectations(t)
	mNetwork.On("RequestJob", mock.Anything, mock.Anything, mock.Anything).Return(&jobData, true)
	mNetwork.On("ProcessJob", mock.Anything, mock.Anything).Return(&mJobTrace, nil)

	var runningBuilds uint32
	e := common.MockExecutor{}
	defer e.AssertExpectations(t)
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("Cleanup").Maybe()
	e.On("Shell").Return(&common.ShellScriptInfo{Shell: "script-shell"})
	e.On("Finish", mock.Anything).Maybe()
	e.On("Run", mock.Anything).Run(func(args mock.Arguments) {
		atomic.AddUint32(&runningBuilds, 1)

		// Simulate work to fill up build queue.
		time.Sleep(1 * time.Second)
	}).Return(nil)

	p := common.MockExecutorProvider{}
	defer p.AssertExpectations(t)
	p.On("Acquire", mock.Anything).Return(nil, nil)
	p.On("Release", mock.Anything, mock.Anything).Return(nil).Maybe()
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil)
	p.On("Create").Return(&e)

	common.RegisterExecutorProvider("multi-runner-build-limit", &p)

	cmd := RunCommand{
		network:      &mNetwork,
		buildsHelper: newBuildsHelper(),
		configOptionsWithListenAddress: configOptionsWithListenAddress{
			configOptions: configOptions{
				config: &common.Config{
					User: "git",
				},
			},
		},
	}

	runners := make(chan *common.RunnerConfig)

	// Start 2 builds.
	wg := sync.WaitGroup{}
	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func(i int) {
			defer wg.Done()

			err := cmd.processRunner(i, &cfg, runners)
			assert.NoError(t, err)
		}(i)
	}

	// Wait until at least two builds have started.
	for atomic.LoadUint32(&runningBuilds) < 2 {
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for all builds to finish.
	wg.Wait()

	limitMetCount := 0
	for _, entry := range hook.AllEntries() {
		if strings.Contains(entry.Message, "runner limit met") {
			limitMetCount++
		}
	}

	assert.Equal(t, 1, limitMetCount)
}

func TestRunCommand_doJobRequest(t *testing.T) {
	returnedJob := new(common.JobResponse)

	waitForContext := func(ctx context.Context) {
		<-ctx.Done()
	}

	tests := map[string]struct {
		requestJob             func(ctx context.Context)
		passSignal             func(c *RunCommand)
		expectedContextTimeout bool
	}{
		"requestJob returns immediately": {
			requestJob:             func(_ context.Context) {},
			passSignal:             func(_ *RunCommand) {},
			expectedContextTimeout: false,
		},
		"requestJob hangs indefinitely": {
			requestJob:             waitForContext,
			passSignal:             func(_ *RunCommand) {},
			expectedContextTimeout: true,
		},
		"requestJob interrupted by interrupt signal": {
			requestJob: waitForContext,
			passSignal: func(c *RunCommand) {
				c.runInterruptSignal <- os.Interrupt
			},
			expectedContextTimeout: false,
		},
		"runFinished signal is passed": {
			requestJob: waitForContext,
			passSignal: func(c *RunCommand) {
				close(c.runFinished)
			},
			expectedContextTimeout: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			runner := new(common.RunnerConfig)

			network := new(common.MockNetwork)
			defer network.AssertExpectations(t)

			network.On("RequestJob", mock.Anything, *runner, mock.Anything).
				Run(func(args mock.Arguments) {
					ctx, ok := args.Get(0).(context.Context)
					require.True(t, ok)

					tt.requestJob(ctx)
				}).
				Return(returnedJob, true).
				Once()

			c := &RunCommand{
				network:            network,
				runInterruptSignal: make(chan os.Signal),
				runFinished:        make(chan bool),
			}

			ctx, cancelFn := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancelFn()

			go tt.passSignal(c)

			job, _ := c.doJobRequest(ctx, runner, nil)

			assert.Equal(t, returnedJob, job)

			if tt.expectedContextTimeout {
				assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
				return
			}
			assert.NoError(t, ctx.Err())
		})
	}
}

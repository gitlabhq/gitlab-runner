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

func TestRunCommand_nextRunnerToReset(t *testing.T) {
	testCases := map[string]struct {
		runners           []common.RunnerCredentials
		expectedIndex     int
		expectedResetTime time.Time
	}{
		"no runners": {
			runners:           []common.RunnerCredentials{},
			expectedIndex:     -1,
			expectedResetTime: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		"no expiration time": {
			runners: []common.RunnerCredentials{
				{
					ID:             1,
					TokenExpiresAt: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedIndex:     -1,
			expectedResetTime: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		"same expiration time": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 5, 0, 0, 0, 0, time.UTC),
				},
				{
					ID:              2,
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 5, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedIndex:     0,
			expectedResetTime: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC),
		},
		"different expiration time": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC),
				},
				{
					ID:              2,
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 5, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedIndex:     1,
			expectedResetTime: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC),
		},
		"different obtained time": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					TokenObtainedAt: time.Date(2022, 1, 5, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC),
				},
				{
					ID:              2,
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedIndex:     1,
			expectedResetTime: time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC),
		},
		"old configuration": {
			runners: []common.RunnerCredentials{
				{
					URL: "https://gitlab1.example.com/",
					// No ID nor time values - replicates entry from before the change was added
				},
				{
					URL: "https://gitlab2.example.com/",
					// No ID nor time values - replicates entry from before the change was added
				},
			},
			expectedIndex:     -1,
			expectedResetTime: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			config := common.NewConfig()

			for _, r := range tc.runners {
				config.Runners = append(config.Runners, &common.RunnerConfig{
					RunnerCredentials: r,
				})
			}

			runnerToReset, resetTime := nextRunnerToReset(config)
			if tc.expectedIndex < 0 {
				assert.Nil(t, runnerToReset)
				assert.True(t, resetTime.IsZero())
				return
			}

			assert.Equal(t, tc.runners[tc.expectedIndex], runnerToReset.RunnerCredentials)
			assert.Equal(t, tc.expectedResetTime, resetTime)
		})
	}
}

type runAtRequest struct {
	time     time.Time
	callback func()
	task     *runAtMockTask
}

type resetTokenRequest struct {
	runner common.RunnerCredentials
}

type resetRunnerTokenTestController struct {
	t                            *testing.T
	runCommand                   RunCommand
	eventChan                    chan interface{}
	resetRunnerTokenResponseChan chan *common.ResetTokenResponse
	waitGroup                    sync.WaitGroup
}

type runAtMockTask struct {
	finished  bool
	cancelled bool
}

func (t *runAtMockTask) cancel() bool {
	finished := t.finished || t.cancelled
	t.cancelled = true
	return !finished
}

func makeResetRunnerTokenTestController(t *testing.T) *resetRunnerTokenTestController {
	mNetwork := common.MockNetwork{}

	data := &resetRunnerTokenTestController{
		t: t,
		runCommand: RunCommand{
			configOptionsWithListenAddress: configOptionsWithListenAddress{
				configOptions: configOptions{
					config: common.NewConfig(),
				},
			},
			runAt:          runAt,
			runFinished:    make(chan bool),
			configReloaded: make(chan int),
			network:        &mNetwork,
		},
		eventChan:                    make(chan interface{}),
		resetRunnerTokenResponseChan: make(chan *common.ResetTokenResponse),
	}
	data.runCommand.runAt = data.mockRunAt

	mNetwork.On("ResetToken", mock.Anything).Return(data.mockResetToken)

	return data
}

func (c *resetRunnerTokenTestController) mockRunAt(time time.Time, callback func()) runAtTask {
	task := runAtMockTask{
		finished: false,
	}
	c.eventChan <- runAtRequest{
		time:     time,
		callback: callback,
		task:     &task,
	}
	return &task
}

func (c *resetRunnerTokenTestController) mockResetToken(runner common.RunnerCredentials) *common.ResetTokenResponse {
	c.eventChan <- resetTokenRequest{
		runner: runner,
	}
	return <-c.resetRunnerTokenResponseChan
}

func (c *resetRunnerTokenTestController) awaitRunAtRequest() runAtRequest {
	event := <-c.eventChan
	e := event.(runAtRequest)
	require.NotNil(c.t, e)
	return e
}

func (c *resetRunnerTokenTestController) awaitResetTokenRequest() resetTokenRequest {
	event := <-c.eventChan
	e := event.(resetTokenRequest)
	require.NotNil(c.t, e)
	return e
}

func (c *resetRunnerTokenTestController) handleRunAtRequest(time time.Time) {
	event := c.awaitRunAtRequest()
	assert.Equal(c.t, time, event.time)
	event.callback()
	event.task.finished = true
}

func (c *resetRunnerTokenTestController) handleResetTokenRequest(runnerID int, response *common.ResetTokenResponse) {
	event := c.awaitResetTokenRequest()
	assert.Equal(c.t, runnerID, event.runner.ID)
	c.resetRunnerTokenResponseChan <- response
}

func (c *resetRunnerTokenTestController) pushToWaitGroup(callback func()) {
	c.waitGroup.Add(1)
	go func() {
		callback()
		c.waitGroup.Done()
	}()
}

func (c *resetRunnerTokenTestController) stop() {
	c.runCommand.stopSignal = os.Interrupt
	close(c.runCommand.runFinished)
}

type resetRunnerTokenTestCase struct {
	runners       []common.RunnerCredentials
	testProcedure func(d *resetRunnerTokenTestController)
}

func TestRunCommand_resetOneRunnerToken(t *testing.T) {
	testCases := map[string]resetRunnerTokenTestCase{
		"no runners stop": {
			runners: []common.RunnerCredentials{},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.False(t, d.runCommand.resetOneRunnerToken())
					close(d.eventChan)
				})
				d.stop()
				d.waitGroup.Wait()
			},
		},
		"no runners reload config": {
			runners: []common.RunnerCredentials{},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
					close(d.eventChan)
				})
				d.runCommand.configReloaded <- 1
				d.waitGroup.Wait()
			},
		},
		"one expiring runner": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
					close(d.eventChan)
				})
				d.handleRunAtRequest(time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(1, &common.ResetTokenResponse{
					Token:           "token2",
					TokenObtainedAt: time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 11, 0, 0, 0, 0, time.UTC),
				})
				d.waitGroup.Wait()

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token2", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 11, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
			},
		},
		"one non-expiring runner": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.False(t, d.runCommand.resetOneRunnerToken())
					close(d.eventChan)
				})
				d.stop()
				d.waitGroup.Wait()
			},
		},
		"two expiring runners": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1_1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC),
				},
				{
					ID:              2,
					Token:           "token2_1",
					TokenObtainedAt: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 10, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
				})
				d.handleRunAtRequest(time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(1, &common.ResetTokenResponse{
					Token:           "token1_2",
					TokenObtainedAt: time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 11, 0, 0, 0, 0, time.UTC),
				})
				d.waitGroup.Wait()

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token1_2", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 11, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)

				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
					close(d.eventChan)
				})
				d.handleRunAtRequest(time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(2, &common.ResetTokenResponse{
					Token:           "token2_2",
					TokenObtainedAt: time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 12, 0, 0, 0, 0, time.UTC),
				})
				d.waitGroup.Wait()

				runner = d.runCommand.config.Runners[1]
				assert.Equal(t, "token2_2", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 12, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
			},
		},
		"one expiring, one non-expiring runner": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1_1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					ID:              2,
					Token:           "token2_1",
					TokenObtainedAt: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 10, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
					close(d.eventChan)
				})
				d.handleRunAtRequest(time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(2, &common.ResetTokenResponse{
					Token:           "token2_2",
					TokenObtainedAt: time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 12, 0, 0, 0, 0, time.UTC),
				})
				d.waitGroup.Wait()

				runner := d.runCommand.config.Runners[1]
				assert.Equal(t, "token2_2", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 12, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
			},
		},
		"one expiring runner stop": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.False(t, d.runCommand.resetOneRunnerToken())
					close(d.eventChan)
				})
				event := d.awaitRunAtRequest()
				assert.Equal(t, time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC), event.time)
				d.stop()
				d.waitGroup.Wait()
				assert.True(t, event.task.cancelled)
				assert.False(t, event.task.finished)
			},
		},
		"one expiring runner reload config": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
					close(d.eventChan)
				})
				event := d.awaitRunAtRequest()
				assert.Equal(t, time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC), event.time)
				d.runCommand.configReloaded <- 1
				d.waitGroup.Wait()
				assert.True(t, event.task.cancelled)
				assert.False(t, event.task.finished)
			},
		},
		"one expiring runner rewrite and reload config": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
				})

				config := common.NewConfig()
				config.Runners = []*common.RunnerConfig{
					{
						RunnerCredentials: common.RunnerCredentials{
							ID:              1,
							Token:           "token2",
							TokenObtainedAt: time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC),
							TokenExpiresAt:  time.Date(2022, 1, 16, 0, 0, 0, 0, time.UTC),
						},
					},
				}

				event := d.awaitRunAtRequest()
				assert.Equal(t, time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC), event.time)

				d.runCommand.configMutex.Lock()
				d.runCommand.config = config
				d.runCommand.configMutex.Unlock()
				d.runCommand.configReloaded <- 1
				d.waitGroup.Wait()
				assert.True(t, event.task.cancelled)
				assert.False(t, event.task.finished)

				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
					close(d.eventChan)
				})
				d.handleRunAtRequest(time.Date(2022, 1, 14, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(1, &common.ResetTokenResponse{
					Token:           "token3",
					TokenObtainedAt: time.Date(2022, 1, 14, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 22, 0, 0, 0, 0, time.UTC),
				})
				d.waitGroup.Wait()
			},
		},
		"one expiring runner rewrite and reload config race condition": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
					close(d.eventChan)
				})

				config := common.NewConfig()
				config.Runners = []*common.RunnerConfig{
					{
						RunnerCredentials: common.RunnerCredentials{
							ID:              1,
							Token:           "token2",
							TokenObtainedAt: time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC),
							TokenExpiresAt:  time.Date(2022, 1, 16, 0, 0, 0, 0, time.UTC),
						},
					},
				}
				d.runCommand.configMutex.Lock()
				d.runCommand.config = config
				d.runCommand.configMutex.Unlock()
				event := d.awaitRunAtRequest()
				d.runCommand.configReloaded <- 1
				d.waitGroup.Wait()
				assert.True(t, event.task.cancelled)
				assert.False(t, event.task.finished)
			},
		},
		"one expiring runner error": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
					close(d.eventChan)
				})
				d.handleRunAtRequest(time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(1, nil)
				d.waitGroup.Wait()

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token1", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			d := makeResetRunnerTokenTestController(t)
			for _, runner := range tc.runners {
				d.runCommand.config.Runners = append(
					d.runCommand.config.Runners,
					&common.RunnerConfig{
						RunnerCredentials: runner,
					})
			}

			tc.testProcedure(d)
			_, res := <-d.eventChan
			require.False(t, res)
		})
	}
}

func TestRunCommand_resetRunnerTokens(t *testing.T) {
	testCases := map[string]resetRunnerTokenTestCase{
		"one non-expiring runner": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					d.runCommand.resetRunnerTokens()
					close(d.eventChan)
				})

				d.stop()
				d.waitGroup.Wait()
			},
		},
		"one expiring runner": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 17, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					d.runCommand.resetRunnerTokens()
					close(d.eventChan)
				})

				event := d.awaitRunAtRequest()

				d.stop()
				d.waitGroup.Wait()

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token1", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 17, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
				assert.True(t, event.task.cancelled)
				assert.False(t, event.task.finished)
			},
		},
		"one expiring runner with non-expiring response": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 17, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					d.runCommand.resetRunnerTokens()
					close(d.eventChan)
				})

				d.handleRunAtRequest(time.Date(2022, 1, 13, 0, 0, 0, 0, time.UTC))
				d.stop()
				d.handleResetTokenRequest(1, &common.ResetTokenResponse{
					Token:           "token2",
					TokenObtainedAt: time.Date(2022, 1, 13, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
				})
				d.waitGroup.Wait()

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token2", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 13, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
			},
		},
		"one expiring runner with expiring response": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 17, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					d.runCommand.resetRunnerTokens()
					close(d.eventChan)
				})

				d.handleRunAtRequest(time.Date(2022, 1, 13, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(1, &common.ResetTokenResponse{
					Token:           "token2",
					TokenObtainedAt: time.Date(2022, 1, 13, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 17, 0, 0, 0, 0, time.UTC),
				})

				event := d.awaitRunAtRequest()

				d.stop()
				d.waitGroup.Wait()

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token2", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 13, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 17, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
				assert.True(t, event.task.cancelled)
				assert.False(t, event.task.finished)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			d := makeResetRunnerTokenTestController(t)
			for _, runner := range tc.runners {
				d.runCommand.config.Runners = append(
					d.runCommand.config.Runners,
					&common.RunnerConfig{
						RunnerCredentials: runner,
					})
			}

			tc.testProcedure(d)
			_, res := <-d.eventChan
			require.False(t, res)
		})
	}
}

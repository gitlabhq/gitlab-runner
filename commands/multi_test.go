//go:build !integration

package commands

import (
	"context"
	"io"
	"os"
	"os/signal"
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
	logrus.SetOutput(io.Discard)

	cfg := common.RunnerConfig{
		Limit:              2,
		RequestConcurrency: 10,
		RunnerSettings: common.RunnerSettings{
			Executor: "multi-runner-build-limit",
		},
		SystemIDState: common.NewSystemIDState(),
	}

	require.NoError(t, cfg.SystemIDState.EnsureSystemID())

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
	mJobTrace.On("SetDebugModeEnabled", mock.Anything)
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

type runAtCall struct {
	time     time.Time
	callback func()
	task     *runAtTaskMock
}

type resetTokenRequest struct {
	runner   common.RunnerCredentials
	systemID string
}

type resetRunnerTokenTestController struct {
	runCommand RunCommand
	eventChan  chan interface{}
	waitGroup  sync.WaitGroup

	networkMock     *common.MockNetwork
	configSaverMock *common.MockConfigSaver
}

type runAtTaskMock struct {
	finished  bool
	cancelled bool
}

func (t *runAtTaskMock) cancel() {
	t.cancelled = true
}

func newResetRunnerTokenTestController() *resetRunnerTokenTestController {
	networkMock := new(common.MockNetwork)
	configSaverMock := new(common.MockConfigSaver)

	data := &resetRunnerTokenTestController{
		runCommand: RunCommand{
			configOptionsWithListenAddress: configOptionsWithListenAddress{
				configOptions: configOptions{
					config: common.NewConfigWithSaver(configSaverMock),
				},
			},
			runAt:          runAt,
			runFinished:    make(chan bool),
			configReloaded: make(chan int),
			network:        networkMock,
		},
		eventChan:       make(chan interface{}),
		networkMock:     networkMock,
		configSaverMock: configSaverMock,
	}
	data.runCommand.runAt = data.runAt

	return data
}

// runAt implements the RunCommand.runAt interface and allows to integrate the call
// done in context of token resetting with the test implementation
func (c *resetRunnerTokenTestController) runAt(time time.Time, callback func()) runAtTask {
	task := runAtTaskMock{
		finished: false,
	}
	c.eventChan <- runAtCall{
		time:     time,
		callback: callback,
		task:     &task,
	}

	return &task
}

// mockResetToken should be run before the tested method call to ensure
// that API call is properly mocked, required and feeds data needed for
// further assertions
//
// Use only when this API call is expected. Otherwise - check assertResetTokenNotCalled
func (c *resetRunnerTokenTestController) mockResetToken(runnerID int64, response *common.ResetTokenResponse) {
	c.networkMock.
		On(
			"ResetToken",
			mock.MatchedBy(func(runner common.RunnerCredentials) bool {
				return runnerID == runner.ID
			}),
			unknownSystemID,
		).
		Return(func(runner common.RunnerCredentials, systemID string) *common.ResetTokenResponse {
			// Sending is a blocking operation, so this blocks until the other thread receives it.
			c.eventChan <- resetTokenRequest{
				runner:   runner,
				systemID: systemID,
			}

			return response
		}).
		Once()
}

// mockConfigSave should be run before the tested method call to ensure
// that configuration file save call is required
//
// Use only when save is expected. Otherwise - check assertConfigSaveNotCalled
func (c *resetRunnerTokenTestController) mockConfigSave() {
	c.configSaverMock.On("Save", "", mock.Anything).Return(nil).Once()
}

// awaitRunAtCall blocks on waiting for the RunCommand.runAt call (in context of token
// resetting) to happen
//
// Returns details about the call for further assertions
func (c *resetRunnerTokenTestController) awaitRunAtCall(t *testing.T) runAtCall {
	event := <-c.eventChan
	e := event.(runAtCall)
	require.NotNil(t, e)

	return e
}

// awaitResetTokenRequest blocks on waiting for the mocked API call for the token reset
// to happen
//
// Returns reset token request details for further assertions
func (c *resetRunnerTokenTestController) awaitResetTokenRequest(t *testing.T) resetTokenRequest {
	event := <-c.eventChan
	e := event.(resetTokenRequest)
	require.NotNil(t, e)

	return e
}

// handleRunAtCall asserts whether the call is the expected one and if yes - executed
// the callback registered for it (so in this case - the call that schedules another
// request for the token reset API)
func (c *resetRunnerTokenTestController) handleRunAtCall(t *testing.T, time time.Time) {
	event := c.awaitRunAtCall(t)
	assert.Equal(t, time, event.time)
	event.callback()
	event.task.finished = true
}

// handleResetTokenRequest asserts whether the request to the API is the one expected
// (basing on the ID and systemID of the Runner)
func (c *resetRunnerTokenTestController) handleResetTokenRequest(t *testing.T, runnerID int64, systemID string) {
	event := c.awaitResetTokenRequest(t)
	assert.Equal(t, runnerID, event.runner.ID)
	assert.Equal(t, systemID, event.systemID)
}

// pushToWaitGroup ensures that the callback function is executed in context
// of a WaitGroup. This allows use to organise the test case flow to be executed
// in the expected order
func (c *resetRunnerTokenTestController) pushToWaitGroup(callback func()) {
	c.waitGroup.Add(1)
	go func() {
		callback()
		c.waitGroup.Done()
	}()
}

// stop simulates RunCommand interruption - the moment when run() is finished
func (c *resetRunnerTokenTestController) stop() {
	c.runCommand.stopSignal = os.Interrupt
	close(c.runCommand.runFinished)
}

// reloadConfig simulates that configuration file update was discovered and that
// it was reloaded (which normally is done by RunCommand in background)
func (c *resetRunnerTokenTestController) reloadConfig() {
	c.runCommand.configReloaded <- 1
}

// setRunners updates the test configuration with given runner credentials.
//
// It should be used as the test case initialisation and may be used to simulate
// config change after reloading
func (c *resetRunnerTokenTestController) setRunners(runners []common.RunnerCredentials) {
	c.runCommand.configMutex.Lock()
	defer c.runCommand.configMutex.Unlock()

	configs := make([]*common.RunnerConfig, 0)

	for _, runner := range runners {
		configs = append(
			configs,
			&common.RunnerConfig{
				RunnerCredentials: runner,
			})
	}
	c.runCommand.config.Runners = configs
}

// wait stops execution until callbacks added currently to the WaitGroup
// are done
func (c *resetRunnerTokenTestController) wait() {
	c.waitGroup.Wait()
}

// finish ensures that channels used by the controller are closed
func (c *resetRunnerTokenTestController) finish() {
	close(c.eventChan)
}

// assertExpectations should be run in defer after creating the controller. If the
// tracked mocks have any command call expectations set, this will ensure to check
// if they were called
func (c *resetRunnerTokenTestController) assertExpectations(t *testing.T) {
	c.networkMock.AssertExpectations(t)
	c.configSaverMock.AssertExpectations(t)
}

// assertConfigSaveNotCalled should be run after the tested method call to ensure
// that configuration saving event was not executed
//
// Use only when configuration save is not expected. Otherwise - check mockConfigSave
func (c *resetRunnerTokenTestController) assertConfigSaveNotCalled(t *testing.T) {
	c.configSaverMock.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

// assertResetTokenNotCalled should be run after the tested method call to ensure
// that the network call to token reset API was not executed
//
// Use only when API call for token reset is not expected. Otherwise - check mockResetToken
func (c *resetRunnerTokenTestController) assertResetTokenNotCalled(t *testing.T) {
	c.networkMock.AssertNotCalled(t, "ResetToken", mock.Anything, mock.Anything)
}

type resetRunnerTokenTestCase struct {
	runners       []common.RunnerCredentials
	testProcedure func(t *testing.T, d *resetRunnerTokenTestController)
}

func TestRunCommand_resetOneRunnerToken(t *testing.T) {
	testCases := map[string]resetRunnerTokenTestCase{
		"no runners stop": {
			runners: []common.RunnerCredentials{},
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.False(t, d.runCommand.resetOneRunnerToken())
					d.assertResetTokenNotCalled(t)
					d.assertConfigSaveNotCalled(t)
				})
				d.stop()
				d.wait()
			},
		},
		"no runners reload config": {
			runners: []common.RunnerCredentials{},
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
					d.assertResetTokenNotCalled(t)
					d.assertConfigSaveNotCalled(t)
				})
				d.reloadConfig()
				d.wait()
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
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					d.mockResetToken(1, &common.ResetTokenResponse{
						Token:           "token2",
						TokenObtainedAt: time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC),
						TokenExpiresAt:  time.Date(2022, 1, 11, 0, 0, 0, 0, time.UTC),
					})
					d.mockConfigSave()
					assert.True(t, d.runCommand.resetOneRunnerToken())
				})
				d.handleRunAtCall(t, time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(t, 1, unknownSystemID)
				d.wait()

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
					// 0001-01-01T00:00:00.0 is the "zero" value of time.Time and is used
					// by resetting mechanism to recognize runners that don't have expiration time assigned
					TokenExpiresAt: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.False(t, d.runCommand.resetOneRunnerToken())
					d.assertResetTokenNotCalled(t)
					d.assertConfigSaveNotCalled(t)
				})
				d.stop()
				d.wait()
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
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					d.mockResetToken(1, &common.ResetTokenResponse{
						Token:           "token1_2",
						TokenObtainedAt: time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC),
						TokenExpiresAt:  time.Date(2022, 1, 11, 0, 0, 0, 0, time.UTC),
					})
					d.mockConfigSave()
					assert.True(t, d.runCommand.resetOneRunnerToken())
				})
				d.handleRunAtCall(t, time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(t, 1, unknownSystemID)
				d.wait()

				d.pushToWaitGroup(func() {
					d.mockResetToken(2, &common.ResetTokenResponse{
						Token:           "token2_2",
						TokenObtainedAt: time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC),
						TokenExpiresAt:  time.Date(2022, 1, 12, 0, 0, 0, 0, time.UTC),
					})
					d.mockConfigSave()
					assert.True(t, d.runCommand.resetOneRunnerToken())
				})
				d.handleRunAtCall(t, time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(t, 2, unknownSystemID)
				d.wait()

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token1_2", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 11, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)

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
					// 0001-01-01T00:00:00.0 is the "zero" value of time.Time and is used
					// by resetting mechanism to recognize runners that don't have expiration time assigned
					TokenExpiresAt: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					ID:              2,
					Token:           "token2_1",
					TokenObtainedAt: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 10, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					d.mockResetToken(2, &common.ResetTokenResponse{
						Token:           "token2_2",
						TokenObtainedAt: time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC),
						TokenExpiresAt:  time.Date(2022, 1, 12, 0, 0, 0, 0, time.UTC),
					})
					d.mockConfigSave()
					assert.True(t, d.runCommand.resetOneRunnerToken())
				})
				d.handleRunAtCall(t, time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(t, 2, unknownSystemID)
				d.wait()

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token1_1", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)

				runner = d.runCommand.config.Runners[1]
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
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.False(t, d.runCommand.resetOneRunnerToken())
					d.assertResetTokenNotCalled(t)
					d.assertConfigSaveNotCalled(t)
				})

				event := d.awaitRunAtCall(t)

				assert.Equal(t, time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC), event.time)

				d.stop()
				d.wait()

				assert.True(t, event.task.cancelled)
				assert.False(t, event.task.finished)

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token1", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
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
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
					d.assertResetTokenNotCalled(t)
					d.assertConfigSaveNotCalled(t)
				})

				event := d.awaitRunAtCall(t)

				assert.Equal(t, time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC), event.time)

				d.reloadConfig()
				d.wait()

				assert.True(t, event.task.cancelled)
				assert.False(t, event.task.finished)

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token1", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
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
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
					d.assertResetTokenNotCalled(t)
					d.assertConfigSaveNotCalled(t)
				})

				event := d.awaitRunAtCall(t)

				assert.Equal(t, time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC), event.time)

				d.setRunners([]common.RunnerCredentials{
					{
						ID:              1,
						Token:           "token2",
						TokenObtainedAt: time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC),
						TokenExpiresAt:  time.Date(2022, 1, 16, 0, 0, 0, 0, time.UTC),
					},
				})
				d.reloadConfig()
				d.wait()

				assert.True(t, event.task.cancelled)
				assert.False(t, event.task.finished)

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token2", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 16, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)

				d.pushToWaitGroup(func() {
					d.mockResetToken(1, &common.ResetTokenResponse{
						Token:           "token3",
						TokenObtainedAt: time.Date(2022, 1, 14, 0, 0, 0, 0, time.UTC),
						TokenExpiresAt:  time.Date(2022, 1, 22, 0, 0, 0, 0, time.UTC),
					})
					d.mockConfigSave()
					assert.True(t, d.runCommand.resetOneRunnerToken())
				})
				d.handleRunAtCall(t, time.Date(2022, 1, 14, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(t, 1, unknownSystemID)
				d.wait()

				runner = d.runCommand.config.Runners[0]
				assert.Equal(t, "token3", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 14, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 22, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
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
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					assert.True(t, d.runCommand.resetOneRunnerToken())
					d.assertResetTokenNotCalled(t)
					d.assertConfigSaveNotCalled(t)
				})

				d.setRunners([]common.RunnerCredentials{
					{
						ID:              1,
						Token:           "token2",
						TokenObtainedAt: time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC),
						TokenExpiresAt:  time.Date(2022, 1, 16, 0, 0, 0, 0, time.UTC),
					},
				})

				event := d.awaitRunAtCall(t)

				d.reloadConfig()
				d.wait()

				assert.True(t, event.task.cancelled)
				assert.False(t, event.task.finished)

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token2", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 8, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 16, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
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
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					d.mockResetToken(1, nil)
					assert.True(t, d.runCommand.resetOneRunnerToken())
					d.assertConfigSaveNotCalled(t)
				})
				d.handleRunAtCall(t, time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(t, 1, unknownSystemID)
				d.wait()

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token1", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			d := newResetRunnerTokenTestController()
			defer d.assertExpectations(t)

			d.setRunners(tc.runners)
			tc.testProcedure(t, d)
			d.finish()
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
					// 0001-01-01T00:00:00.0 is the "zero" value of time.Time and is used
					// by resetting mechanism to recognize runners that don't have expiration time assigned
					TokenExpiresAt: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					d.runCommand.resetRunnerTokens()
					d.assertResetTokenNotCalled(t)
					d.assertConfigSaveNotCalled(t)
				})

				d.stop()
				d.wait()
			},
		},
		"one expiring runner stop": {
			runners: []common.RunnerCredentials{
				{
					ID:              1,
					Token:           "token1",
					TokenObtainedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					TokenExpiresAt:  time.Date(2022, 1, 17, 0, 0, 0, 0, time.UTC),
				},
			},
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					d.runCommand.resetRunnerTokens()
					d.assertResetTokenNotCalled(t)
					d.assertConfigSaveNotCalled(t)
				})

				event := d.awaitRunAtCall(t)

				d.stop()
				d.wait()

				assert.True(t, event.task.cancelled)
				assert.False(t, event.task.finished)

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token1", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 17, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
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
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					d.mockResetToken(1, &common.ResetTokenResponse{
						Token:           "token2",
						TokenObtainedAt: time.Date(2022, 1, 13, 0, 0, 0, 0, time.UTC),
						TokenExpiresAt:  time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
					})
					d.mockConfigSave()
					d.runCommand.resetRunnerTokens()
				})

				d.handleRunAtCall(t, time.Date(2022, 1, 13, 0, 0, 0, 0, time.UTC))
				d.stop()
				d.handleResetTokenRequest(t, 1, unknownSystemID)
				d.wait()

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
			testProcedure: func(t *testing.T, d *resetRunnerTokenTestController) {
				d.pushToWaitGroup(func() {
					d.mockResetToken(1, &common.ResetTokenResponse{
						Token:           "token2",
						TokenObtainedAt: time.Date(2022, 1, 13, 0, 0, 0, 0, time.UTC),
						TokenExpiresAt:  time.Date(2022, 1, 17, 0, 0, 0, 0, time.UTC),
					})
					d.mockConfigSave()
					d.runCommand.resetRunnerTokens()
				})

				d.handleRunAtCall(t, time.Date(2022, 1, 13, 0, 0, 0, 0, time.UTC))
				d.handleResetTokenRequest(t, 1, unknownSystemID)

				event := d.awaitRunAtCall(t)

				d.stop()
				d.wait()

				assert.True(t, event.task.cancelled)
				assert.False(t, event.task.finished)

				runner := d.runCommand.config.Runners[0]
				assert.Equal(t, "token2", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 13, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 17, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			d := newResetRunnerTokenTestController()
			defer d.assertExpectations(t)

			d.setRunners(tc.runners)
			tc.testProcedure(t, d)
			d.finish()
		})
	}
}

func TestRunCommand_configReloadingRegression(t *testing.T) {
	// Main context to handle go test interrupts and to close all the utility loops in this test function
	nCtx, nCancelFn := signal.NotifyContext(context.Background(), os.Interrupt)
	defer nCancelFn()

	// Preparing fake configuration file
	configFile, err := os.CreateTemp("", "config-reload-test")
	require.NoError(t, err)
	err = configFile.Close()
	require.NoError(t, err)

	c := &RunCommand{
		configOptionsWithListenAddress: configOptionsWithListenAddress{
			configOptions: configOptions{
				ConfigFile: configFile.Name(),
			},
		},
		runInterruptSignal:   make(chan os.Signal, 1),
		reloadSignal:         make(chan os.Signal, 1),
		configReloaded:       make(chan int, 1),
		reloadConfigInterval: 5 * time.Millisecond,
	}

	// Counting discovered configuration reloads
	configReloadedCount := 0
	go func() {
		for {
			select {
			case <-nCtx.Done():
				return
			case <-c.configReloaded:
				configReloadedCount++
			}
		}
	}()

	// Loading the configuration file and creating config object (as until now it's undefined in c
	// This is the first expected reload to happen.
	err = c.reloadConfig()
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)

	c.config.Runners = append(c.config.Runners, &common.RunnerConfig{
		Name: "config-reload-test-runner",
	})

	// Reloading the configuration. This is the reload call that normally would happen at start when
	// executing the gitlab-runner run command.
	// This is the second expected reload to happen.
	err = c.reloadConfig()
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)

	// Additional context to limit the time of test function execution
	ctx, cancelFn := context.WithTimeout(nCtx, 3*time.Second)
	defer cancelFn()

	go func() {
		time.Sleep(1 * time.Second)

		timeNow := time.Now()

		// Delayed external change to the modification time of the configuration file.
		// This is to trigger the discovery of configuration changes and automatic reload.
		// This is the third expected configuration reload.
		err = os.Chtimes(configFile.Name(), timeNow, timeNow)
		require.NoError(t, err)
	}()

	// Pushing interrupt signal to RunCommand to stop the updateConfig loop bellow
	go func() {
		<-ctx.Done()
		for {
			c.runInterruptSignal <- os.Interrupt
		}
	}()

	// Simulating the updateConfig loop that normally happens inside RunCommand execution
	var signaled os.Signal
	for signaled == nil {
		signaled = c.updateConfig()
	}

	assert.Equal(t, 3, configReloadedCount, "Exceeded the expected number of configReload calls")
}

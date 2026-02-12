//go:build !integration

package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/commands/internal/configfile"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	helper_test "gitlab.com/gitlab-org/gitlab-runner/helpers/test"
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
	}

	mJobTrace := common.NewMockLightJobTrace(t)
	mJobTrace.On("SetFailuresCollector", mock.Anything)
	mJobTrace.On("IsStdout").Return(false)
	mJobTrace.On("SetCancelFunc", mock.Anything)
	mJobTrace.On("SetAbortFunc", mock.Anything)
	mJobTrace.On("SetDebugModeEnabled", mock.Anything)
	mJobTrace.On("Success").Return(nil)

	mNetwork := common.NewMockNetwork(t)
	mNetwork.On("RequestJob", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, config common.RunnerConfig, sessionInfo *common.SessionInfo) (*spec.Job, bool) {
		return &spec.Job{
			ID: 1,
			Steps: []spec.Step{
				{
					Name:         "sleep",
					Script:       spec.StepScript{"sleep 10"},
					Timeout:      15,
					When:         "",
					AllowFailure: false,
				},
			},
		}, true
	})
	mNetwork.On("UpdateJob", mock.Anything, mock.Anything, mock.Anything).Return(common.UpdateJobResult{State: common.UpdateSucceeded})
	mNetwork.On("ProcessJob", mock.Anything, mock.Anything).Return(mJobTrace, nil)

	var runningBuilds uint32
	e := common.NewMockExecutor(t)
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("Cleanup").Maybe()
	e.On("Shell").Return(&common.ShellScriptInfo{Shell: "script-shell"})
	e.On("Finish", mock.Anything).Maybe()
	e.On("Run", mock.Anything).Run(func(args mock.Arguments) {
		atomic.AddUint32(&runningBuilds, 1)

		// Simulate work to fill up build queue.
		time.Sleep(100 * time.Millisecond)
	}).Return(nil)

	p := common.NewMockExecutorProvider(t)
	p.On("Acquire", mock.Anything).Return(nil, nil)
	p.On("Release", mock.Anything, mock.Anything).Return(nil).Maybe()
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil)
	p.On("Create").Return(e)

	common.RegisterExecutorProviderForTest(t, "multi-runner-build-limit", p)

	cmd := RunCommand{
		network:      mNetwork,
		buildsHelper: newBuildsHelper(),
		configfile: configfile.New("", configfile.WithExistingConfig(
			&common.Config{User: "git"},
		), configfile.WithSystemID(common.UnknownSystemID)),
	}

	runners := make(chan *common.RunnerConfig)

	cmd.buildsHelper.getRunnerCounter(&cfg).adaptiveConcurrencyLimit = 100

	// Start concurrent jobs
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
	returnedJob := new(spec.Job)

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

			network := common.NewMockNetwork(t)
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

			ctx, cancelFn := context.WithTimeout(t.Context(), 1*time.Second)
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
	runner   common.RunnerConfig
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

func newResetRunnerTokenTestController(t *testing.T) *resetRunnerTokenTestController {
	networkMock := common.NewMockNetwork(t)
	configSaverMock := common.NewMockConfigSaver(t)

	configPath := filepath.Join(t.TempDir(), "config.toml")

	data := &resetRunnerTokenTestController{
		runCommand: RunCommand{
			configfile: configfile.New(configPath, configfile.WithExistingConfig(
				common.NewConfigWithSaver(configSaverMock),
			), configfile.WithSystemID(common.UnknownSystemID)),
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
			mock.MatchedBy(func(runner common.RunnerConfig) bool {
				return runnerID == runner.ID
			}),
			common.UnknownSystemID,
		).
		Return(func(runner common.RunnerConfig, systemID string) *common.ResetTokenResponse {
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
	c.configSaverMock.On("Save", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		_ = os.WriteFile(args.Get(0).(string), args.Get(1).([]byte), 0o600)
	}).Return(nil).Once()
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
//
//nolint:unparam
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
	_ = c.runCommand.configfile.Load(configfile.WithMutateOnLoad(func(cfg *common.Config) error {
		var set []*common.RunnerConfig

		for _, runner := range runners {
			set = append(set, &common.RunnerConfig{
				RunnerCredentials: runner,
			})
		}

		cfg.Runners = set

		return nil
	}))

	// silently save changes to disk without going via mock
	saver := c.runCommand.configfile.Config().ConfigSaver
	c.runCommand.configfile.Config().ConfigSaver = nil
	defer func() {
		c.runCommand.configfile.Config().ConfigSaver = saver
	}()
	_ = c.runCommand.configfile.Save()
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
				d.handleResetTokenRequest(t, 1, common.UnknownSystemID)
				d.wait()

				runner := d.runCommand.configfile.Config().Runners[0]
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
				d.handleResetTokenRequest(t, 1, common.UnknownSystemID)
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
				d.handleResetTokenRequest(t, 2, common.UnknownSystemID)
				d.wait()

				runner := d.runCommand.configfile.Config().Runners[0]
				assert.Equal(t, "token1_2", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 7, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 11, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)

				runner = d.runCommand.configfile.Config().Runners[1]
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
				d.handleResetTokenRequest(t, 2, common.UnknownSystemID)
				d.wait()

				runner := d.runCommand.configfile.Config().Runners[0]
				assert.Equal(t, "token1_1", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)

				runner = d.runCommand.configfile.Config().Runners[1]
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

				runner := d.runCommand.configfile.Config().Runners[0]
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

				runner := d.runCommand.configfile.Config().Runners[0]
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

				runner := d.runCommand.configfile.Config().Runners[0]
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
				d.handleResetTokenRequest(t, 1, common.UnknownSystemID)
				d.wait()

				runner = d.runCommand.configfile.Config().Runners[0]
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

				runner := d.runCommand.configfile.Config().Runners[0]
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
				d.handleResetTokenRequest(t, 1, common.UnknownSystemID)
				d.wait()

				runner := d.runCommand.configfile.Config().Runners[0]
				assert.Equal(t, "token1", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 9, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			d := newResetRunnerTokenTestController(t)

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

				runner := d.runCommand.configfile.Config().Runners[0]
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
				d.handleResetTokenRequest(t, 1, common.UnknownSystemID)
				d.wait()

				runner := d.runCommand.configfile.Config().Runners[0]
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
				d.handleResetTokenRequest(t, 1, common.UnknownSystemID)

				event := d.awaitRunAtCall(t)

				d.stop()
				d.wait()

				assert.True(t, event.task.cancelled)
				assert.False(t, event.task.finished)

				runner := d.runCommand.configfile.Config().Runners[0]
				assert.Equal(t, "token2", runner.Token)
				assert.Equal(t, time.Date(2022, 1, 13, 0, 0, 0, 0, time.UTC), runner.TokenObtainedAt)
				assert.Equal(t, time.Date(2022, 1, 17, 0, 0, 0, 0, time.UTC), runner.TokenExpiresAt)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			d := newResetRunnerTokenTestController(t)

			d.setRunners(tc.runners)

			tc.testProcedure(t, d)
			d.finish()
		})
	}
}

func TestRunCommand_configReloadingRegression(t *testing.T) {
	// fake config
	configName := filepath.Join(t.TempDir(), "config-reload-test")
	require.NoError(t, os.WriteFile(configName, nil, 0o777))

	c := &RunCommand{
		ConfigFile:           configName,
		configfile:           configfile.New(configName),
		runInterruptSignal:   make(chan os.Signal, 1),
		reloadSignal:         make(chan os.Signal, 1),
		configReloaded:       make(chan int, 1),
		reloadConfigInterval: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// Counting discovered configuration reloads
	var configReloadedCount atomic.Int64
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(done)
				return
			case <-c.configReloaded:
				configReloadedCount.Add(1)
			default:
				c.updateConfig()
			}
		}
	}()

	// force reload twice
	require.NoError(t, c.reloadConfig())
	require.NoError(t, c.reloadConfig())

	// trigger automatic reload (by changing time of config file) and wait
	update := time.Now().Add(time.Second)
	require.NoError(t, os.Chtimes(configName, update, update))

	// sleep for 5 times the reload config interval to make sure we don't reload
	// more than we should
	time.Sleep(c.reloadConfigInterval * 5)

	cancel()
	for len(c.configReloaded) > 0 {
		<-c.configReloaded
		configReloadedCount.Add(1)
	}
	<-done

	assert.Equal(t, int64(3), configReloadedCount.Load())
}

func TestRunCommand_configReloading(t *testing.T) {
	// This test is flaky on Win21H2 platform
	// Skipping until https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37920 is resolved.
	helper_test.SkipIfGitLabCIOn(t, helper_test.OSWindows)

	_, cleanup := test.NewHook()
	defer cleanup()

	config := `concurrent = 1
check_interval = 1
log_level = "debug"
shutdown_timeout = 0`

	configChanged := `concurrent = 1
check_interval = 1
shutdown_timeout = 0`

	configName := filepath.Join(t.TempDir(), "config-reload-test")
	require.NoError(t, os.WriteFile(configName, []byte(config), 0o777))

	c := &RunCommand{
		ConfigFile:           configName,
		configfile:           configfile.New(configName),
		runInterruptSignal:   make(chan os.Signal, 1),
		reloadSignal:         make(chan os.Signal, 1),
		configReloaded:       make(chan int, 1),
		reloadConfigInterval: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// Counting discovered configuration reloads
	var configReloadedCount atomic.Int64
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				wg.Done()
				return
			case <-c.configReloaded:
				configReloadedCount.Add(1)
			default:
				c.updateConfig()
			}
		}
	}()

	// force reload twice
	require.NoError(t, c.reloadConfig())
	require.NoError(t, c.reloadConfig())

	// trigger automatic reload (by changing time of config file) and wait
	file, err := os.OpenFile(configName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o777)
	require.NoError(t, err)
	_, err = file.WriteString(configChanged)
	require.NoError(t, err)
	file.Close()

	// sleep for 15 times the reload config interval to make sure we don't reload
	// more than we should
	time.Sleep(c.reloadConfigInterval * 15)

	cancel()
	for len(c.configReloaded) > 0 {
		<-c.configReloaded
		configReloadedCount.Add(1)
	}

	wg.Wait()

	assert.Equal(t, "info", logrus.GetLevel().String())
	assert.Equal(t, int64(3), configReloadedCount.Load())
}

func TestListenAddress(t *testing.T) {
	type source string

	const (
		configurationFromCli    source = "from-cli"
		configurationFromConfig source = "from-config"
	)

	examples := map[string]struct {
		address         string
		setAddress      bool
		expectedAddress string
		errorIsExpected bool
	}{
		"address-set-without-port": {"localhost", true, "localhost:9252", false},
		"port-set-without-address": {":1234", true, ":1234", false},
		"address-set-with-port":    {"localhost:1234", true, "localhost:1234", false},
		"address-is-empty":         {"", true, "", false},
		"address-is-invalid":       {"localhost::1234", true, "", true},
		"address-not-set":          {"", false, "", false},
	}

	for exampleName, example := range examples {
		for _, testType := range []source{configurationFromCli, configurationFromConfig} {
			t.Run(fmt.Sprintf("%s-%s", exampleName, testType), func(t *testing.T) {
				cfg := &common.Config{}
				var address string

				if example.setAddress {
					if testType == configurationFromCli {
						address = example.address
					} else {
						cfg.ListenAddress = example.address
					}
				}

				address, err := listenAddress(cfg, address)
				assert.Equal(t, example.expectedAddress, address)
				if example.errorIsExpected {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	}
}

func TestRequestBottleneckWarning(t *testing.T) {
	tests := []struct {
		name             string
		config           *common.Config
		expectWarning    bool
		expectedWarnings []string // Specific warning messages to look for
		description      string
	}{
		{
			name: "worker_starvation",
			config: &common.Config{
				Concurrent: 2,
				Runners: []*common.RunnerConfig{
					{RunnerCredentials: common.RunnerCredentials{Token: "runner1"}},
					{RunnerCredentials: common.RunnerCredentials{Token: "runner2"}},
					{RunnerCredentials: common.RunnerCredentials{Token: "runner3"}},
				},
			},
			expectWarning:    true,
			expectedWarnings: []string{"Worker starvation bottleneck"},
			description:      "Should warn when concurrent < runners",
		},
		{
			name: "request_bottleneck",
			config: &common.Config{
				Concurrent: 4,
				Runners: []*common.RunnerConfig{
					{
						RequestConcurrency: 1,
						Limit:              10,
						RunnerCredentials:  common.RunnerCredentials{Token: "runner1"},
					},
					{
						RequestConcurrency: 1,
						Limit:              8,
						RunnerCredentials:  common.RunnerCredentials{Token: "runner2"},
					},
				},
			},
			expectWarning:    true,
			expectedWarnings: []string{"Request bottleneck"},
			description:      "Should warn about request bottleneck",
		},
		{
			name: "build_limit_saturation",
			config: &common.Config{
				Concurrent: 4,
				Runners: []*common.RunnerConfig{
					{
						Limit:              2,
						RequestConcurrency: 1,
						RunnerCredentials:  common.RunnerCredentials{Token: "runner1"},
					},
					{
						Limit:              1,
						RequestConcurrency: 1,
						RunnerCredentials:  common.RunnerCredentials{Token: "runner2"},
					},
				},
			},
			expectWarning:    true,
			expectedWarnings: []string{"Build limit bottleneck"},
			description:      "Should warn about build limit saturation",
		},
		{
			name: "multiple_scenarios",
			config: &common.Config{
				Concurrent: 4,
				Runners: []*common.RunnerConfig{
					{
						RequestConcurrency: 1,
						Limit:              2,
						RunnerCredentials:  common.RunnerCredentials{Token: "runner1"},
					},
					{
						RequestConcurrency: 1,
						Limit:              1,
						RunnerCredentials:  common.RunnerCredentials{Token: "runner2"},
					},
					{
						RequestConcurrency: 2,
						Limit:              5,
						RunnerCredentials:  common.RunnerCredentials{Token: "runner3"},
					},
				},
			},
			expectWarning:    true,
			expectedWarnings: []string{"Request bottleneck", "Build limit bottleneck"},
			description:      "Should warn about multiple issues",
		},
		{
			name: "healthy_configuration",
			config: &common.Config{
				Concurrent: 6,
				Runners: []*common.RunnerConfig{
					{
						RequestConcurrency: 3,
						Limit:              10,
						RunnerCredentials:  common.RunnerCredentials{Token: "runner1"},
					},
					{
						RequestConcurrency: 2,
						Limit:              5,
						RunnerCredentials:  common.RunnerCredentials{Token: "runner2"},
					},
				},
			},
			expectWarning:    false,
			expectedWarnings: nil,
			description:      "Should not warn for healthy configuration",
		},
		{
			name: "adequate_concurrent",
			config: &common.Config{
				Concurrent: 3,
				Runners: []*common.RunnerConfig{
					{
						RequestConcurrency: 2,
						RunnerCredentials:  common.RunnerCredentials{Token: "runner1"},
					},
					{
						RequestConcurrency: 2,
						RunnerCredentials:  common.RunnerCredentials{Token: "runner2"},
					},
					{
						RequestConcurrency: 2,
						RunnerCredentials:  common.RunnerCredentials{Token: "runner3"},
					},
				},
			},
			expectWarning:    false,
			expectedWarnings: nil,
			description:      "Should not warn when concurrent >= runners",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook, cleanup := test.NewHook()
			defer cleanup()

			logrus.SetLevel(logrus.WarnLevel)
			logrus.SetOutput(io.Discard)

			cmd := RunCommand{
				configfile: configfile.New("", configfile.WithExistingConfig(tt.config),
					configfile.WithSystemID(common.UnknownSystemID)),
			}

			cmd.checkConfigConcurrency(tt.config)

			foundMainWarning := false
			for _, entry := range hook.AllEntries() {
				if strings.Contains(entry.Message, "CONFIGURATION:") &&
					strings.Contains(entry.Message, "Long polling issues detected") {
					foundMainWarning = true
					break
				}
			}

			if !tt.expectWarning {
				assert.False(t, foundMainWarning, tt.description)
				return
			}

			assert.True(t, foundMainWarning, tt.description)

			for _, expectedWarning := range tt.expectedWarnings {
				foundSpecificWarning := false
				for _, entry := range hook.AllEntries() {
					if strings.Contains(entry.Message, expectedWarning) {
						foundSpecificWarning = true
						break
					}
				}
				assert.True(t, foundSpecificWarning, fmt.Sprintf("Should contain warning: %s", expectedWarning))
			}
		})
	}
}

func TestRunCommand_requestJob_HandlesUpdateAbort(t *testing.T) {
	runner := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-token",
		},
	}

	jobData := &spec.Job{
		ID:    123,
		Token: "job-token",
	}

	network := common.NewMockNetwork(t)
	mockTrace := common.NewMockJobTrace(t)
	mockTrace.On("SetFailuresCollector", mock.Anything).Return()
	mockTrace.On("Finish").Return()

	// Mock RequestJob to return a job
	network.On("RequestJob", mock.Anything, *runner, mock.Anything).Return(jobData, true)
	// Mock ProcessJob to return a trace
	network.On("ProcessJob", *runner, mock.AnythingOfType("*common.JobCredentials")).Return(mockTrace, nil)
	// Mock UpdateJob to return UpdateAbort
	network.On("UpdateJob", *runner, mock.AnythingOfType("*common.JobCredentials"), mock.AnythingOfType("common.UpdateJobInfo")).
		Return(common.UpdateJobResult{State: common.UpdateAbort})

	cmd := &RunCommand{
		network: network,
	}

	trace, response, err := cmd.requestJob(runner, nil)

	// When UpdateJob returns UpdateAbort, requestJob should return nil
	assert.Nil(t, trace, "Should return nil trace when update is aborted")
	assert.Nil(t, response, "Should return nil response when update is aborted")
	assert.Nil(t, err, "Should return nil error when update is aborted")

	network.AssertExpectations(t)
	mockTrace.AssertExpectations(t)
}

func TestRunCommand_requestJob_HandlesCancelRequested(t *testing.T) {
	runner := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-token",
		},
	}

	jobData := &spec.Job{
		ID:    123,
		Token: "job-token",
	}

	network := common.NewMockNetwork(t)
	mockTrace := common.NewMockJobTrace(t)
	mockTrace.On("SetFailuresCollector", mock.Anything).Return()
	mockTrace.On("Finish").Return()

	// Mock RequestJob to return a job
	network.On("RequestJob", mock.Anything, *runner, mock.Anything).Return(jobData, true)
	// Mock ProcessJob to return a trace
	network.On("ProcessJob", *runner, mock.AnythingOfType("*common.JobCredentials")).Return(mockTrace, nil)
	// Mock UpdateJob to return success but with CancelRequested=true
	network.On("UpdateJob", *runner, mock.AnythingOfType("*common.JobCredentials"), mock.AnythingOfType("common.UpdateJobInfo")).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded, CancelRequested: true})

	cmd := &RunCommand{
		network: network,
	}

	trace, response, err := cmd.requestJob(runner, nil)

	// When UpdateJob has CancelRequested=true, requestJob should return nil
	assert.Nil(t, trace, "Should return nil trace when job is being canceled")
	assert.Nil(t, response, "Should return nil response when job is being canceled")
	assert.Nil(t, err, "Should return nil error when job is being canceled")

	network.AssertExpectations(t)
	mockTrace.AssertExpectations(t)
}

func TestRunCommand_requestJob_ContinuesWhenUpdateSucceeds(t *testing.T) {
	runner := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-token",
		},
	}

	jobData := &spec.Job{
		ID:    123,
		Token: "job-token",
	}

	mockTrace := &common.MockJobTrace{}
	mockTrace.On("SetFailuresCollector", mock.Anything).Return()

	network := common.NewMockNetwork(t)
	// Mock RequestJob to return a job
	network.On("RequestJob", mock.Anything, *runner, mock.Anything).Return(jobData, true)
	// Mock UpdateJob to return success
	network.On("UpdateJob", *runner, mock.AnythingOfType("*common.JobCredentials"), mock.AnythingOfType("common.UpdateJobInfo")).
		Return(common.UpdateJobResult{State: common.UpdateSucceeded})
	// Mock ProcessJob to return a trace
	network.On("ProcessJob", *runner, mock.AnythingOfType("*common.JobCredentials")).Return(mockTrace, nil)

	cmd := &RunCommand{
		network: network,
	}

	trace, response, err := cmd.requestJob(runner, nil)

	// When UpdateJob succeeds, requestJob should continue and return the job
	assert.Equal(t, mockTrace, trace, "Should return the job trace when update succeeds")
	assert.Equal(t, jobData, response, "Should return the job response when update succeeds")
	assert.Nil(t, err, "Should return no error when update succeeds")

	network.AssertExpectations(t)
	mockTrace.AssertExpectations(t)
}

func TestRunCommand_requestJob_ReturnsNilWhenNoJob(t *testing.T) {
	runner := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-token",
		},
	}

	network := common.NewMockNetwork(t)
	// Mock RequestJob to return no job
	network.On("RequestJob", mock.Anything, *runner, mock.Anything).Return(nil, true)

	cmd := &RunCommand{
		network: network,
	}

	trace, response, err := cmd.requestJob(runner, nil)

	// When no job is available, requestJob should return nil without calling UpdateJob
	assert.Nil(t, trace, "Should return nil trace when no job available")
	assert.Nil(t, response, "Should return nil response when no job available")
	assert.Nil(t, err, "Should return nil error when no job available")

	network.AssertExpectations(t)
}

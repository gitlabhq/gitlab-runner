//go:build !integration

package shell

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

func TestExecutor_Run(t *testing.T) {
	var testErr = errors.New("test error")
	var exitErr = &exec.ExitError{}

	tests := map[string]struct {
		commanderAssertions     func(*process.MockCommander, chan time.Time)
		processKillerAssertions func(*process.MockKillWaiter, chan time.Time)
		cancelJob               bool
		expectedErr             error
	}{
		"canceled job uses new process termination": {
			commanderAssertions: func(mCmd *process.MockCommander, waitCalled chan time.Time) {
				mCmd.On("Start").Return(nil).Once()
				mCmd.On("Wait").Run(func(args mock.Arguments) {
					close(waitCalled)
				}).Return(nil).Once()
			},
			processKillerAssertions: func(mProcessKillWaiter *process.MockKillWaiter, waitCalled chan time.Time) {
				mProcessKillWaiter.
					On("KillAndWait", mock.Anything, mock.Anything).
					Return(nil).
					WaitUntil(waitCalled)
			},
			cancelJob:   true,
			expectedErr: nil,
		},
		"cmd fails to start": {
			commanderAssertions: func(mCmd *process.MockCommander, _ chan time.Time) {
				mCmd.On("Start").Return(testErr).Once()
			},
			processKillerAssertions: func(_ *process.MockKillWaiter, _ chan time.Time) {

			},
			expectedErr: testErr,
		},
		"wait returns error": {
			commanderAssertions: func(mCmd *process.MockCommander, waitCalled chan time.Time) {
				mCmd.On("Start").Return(nil).Once()
				mCmd.On("Wait").Run(func(args mock.Arguments) {
					close(waitCalled)
				}).Return(testErr).Once()
			},
			processKillerAssertions: func(mProcessKillWaiter *process.MockKillWaiter, waitCalled chan time.Time) {},
			cancelJob:               false,
			expectedErr:             testErr,
		},
		"wait returns exit error": {
			commanderAssertions: func(mCmd *process.MockCommander, waitCalled chan time.Time) {
				mCmd.On("Start").Return(nil).Once()
				mCmd.On("Wait").Run(func(args mock.Arguments) {
					close(waitCalled)
				}).Return(exitErr).Once()
			},
			processKillerAssertions: func(mProcessKillWaiter *process.MockKillWaiter, waitCalled chan time.Time) {},
			cancelJob:               false,
			expectedErr:             &common.BuildError{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			shellstest.OnEachShell(t, func(t *testing.T, shell string) {
				mProcessKillWaiter, mCmd, cleanup := setupProcessMocks(t)
				defer cleanup()

				waitCalled := make(chan time.Time)
				tt.commanderAssertions(mCmd, waitCalled)
				tt.processKillerAssertions(mProcessKillWaiter, waitCalled)

				executor := executor{
					AbstractExecutor: executors.AbstractExecutor{
						Build: &common.Build{
							Job:    spec.Job{},
							Runner: &common.RunnerConfig{},
						},
						BuildShell: &common.ShellConfiguration{
							Command: shell,
						},
					},
				}

				ctx, cancelJob := context.WithCancel(t.Context())
				defer cancelJob()

				cmd := common.ExecutorCommand{
					Script:     "echo hello",
					Predefined: false,
					Context:    ctx,
				}

				if tt.cancelJob {
					cancelJob()
				}

				err := executor.Run(cmd)
				assert.ErrorIs(t, err, tt.expectedErr)
			})
		})
	}
}

func setupProcessMocks(t *testing.T) (*process.MockKillWaiter, *process.MockCommander, func()) {
	mProcessKillWaiter := process.NewMockKillWaiter(t)
	mCmd := process.NewMockCommander(t)

	oldNewProcessKillWaiter := newProcessKillWaiter
	oldCmd := newCommander

	newProcessKillWaiter = func(
		logger process.Logger,
		gracefulKillTimeout time.Duration,
		forceKillTimeout time.Duration,
	) process.KillWaiter {
		return mProcessKillWaiter
	}

	newCommander = func(executable string, args []string, options process.CommandOptions) process.Commander {
		return mCmd
	}

	return mProcessKillWaiter, mCmd, func() {
		newProcessKillWaiter = oldNewProcessKillWaiter
		newCommander = oldCmd
	}
}

func TestExecutor_Prepare_MakesPathsAbsolute(t *testing.T) {
	tests := map[string]struct {
		defaultBuildsDir string
		defaultCacheDir  string
	}{
		"relative paths": {
			defaultBuildsDir: "builds",
			defaultCacheDir:  "cache",
		},
		"paths with $PWD": {
			defaultBuildsDir: "$PWD/builds",
			defaultCacheDir:  "$PWD/cache",
		},
		"already absolute paths": {
			defaultBuildsDir: "/tmp/builds",
			defaultCacheDir:  "/tmp/cache",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			wd, err := os.Getwd()
			require.NoError(t, err)

			shellstest.OnEachShell(t, func(t *testing.T, shell string) {
				e := &executor{
					AbstractExecutor: executors.AbstractExecutor{
						ExecutorOptions: executors.ExecutorOptions{
							DefaultBuildsDir: tt.defaultBuildsDir,
							DefaultCacheDir:  tt.defaultCacheDir,
							Shell: common.ShellScriptInfo{
								Shell: shell,
							},
						},
					},
				}

				// Create a minimal build for Prepare to work
				build := &common.Build{
					Job: spec.Job{
						Variables: spec.Variables{},
					},
					Runner: &common.RunnerConfig{},
				}

				// Call Prepare which should make paths absolute
				err = e.Prepare(common.ExecutorPrepareOptions{
					Config:  &common.RunnerConfig{},
					Build:   build,
					Context: t.Context(),
				})
				require.NoError(t, err)

				// Verify that both paths are now absolute
				assert.True(t, filepath.IsAbs(e.DefaultBuildsDir), "DefaultBuildsDir should be absolute, got: %s", e.DefaultBuildsDir)
				assert.True(t, filepath.IsAbs(e.DefaultCacheDir), "DefaultCacheDir should be absolute, got: %s", e.DefaultCacheDir)

				// Verify that relative paths are resolved relative to current working directory
				if tt.defaultBuildsDir == "builds" {
					assert.Equal(t, filepath.Join(wd, "builds"), e.DefaultBuildsDir)
				}
				if tt.defaultCacheDir == "cache" {
					assert.Equal(t, filepath.Join(wd, "cache"), e.DefaultCacheDir)
				}
			})
		})
	}
}

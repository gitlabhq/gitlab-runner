//go:build !integration

package command

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

func newCommand(
	ctx context.Context,
	t *testing.T,
	executable string,
	cmdOpts process.CommandOptions,
	options Options,
) (*process.MockCommander, *process.MockKillWaiter, Command, func()) {
	commanderMock := new(process.MockCommander)
	processKillWaiterMock := new(process.MockKillWaiter)

	oldNewCmd := newCommander
	oldNewProcessKillWaiter := newProcessKillWaiter

	cleanup := func() {
		newCommander = oldNewCmd
		newProcessKillWaiter = oldNewProcessKillWaiter

		commanderMock.AssertExpectations(t)
		processKillWaiterMock.AssertExpectations(t)
	}

	newCommander = func(string, []string, process.CommandOptions) process.Commander {
		return commanderMock
	}

	newProcessKillWaiter = func(process.Logger, time.Duration, time.Duration) process.KillWaiter {
		return processKillWaiterMock
	}

	c := New(ctx, executable, []string{}, cmdOpts, options)

	return commanderMock, processKillWaiterMock, c, cleanup
}

func TestCommand_Run(t *testing.T) {
	testErr := errors.New("test error")

	tests := map[string]struct {
		cmdStartErr       error
		cmdWaitErr        error
		getExitCode       func(err *exec.ExitError) int
		contextClosed     bool
		process           *os.Process
		expectedError     string
		expectedErrorType interface{}
		expectedExitCode  int
		options           Options
	}{
		"error on cmd start()": {
			cmdStartErr:   errors.New("test-error"),
			expectedError: "failed to start command: test-error",
		},
		"command ends with a build failure": {
			cmdWaitErr:        &exec.ExitError{ProcessState: &os.ProcessState{}},
			getExitCode:       func(err *exec.ExitError) int { return BuildFailureExitCode },
			expectedError:     "exit status 0",
			expectedErrorType: &common.BuildError{},
			expectedExitCode:  BuildFailureExitCode,
		},
		"command ends with a system failure": {
			cmdWaitErr:        &exec.ExitError{ProcessState: &os.ProcessState{}},
			getExitCode:       func(err *exec.ExitError) int { return SystemFailureExitCode },
			expectedError:     "exit status 0",
			expectedErrorType: &exec.ExitError{},
		},
		"command ends with a unknown failure": {
			cmdWaitErr:  &exec.ExitError{ProcessState: &os.ProcessState{}},
			getExitCode: func(err *exec.ExitError) int { return 255 },
			expectedError: "unknown Custom executor executable exit code 255; " +
				"executable execution terminated with: exit status 0",
			expectedErrorType: &ErrUnknownFailure{},
		},
		"command times out": {
			contextClosed: true,
			process:       &os.Process{Pid: 1234},
			expectedError: testErr.Error(),
		},
		"command ends with invalid build failure file": {
			cmdWaitErr:  &exec.ExitError{ProcessState: &os.ProcessState{}},
			getExitCode: func(err *exec.ExitError) int { return BuildFailureExitCode },
			expectedError: "unknown Custom executor executable exit code 2; " +
				"executable execution terminated with: exit status 0",
			expectedErrorType: &ErrUnknownFailure{},
			options: func() Options {
				filename := t.TempDir() + "/invalid"
				err := os.WriteFile(filename, []byte("invalid"), 0o600)
				require.NoError(t, err)
				return Options{BuildExitCodeFile: filename}
			}(),
		},
		"command ends with build failure file": {
			cmdWaitErr:        &exec.ExitError{ProcessState: &os.ProcessState{}},
			getExitCode:       func(err *exec.ExitError) int { return BuildFailureExitCode },
			expectedError:     "exit status 42",
			expectedErrorType: &common.BuildError{},
			expectedExitCode:  42,
			options: func() Options {
				filename := t.TempDir() + "/valid"
				err := os.WriteFile(filename, []byte("42"), 0o600)
				require.NoError(t, err)
				return Options{BuildExitCodeFile: filename}
			}(),
		},
		"additional information ignored": {
			cmdWaitErr:        &exec.ExitError{ProcessState: &os.ProcessState{}},
			getExitCode:       func(err *exec.ExitError) int { return BuildFailureExitCode },
			expectedError:     "exit status 42",
			expectedErrorType: &common.BuildError{},
			expectedExitCode:  42,
			options: func() Options {
				filename := t.TempDir() + "/valid"
				err := os.WriteFile(filename, []byte("42\n\nTesting..."), 0o600)
				require.NoError(t, err)
				return Options{BuildExitCodeFile: filename}
			}(),
		},
	}

	for testName, tt := range tests {
		tt := tt
		t.Run(testName, func(t *testing.T) {
			ctx, ctxCancel := context.WithCancel(context.Background())
			defer ctxCancel()

			cmdOpts := process.CommandOptions{
				Logger:              new(process.MockLogger),
				GracefulKillTimeout: 100 * time.Millisecond,
				ForceKillTimeout:    100 * time.Millisecond,
			}

			commanderMock, processKillWaiterMock, c, cleanup := newCommand(ctx, t, "exec", cmdOpts, tt.options)
			defer cleanup()

			commanderMock.On("Start").
				Return(tt.cmdStartErr)
			commanderMock.On("Wait").
				Return(func() error {
					<-time.After(500 * time.Millisecond)
					return tt.cmdWaitErr
				}).
				Maybe()

			if tt.getExitCode != nil {
				oldGetExitCode := getExitCode
				defer func() {
					getExitCode = oldGetExitCode
				}()
				getExitCode = tt.getExitCode
			}

			if tt.contextClosed {
				ctxCancel()
				processKillWaiterMock.
					On("KillAndWait", commanderMock, mock.Anything).
					Return(testErr).
					Once()
			}

			err := c.Run()

			if tt.expectedError == "" {
				assert.NoError(t, err)

				return
			}

			assert.EqualError(t, err, tt.expectedError)
			if tt.expectedErrorType != nil {
				assert.IsType(t, tt.expectedErrorType, err)
			}

			if tt.expectedExitCode != 0 {
				var buildError *common.BuildError
				if errors.As(err, &buildError) {
					assert.Equal(t, tt.expectedExitCode, buildError.ExitCode)
				}
			}
		})
	}
}

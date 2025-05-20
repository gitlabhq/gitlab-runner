//go:build !integration

package process

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func mockKillerFactory(t *testing.T) *mockKiller {
	t.Helper()

	killerMock := newMockKiller(t)

	oldNewProcessKiller := newProcessKiller

	t.Cleanup(func() {
		newProcessKiller = oldNewProcessKiller
	})

	newProcessKiller = func(logger Logger, cmd Commander) killer {
		return killerMock
	}

	return killerMock
}

func TestOSKillWait_KillAndWait(t *testing.T) {
	testProcess := &os.Process{Pid: 1234}
	processStoppedErr := errors.New("process stopped properly")
	killProcessErr := KillProcessError{testProcess.Pid}

	tests := map[string]struct {
		process          *os.Process
		terminateProcess bool
		forceKillProcess bool
		expectedError    error
	}{
		"process is nil": {
			process:       nil,
			expectedError: ErrProcessNotStarted,
		},
		"process terminated": {
			process:          testProcess,
			terminateProcess: true,
			expectedError:    processStoppedErr,
		},
		"process force-killed": {
			process:          testProcess,
			forceKillProcess: true,
			expectedError:    processStoppedErr,
		},
		"process killing failed": {
			process:       testProcess,
			expectedError: &killProcessErr,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			waitCh := make(chan error, 1)

			killerMock := mockKillerFactory(t)
			loggerMock := NewMockLogger(t)
			commanderMock := NewMockCommander(t)

			commanderMock.On("Process").Return(testCase.process)

			if testCase.process != nil {
				loggerMock.
					On("WithFields", mock.Anything).
					Return(loggerMock)

				terminateCall := killerMock.On("Terminate")
				forceKillCall := killerMock.On("ForceKill").Maybe()

				if testCase.terminateProcess {
					terminateCall.Run(func(_ mock.Arguments) {
						waitCh <- processStoppedErr
					})
				}

				if testCase.forceKillProcess {
					forceKillCall.Run(func(_ mock.Arguments) {
						waitCh <- processStoppedErr
					})
				}
			}

			kw := NewOSKillWait(loggerMock, 100*time.Millisecond, 100*time.Millisecond)
			err := kw.KillAndWait(commanderMock, waitCh)

			if testCase.expectedError == nil {
				assert.NoError(t, err)
				return
			}

			assert.ErrorIs(t, testCase.expectedError, err)
		})
	}
}

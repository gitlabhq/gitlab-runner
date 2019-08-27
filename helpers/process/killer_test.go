package process

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func mockKillerFactory(t *testing.T) (*mockKiller, func()) {
	t.Helper()

	killerMock := new(mockKiller)

	oldNewProcessKiller := newProcessKiller
	cleanup := func() {
		newProcessKiller = oldNewProcessKiller
		killerMock.AssertExpectations(t)
	}

	newProcessKiller = func(logger common.BuildLogger, process *os.Process) killer {
		return killerMock
	}

	return killerMock, cleanup
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
			killerMock, cleanup := mockKillerFactory(t)
			defer cleanup()

			logger := common.NewBuildLogger(nil, logrus.NewEntry(logrus.New()))
			kw := NewOSKillWait(logger, 10*time.Millisecond, 10*time.Millisecond)

			waitCh := make(chan error, 1)

			if testCase.process != nil {
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

			err := kw.KillAndWait(testCase.process, waitCh)

			if testCase.expectedError == nil {
				assert.NoError(t, err)
				return
			}

			assert.True(t, errors.Is(testCase.expectedError, err))
		})
	}
}

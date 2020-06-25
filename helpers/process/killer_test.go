package process

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func mockKillerFactory(t *testing.T) (*mockKiller, func()) {
	t.Helper()

	killerMock := new(mockKiller)

	oldNewProcessKiller := newProcessKiller
	cleanup := func() {
		newProcessKiller = oldNewProcessKiller
		killerMock.AssertExpectations(t)
	}

	newProcessKiller = func(logger Logger, cmd Commander) killer {
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
			waitCh := make(chan error, 1)

			killerMock, cleanup := mockKillerFactory(t)
			defer cleanup()

			loggerMock := new(MockLogger)
			defer loggerMock.AssertExpectations(t)

			commanderMock := new(MockCommander)
			defer commanderMock.AssertExpectations(t)

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

			assert.True(t, errors.Is(testCase.expectedError, err))
		})
	}
}

func newKillerWithLoggerAndCommand(
	t *testing.T,
	duration string,
	skipTerminate bool,
) (killer, *MockLogger, Commander, func()) {
	t.Helper()

	loggerMock := new(MockLogger)
	sleepBinary := prepareTestBinary(t)

	args := []string{duration}
	if skipTerminate {
		args = append(args, "skip-terminate-signals")
	}

	command := NewOSCmd(sleepBinary, args, CommandOptions{})
	err := command.Start()
	require.NoError(t, err)

	k := newKiller(loggerMock, command)

	cleanup := func() {
		loggerMock.AssertExpectations(t)
		err = os.RemoveAll(filepath.Dir(sleepBinary))
		if err != nil {
			t.Logf("Failed to cleanup files %q: %v", filepath.Dir(sleepBinary), err)
		}
	}

	return k, loggerMock, command, cleanup
}

func prepareTestBinary(t *testing.T) string {
	t.Helper()

	dir, err := ioutil.TempDir("", strings.ReplaceAll(t.Name(), "/", ""))
	require.NoError(t, err)
	binaryPath := filepath.Join(dir, strconv.FormatInt(time.Now().UnixNano(), 10))

	// Windows can only have executables ending with `.exe`
	if runtime.GOOS == "windows" {
		binaryPath = fmt.Sprintf("%s.exe", binaryPath)
	}

	_, currentTestFile, _, _ := runtime.Caller(0) // nolint:dogsled
	sleepCommandSource := filepath.Clean(filepath.Join(filepath.Dir(currentTestFile), "testdata", "sleep", "main.go"))

	command := exec.Command("go", "build", "-o", binaryPath, sleepCommandSource)
	err = command.Run()
	require.NoError(t, err)

	return binaryPath
}

// Unix and Windows have different test cases expecting different data, check
// killer_unix_test.go and killer_windows_test.go for each system test case.
type testKillerTestCase struct {
	alreadyStopped bool
	skipTerminate  bool
	expectedError  string
}

func TestKiller(t *testing.T) {
	sleepDuration := "3s"

	for testName, testCase := range testKillerTestCases() {
		t.Run(testName, func(t *testing.T) {
			k, loggerMock, cmd, cleanup := newKillerWithLoggerAndCommand(t, sleepDuration, testCase.skipTerminate)
			defer cleanup()

			waitCh := make(chan error)

			if testCase.alreadyStopped {
				_ = cmd.Process().Kill()

				loggerMock.On(
					"Warn",
					"Failed to terminate process:",
					mock.Anything,
				)
				loggerMock.On(
					"Warn",
					"Failed to force-kill:",
					mock.Anything,
				)
			}

			go func() {
				waitCh <- cmd.Wait()
			}()

			time.Sleep(1 * time.Second)
			k.Terminate()

			err := <-waitCh
			if testCase.expectedError == "" {
				assert.NoError(t, err)
				return
			}

			assert.EqualError(t, err, testCase.expectedError)
		})
	}
}

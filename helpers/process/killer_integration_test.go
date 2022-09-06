//go:build integration

package process_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

func newKillerWithLoggerAndCommand(
	t *testing.T,
	duration string,
	skipTerminate bool,
) (process.Killer, *process.MockLogger, process.Commander, func()) {
	t.Helper()

	loggerMock := new(process.MockLogger)
	sleepBinary := prepareTestBinary(t)

	args := []string{duration}
	if skipTerminate {
		args = append(args, "skip-terminate-signals")
	}

	command := process.NewOSCmd(sleepBinary, args, process.CommandOptions{})
	err := command.Start()
	require.NoError(t, err)

	k := process.NewKillerForTest(loggerMock, command)

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

	dir := t.TempDir()
	binaryPath := filepath.Join(dir, strconv.FormatInt(time.Now().UnixNano(), 10))

	// Windows can only have executables ending with `.exe`
	if runtime.GOOS == "windows" {
		binaryPath = fmt.Sprintf("%s.exe", binaryPath)
	}

	_, currentTestFile, _, _ := runtime.Caller(0) // nolint:dogsled
	sleepCommandSource := filepath.Clean(filepath.Join(filepath.Dir(currentTestFile), "testdata", "sleep", "main.go"))

	command := exec.Command("go", "build", "-o", binaryPath, sleepCommandSource)
	err := command.Run()
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

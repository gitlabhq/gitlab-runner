//go:build integration

package process_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

func newKillerWithLoggerAndCommand(
	t *testing.T,
	duration string,
	skipTerminate bool,
	useWindowsLegacyProcessStrategy bool,
	useWindowsJobObject bool,
) (process.Killer, *process.MockLogger, process.Commander, func(), *dumbTestLogger) {
	t.Helper()

	loggerMock := new(process.MockLogger)
	sleepBinary := prepareTestBinary(t)

	args := []string{duration}
	if skipTerminate {
		args = append(args, "skip-terminate-signals")
	}

	logger := dumbTestLogger{}

	command := process.NewOSCmd(sleepBinary, args,
		process.CommandOptions{
			UseWindowsLegacyProcessStrategy: useWindowsLegacyProcessStrategy,
			UseWindowsJobObject:             useWindowsJobObject,
			Logger:                          &logger,
		})
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

	return k, loggerMock, command, cleanup, &logger
}

var _ process.Logger = (*dumbTestLogger)(nil)

type dumbTestLogger struct {
	buf    bytes.Buffer
	fields []logrus.Fields
}

func (d *dumbTestLogger) WithFields(fields logrus.Fields) process.Logger {
	return &dumbTestLogger{
		fields: append(d.fields, fields),
	}
}

func (d *dumbTestLogger) Warn(args ...any) {
	allArgs := []any{}
	for _, f := range d.fields {
		allArgs = append(allArgs, f)
	}
	allArgs = append(allArgs, args...)

	d.buf.WriteString(fmt.Sprintln(allArgs))
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
	alreadyStopped                  bool
	skipTerminate                   bool
	expectedError                   string
	useWindowsLegacyProcessStrategy bool
	useWindowsJobObject             bool
}

func TestKiller(t *testing.T) {
	sleepDuration := "3s"

	for testName, testCase := range testKillerTestCases() {
		t.Run(testName, func(t *testing.T) {
			k, loggerMock, cmd, cleanup, logs := newKillerWithLoggerAndCommand(t, sleepDuration, testCase.skipTerminate, testCase.useWindowsLegacyProcessStrategy, testCase.useWindowsJobObject)
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

			if testCase.useWindowsJobObject {
				k.ForceKill()
			} else {
				k.Terminate()
			}

			err := <-waitCh
			if testCase.expectedError == "" {
				assert.NoError(t, err)
				return
			}

			assert.Empty(t, logs.buf.String())
			assert.EqualError(t, err, testCase.expectedError)
		})
	}
}

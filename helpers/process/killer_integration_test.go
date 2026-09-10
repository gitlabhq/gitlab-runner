//go:build integration

package process_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	loggerMock := process.NewMockLogger(t)
	sleepBinary := buildTestBinary(t, "sleep")

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

// buildTestBinary compiles testdata/<name>/main.go and returns the path to the
// binary.
func buildTestBinary(t *testing.T, name string) string {
	t.Helper()

	binaryPath := filepath.Join(t.TempDir(), name)

	// Windows can only have executables ending with `.exe`
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	_, currentTestFile, _, _ := runtime.Caller(0) // nolint:dogsled
	source := filepath.Clean(filepath.Join(filepath.Dir(currentTestFile), "testdata", name, "main.go"))

	output, err := exec.Command("go", "build", "-o", binaryPath, source).CombinedOutput()
	require.NoError(t, err, "building %s: %s", name, output)

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

			if testCase.useWindowsJobObject {
				// ForceKill may call Warn if taskKill fails
				loggerMock.On(
					"Warn",
					"Failed to force-kill:",
					mock.Anything,
				).Maybe()
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

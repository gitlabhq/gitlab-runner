//go:build integration && windows

package process_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
	"golang.org/x/sys/windows"
)

type testProcessOutput struct {
	childPID int
	err      error
}

func TestEnsureSubprocessTerminationOnExit(t *testing.T) {
	testBinary := prepareEnsureSubprocessTerminationBinary(t)

	testProcess := exec.Command(testBinary)
	stdout, err := testProcess.StdoutPipe()
	require.NoError(t, err)

	var stderr bytes.Buffer
	testProcess.Stderr = &stderr

	require.NoError(t, testProcess.Start())

	testProcessKilled := false
	t.Cleanup(func() {
		if testProcessKilled {
			return
		}
		_ = testProcess.Process.Kill()
		_ = testProcess.Wait()
	})

	resultCh := make(chan testProcessOutput, 1)
	go readTestBinaryOutput(stdout, resultCh)

	var result testProcessOutput
	select {
	case result = <-resultCh:
	case <-time.After(15 * time.Second):
		t.Fatalf("timed out waiting for test process readiness, stderr: %s", stderr.String())
	}

	require.NoError(t, result.err, "stderr: %s", stderr.String())
	require.NotZero(t, result.childPID)

	childHandle, err := process.FindProcessHandleFromPID(result.childPID)
	require.NoError(t, err)
	defer windows.CloseHandle(childHandle)

	require.NoError(t, testProcess.Process.Kill())
	_ = testProcess.Wait()
	testProcessKilled = true

	err = waitForProcess(childHandle, 1*time.Second)
	if err != nil {
		_ = windows.TerminateProcess(childHandle, 1)
	}
	require.NoErrorf(t, err, "child subprocess didn't exit, PID = %d", result.childPID)
}

func readTestBinaryOutput(r io.Reader, resultCh chan<- testProcessOutput) {
	var childPID int

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "Child PID:") {
			pidText := strings.TrimSpace(strings.TrimPrefix(line, "Child PID:"))
			pid, err := strconv.Atoi(pidText)
			if err != nil {
				resultCh <- testProcessOutput{err: fmt.Errorf("parsing child pid from %q: %w", line, err)}
				return
			}
			childPID = pid
			continue
		}

		if line == "READY" {
			if childPID == 0 {
				resultCh <- testProcessOutput{err: errors.New("received READY without child pid")}
				return
			}

			resultCh <- testProcessOutput{childPID: childPID}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		resultCh <- testProcessOutput{err: fmt.Errorf("reading test process output: %w", err)}
		return
	}

	resultCh <- testProcessOutput{err: errors.New("test process exited before signaling readiness")}
}

func waitForProcess(handle windows.Handle, timeout time.Duration) error {
	status, err := windows.WaitForSingleObject(handle, uint32(timeout/time.Millisecond))
	if err != nil {
		return err
	}

	switch status {
	case windows.WAIT_OBJECT_0:
		return nil
	case uint32(windows.WAIT_TIMEOUT):
		return fmt.Errorf("timed out waiting for process after %s", timeout)
	default:
		return fmt.Errorf("unexpected wait status: %d", status)
	}
}

func prepareEnsureSubprocessTerminationBinary(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	binaryPath := filepath.Join(dir, strconv.FormatInt(time.Now().UnixNano(), 10)+".exe")

	_, currentTestFile, _, _ := runtime.Caller(0) //nolint:dogsled
	source := filepath.Clean(filepath.Join(
		filepath.Dir(currentTestFile),
		"testdata",
		"ensure_subprocess_termination",
		"main.go",
	))

	buildCmd := exec.Command("go", "build", "-o", binaryPath, source)
	output, err := buildCmd.CombinedOutput()
	require.NoErrorf(t, err, "building test binary failed: %s", output)

	return binaryPath
}

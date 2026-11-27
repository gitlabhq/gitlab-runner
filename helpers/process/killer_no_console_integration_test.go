//go:build integration && windows

package process_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const detachedProcess = 0x00000008

type terminationResult struct {
	WaitError  string   `json:"wait_error"`
	Warnings   []string `json:"warnings"`
	HadConsole bool     `json:"had_console"`
}

// TestKillerWithoutConsole covers a runner that holds no console, which is the
// state the Service Control Manager starts it in. GenerateConsoleCtrlEvent
// needs a console, and AttachConsole cannot reach a parent that has none, so
// in this state the Ctrl-Break event must go through the console of the job
// process.
//
// The test cannot install a service, so it reaches the same console state with
// the DETACHED_PROCESS creation flag.
func TestKillerWithoutConsole(t *testing.T) {
	dir := t.TempDir()
	resultPath := filepath.Join(dir, "result.json")

	terminator := buildTestBinary(t, "detached_terminator")

	cmd := exec.Command(
		terminator,
		"-stage", "outer",
		"-sleep-binary", buildTestBinary(t, "sleep"),
		"-result", resultPath,
	)
	// The test process holds a console, so detach the outer stage to keep it
	// from being inherited.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess}

	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	result := readResult(t, resultPath)

	// Without this the test proves nothing: a process that kept a console
	// takes the path that already worked.
	require.False(t, result.HadConsole,
		"the detached process holds a console, so it cannot cover this case")

	// The sleep binary exits with 3 when it is asked to stop. Any other outcome
	// means it was killed instead of asked.
	assert.Equal(t, "exit status 3", result.WaitError,
		"process was killed rather than terminated, warnings: %v", result.Warnings)

	// The process is reached through its own console only after the direct
	// call fails, and that is logged. Requiring the warning proves the test met
	// the state it is here to cover.
	assert.Contains(t, strings.Join(result.Warnings, "\n"),
		"Failed to send Ctrl-Break event, retrying through the console of the process:")
}

func readResult(t *testing.T, path string) terminationResult {
	t.Helper()

	if !assert.Eventually(t, func() bool {
		raw, err := os.ReadFile(path)
		return err == nil && len(raw) > 0
	}, time.Minute, 200*time.Millisecond) {
		output, _ := os.ReadFile(path + ".log")
		require.FailNow(t, "the detached process wrote no outcome", "output: %s", output)
	}

	// The terminator writes the outcome to a temporary file and renames it into
	// place, so this read sees the whole file.
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var result terminationResult
	require.NoError(t, json.Unmarshal(raw, &result))

	return result
}

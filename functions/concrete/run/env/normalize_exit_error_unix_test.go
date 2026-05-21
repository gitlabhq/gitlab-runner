//go:build !windows && !integration

package env

import (
	"errors"
	"io"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeExitError_NilStaysNil(t *testing.T) {
	assert.NoError(t, normalizeExitError(nil, nil))
}

func TestNormalizeExitError_PassesThroughWhenProcessStateMissing(t *testing.T) {
	sentinel := errors.New("boom")
	assert.Same(t, sentinel, normalizeExitError(sentinel, nil))
}

func TestNormalizeExitError_PassesThroughForOtherErrors(t *testing.T) {
	// `exit 1` is the bog-standard script failure path: ExitCode==1,
	// no signal involvement, no WaitDelay. normalizeExitError must not
	// swallow it.
	cmd := exec.Command("/bin/sh", "-c", "exit 1")
	err := cmd.Run()
	require.Error(t, err)
	got := normalizeExitError(err, cmd.ProcessState)
	assert.Equal(t, err, got, "exit 1 must surface as a real error")
}

func TestNormalizeExitError_UnwrapsWaitDelayWhenExitCodeIsZero(t *testing.T) {
	// Launch a script that exits 0 but leaves a backgrounded sleep
	// inheriting stdout. The std-lib exec.Cmd.WaitDelay timer fires,
	// killing the bg child and returning exec.ErrWaitDelay even
	// though the foreground script exited cleanly.
	cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", "(sleep 30) & exit 0")
	cmd.WaitDelay = 200 * time.Millisecond
	// Setting Stdout to a non-pipe writer makes Go spin up a copy
	// goroutine that holds the child's write-end open via the bg
	// sleep, which is what trips WaitDelay.
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	runErr := cmd.Run()
	require.Error(t, runErr, "expected WaitDelay error from bg-child pipe inheritance")
	require.True(t, errors.Is(runErr, exec.ErrWaitDelay),
		"sanity: error must be ErrWaitDelay, got %v", runErr)
	require.NotNil(t, cmd.ProcessState)
	require.Equal(t, 0, cmd.ProcessState.ExitCode(),
		"sanity: foreground script must have exited 0")

	assert.NoError(t, normalizeExitError(runErr, cmd.ProcessState),
		"WaitDelay + clean exit must reclassify as success")
}

func TestNormalizeExitError_UnwrapsNonFatalUserSignals(t *testing.T) {
	for _, sig := range []syscall.Signal{
		syscall.SIGUSR1,
		syscall.SIGUSR2,
		syscall.SIGHUP,
		syscall.SIGPIPE,
	} {
		t.Run(sig.String(), func(t *testing.T) {
			// Spawn /bin/sh and kill it with the signal under test.
			cmd := exec.Command("/bin/sh", "-c", "sleep 30")
			require.NoError(t, cmd.Start())

			// Some shells install a default handler for SIGHUP that
			// re-execs or ignores; -KILL into the syscall.Kill directly.
			require.NoError(t, syscall.Kill(cmd.Process.Pid, sig))

			runErr := cmd.Wait()
			require.Error(t, runErr)
			require.NotNil(t, cmd.ProcessState)

			ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
			require.True(t, ok, "expected unix WaitStatus")
			require.True(t, ws.Signaled(), "process must report signal exit")
			require.Equal(t, sig, ws.Signal(), "process must report the signal we sent")

			assert.NoError(t, normalizeExitError(runErr, cmd.ProcessState),
				"%s must be reclassified as success", sig)
		})
	}
}

func TestNormalizeExitError_PassesThroughFatalSignal(t *testing.T) {
	// SIGKILL is the canonical "this script was killed for cause".
	// normalizeExitError must NOT swallow it — the runner relies on
	// it to surface things like Docker OOM-kill (alpine-id-overflow).
	cmd := exec.Command("/bin/sh", "-c", "sleep 30")
	require.NoError(t, cmd.Start())
	require.NoError(t, syscall.Kill(cmd.Process.Pid, syscall.SIGKILL))

	runErr := cmd.Wait()
	require.Error(t, runErr)
	got := normalizeExitError(runErr, cmd.ProcessState)
	assert.Equal(t, runErr, got, "SIGKILL must remain a job failure")
}

func TestNormalizeExitError_PassesThroughSIGTERM(t *testing.T) {
	// SIGTERM is what gracefulexitcmd.Cmd.Cancel raises on context
	// cancellation. The runner needs this to surface so the build is
	// reported as canceled rather than silently passing.
	cmd := exec.Command("/bin/sh", "-c", "sleep 30")
	require.NoError(t, cmd.Start())
	require.NoError(t, syscall.Kill(cmd.Process.Pid, syscall.SIGTERM))

	runErr := cmd.Wait()
	require.Error(t, runErr)
	got := normalizeExitError(runErr, cmd.ProcessState)
	assert.Equal(t, runErr, got, "SIGTERM must remain a job failure")
}

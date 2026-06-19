package internal

import (
	"errors"
	"os"
	"os/exec"
)

// normalizeExitError reclassifies two exec outcomes that gracefulexitcmd
// surfaces as errors but which the runner's legacy bash-pipe execution
// (shells/bash.go) effectively treats as success:
//
//  1. The script exited 0, but a backgrounded child outlived
//     gracefulexitcmd's WaitDelay holding the parent's stdio pipes
//     open. WaitDelay's job is to bound that drain, not to fail the
//     job; the exit code already says the user script was fine.
//
//  2. The script's outer shell was terminated by a non-fatal
//     user-defined signal (SIGUSR1, SIGUSR2, SIGHUP, SIGPIPE). These
//     are routinely raised by user scripts that signal themselves
//     (e.g. `kill -USR1 $$`), so what looks like a "script failure"
//     here is actually expected behaviour. Surfacing these as failures
//     diverges from the legacy executor without offering a recovery
//     path inside the user's script.
//
// This mirrors the equivalent reclassification on the concrete run path
// (functions/concrete/run/env). The two are deliberately kept in sync so
// FF_SCRIPT_TO_STEP_MIGRATION and FF_USE_CONCRETE behave identically on
// these benign teardown outcomes.
//
// Cancellation-driven SIGTERM (from gracefulexitcmd.Cmd.Cancel) is
// deliberately NOT included: the runner needs that to propagate so the
// build is reported as canceled rather than passed silently.
func normalizeExitError(err error, ps *os.ProcessState) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, exec.ErrWaitDelay) && ps != nil && ps.ExitCode() == 0 {
		return nil
	}

	if ps != nil && isNonFatalUserSignal(ps) {
		return nil
	}

	return err
}

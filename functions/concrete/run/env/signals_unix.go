//go:build !windows

package env

import (
	"os"
	"syscall"
)

// isNonFatalUserSignal reports whether ps describes a process that
// was terminated by a non-fatal user-defined signal (USR1, USR2, HUP,
// PIPE). These are deliberately treated as recoverable in
// normalizeExitError — see the doc comment there.
func isNonFatalUserSignal(ps *os.ProcessState) bool {
	ws, ok := ps.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}
	if !ws.Signaled() {
		return false
	}
	switch ws.Signal() {
	case syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGHUP, syscall.SIGPIPE:
		return true
	}
	return false
}

//go:build windows

package env

import "os"

// isNonFatalUserSignal: Windows has no SIGUSR1/SIGUSR2/SIGHUP/SIGPIPE,
// so there's nothing here to reclassify.
func isNonFatalUserSignal(_ *os.ProcessState) bool {
	return false
}

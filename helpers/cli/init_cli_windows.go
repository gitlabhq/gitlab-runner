package cli_helpers

import (
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

// InitCli initializes the Windows console window by activating virtual terminal features.
// Calling this function enables colored terminal output.
func InitCli() {
	setConsoleMode(windows.Stdout, windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) // enable VT processing on standard output stream
	setConsoleMode(windows.Stderr, windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) // enable VT processing on standard error stream
}

// setConsoleMode sets the given flags on the given
// console standard stream.
func setConsoleMode(handle windows.Handle, flags uint32) {
	var mode uint32

	// add console mode flag
	if err := windows.GetConsoleMode(handle, &mode); err == nil {
		err := windows.SetConsoleMode(handle, mode|flags)
		if err != nil {
			logrus.WithError(err).Info("Did not set console mode for cli")
		}
	}
}

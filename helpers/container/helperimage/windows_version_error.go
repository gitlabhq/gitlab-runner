package helperimage

import "fmt"

type UnsupportedWindowsVersionError struct {
	version string
}

func NewUnsupportedWindowsVersionError(version string) *UnsupportedWindowsVersionError {
	return &UnsupportedWindowsVersionError{version: version}
}

func (d *UnsupportedWindowsVersionError) Error() string {
	return fmt.Sprintf("unsupported Windows version: %v", d.version)
}

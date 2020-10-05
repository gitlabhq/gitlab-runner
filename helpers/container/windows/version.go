package windows

import (
	"fmt"
	"strings"
)

const (
	// V1809 is the Windows version that is 1809 and also known as Windows 2019
	// ltsc.
	V1809 = "1809"
	// V1903 is the Windows version that is 1903 sac.
	V1903 = "1903"
	// V1909 is the Windows version that is 1903 sac.
	V1909 = "1909"
	// V2004 is the Windows version that is 2004 sac.
	V2004 = "2004"
)

// UnsupportedWindowsVersionError represents that the version specified is not
// supported.
type UnsupportedWindowsVersionError struct {
	Version string
}

func NewUnsupportedWindowsVersionError(version string) *UnsupportedWindowsVersionError {
	return &UnsupportedWindowsVersionError{Version: version}
}

func (e *UnsupportedWindowsVersionError) Error() string {
	return fmt.Sprintf("unsupported Windows Version: %s", e.Version)
}

func (e *UnsupportedWindowsVersionError) Is(err error) bool {
	_, ok := err.(*UnsupportedWindowsVersionError)

	return ok
}

var supportedWindowsVersions = []string{
	V1809,
	V1903,
	V1909,
	V2004,
}

// Version checks the specified operatingSystem to see if it's one of the
// supported Windows version. If true, it returns the os version.
// UnsupportedWindowsVersionError is returned when no supported Windows version
// is found in the string.
func Version(operatingSystem string) (string, error) {
	for _, windowsVersion := range supportedWindowsVersions {
		if strings.Contains(operatingSystem, fmt.Sprintf(" %s ", windowsVersion)) {
			return windowsVersion, nil
		}
	}

	return "", NewUnsupportedWindowsVersionError(operatingSystem)
}

package windows

import (
	"fmt"
	"strings"
)

const (
	// V1809 is the Windows version that is 1809 and also known as Windows 2019
	// ltsc.
	V1809 = "1809"
	// V2004 is the Windows version that is 2004 sac.
	V2004 = "2004"
	// V20H2 is the Windows version that is 2009 sac.
	V20H2 = "2009"
	// V21H1 is the Windows version that is 21H1 sac.
	V21H1 = "21H1"
	// V21H2 is the Windows version that is 21H2 also known as Windows 2022 LTSC.
	V21H2 = "21H2"
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
	V21H2,
	V21H1,
	V20H2,
	V2004,
	V1809,
}

var supportedWindowsBuilds = map[string]string{
	"10.0.17763": V1809,
	"10.0.19041": V2004,
	"10.0.19042": V20H2,
}

// Version checks the specified operatingSystem to see if it's one of the
// supported Windows version. If true, it returns the os version.
// UnsupportedWindowsVersionError is returned when no supported Windows version
// is found in the string.
func Version(operatingSystem string) (string, error) {
	for _, windowsVersion := range supportedWindowsVersions {
		if strings.Contains(operatingSystem, fmt.Sprintf(" %s ", windowsVersion)) {
			if !strings.Contains(operatingSystem, fmt.Sprintf(" %s ", windowsVersion)) {
				continue
			}

			// Both V20H2 and LTSC2022 have the Version 2009
			// However, the year 2022 is found also in the name
			// of the 21H1/LTSC2022 operating system name (Windows Server 2022 Datacenter Version 2009).
			// When the version is 2009, we also check if "Windows Server 2022" can be found in the name
			// to better detect the windows version
			if windowsVersion == V20H2 && isLTSC2022(operatingSystem) {
				return V21H1, nil
			}

			return windowsVersion, nil
		}
	}

	windowsVersion, ok := supportedWindowsBuilds[operatingSystem]
	if ok {
		return windowsVersion, nil
	}

	return "", NewUnsupportedWindowsVersionError(operatingSystem)
}

func isLTSC2022(operatingSystem string) bool {
	return strings.Contains(operatingSystem, "Windows Server 2022")
}

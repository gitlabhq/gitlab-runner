package windows

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	// V1809 is the Windows version that is 1809 and also known as Windows 2019
	// ltsc.
	V1809 = "1809"
	// V21H2 is the Windows version that is 21H2 also known as Windows 2022 LTSC.
	V21H2 = "21H2"
	// V24H2 is the Windows version that is 24H2 also known as Windows 2025 LTSC.
	V24H2 = "24H2"
)

var ErrUnsupportedWindowsVersion = errors.New("unsupported Windows version")

var supportedWindowsBuilds = map[string]string{
	// Windows server versions: https://en.wikipedia.org/wiki/List_of_Microsoft_Windows_versions#Server_versions
	// Compatibility: https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/version-compatibility#windows-server-host-os-compatibility
	"10.0.17763": V1809,
	"10.0.20348": V21H2,
	"10.0.26100": V24H2,

	// Windows client versions: https://en.wikipedia.org/wiki/List_of_Microsoft_Windows_versions#Personal_computer_versions
	// Compatibility: https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/version-compatibility#windows-client-host-os-compatibility
	"10.0.19043": V1809,
	"10.0.19044": V1809,
	"10.0.19045": V1809,
	"10.0.22000": V21H2,
	"10.0.22621": V21H2,
	"10.0.22631": V21H2,
	"10.0.26200": V24H2,
	"10.0.26300": V24H2,
}

// Resolution is the result of mapping a host kernel version to a servercore version.
type Resolution struct {
	Version      string // servercore version, e.g. V24H2
	HostBuild    string // "10.0.26400"
	MatchedBuild string // entry used; differs from HostBuild on fallback
}

// IsFallback reports whether a nearby known build was used instead of an exact
// match. Callers must check Resolve's error first: a zero-value Resolution
// (returned on error) reports false and is not meaningful.
func (r Resolution) IsFallback() bool {
	return r.HostBuild != r.MatchedBuild
}

// errUnsupported returns a wrapped ErrUnsupportedWindowsVersion for the given
// kernel version string.
func errUnsupported(kernelVersion string) error {
	return fmt.Errorf("%w: %v", ErrUnsupportedWindowsVersion, kernelVersion)
}

// parseBuildKey parses a "major.minor.build" key into its three integer
// components. It is the canonical parser for supportedWindowsBuilds keys.
// Returns ok=false if any component is non-numeric or the key has fewer than
// three dot-separated components.
func parseBuildKey(key string) (int, int, int, bool) {
	parts := strings.Split(key, ".")
	if len(parts) < 3 {
		return 0, 0, 0, false
	}
	var n [3]int
	for i := range n {
		v, err := strconv.Atoi(parts[i])
		if err != nil {
			return 0, 0, 0, false
		}
		n[i] = v
	}
	return n[0], n[1], n[2], true
}

// Resolve maps a kernel version string to a servercore version. An exact build
// match wins; otherwise the highest known build strictly below the host build
// with the same major.minor is used. Returns ErrUnsupportedWindowsVersion when
// neither applies (host below the oldest known build, different major.minor, or
// unparseable input).
func Resolve(kernelVersion string) (Resolution, error) {
	semver := strings.FieldsFunc(kernelVersion, func(r rune) bool {
		return r == '.' || r == ' '
	})
	if len(semver) < 3 {
		return Resolution{}, errUnsupported(kernelVersion)
	}

	build := strings.Join(semver[:3], ".")

	// Exact lookup first: a strict build < host loop alone would turn every
	// exact match into a fallback (e.g. host 26300 → matched 26200).
	if v, ok := supportedWindowsBuilds[build]; ok {
		return Resolution{Version: v, HostBuild: build, MatchedBuild: build}, nil
	}

	hostMaj, hostMin, hostBld, ok := parseBuildKey(build)
	if !ok {
		return Resolution{}, errUnsupported(kernelVersion)
	}

	// No exact match: find the highest known build that is strictly below the
	// host build and shares the same major.minor.
	bestBld := -1 // -1 means no candidate yet; all real builds are non-negative.
	bestVer := ""
	bestKey := ""
	for k, v := range supportedWindowsBuilds {
		kmaj, kmin, kbld, kOK := parseBuildKey(k)
		if !kOK {
			continue
		}
		if kmaj == hostMaj && kmin == hostMin && kbld < hostBld && kbld > bestBld {
			bestBld = kbld
			bestVer = v
			bestKey = k
		}
	}

	if bestBld < 0 {
		return Resolution{}, errUnsupported(kernelVersion)
	}

	return Resolution{Version: bestVer, HostBuild: build, MatchedBuild: bestKey}, nil
}

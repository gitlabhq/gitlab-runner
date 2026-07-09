package helperimage

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/windows"
)

// ltsc maps a supported Windows version to its LTSC marketing year. This is the
// one fact that doesn't follow from the version string (e.g. 24H2 -> 2025); the
// image tag and prebuilt bundle names are then derived from it by pattern.
var ltsc = map[string]string{
	windows.V1809: "ltsc2019",
	windows.V21H2: "ltsc2022",
	windows.V24H2: "ltsc2025",
}

type windowsInfo struct{}

func (w *windowsInfo) Create(revision string, cfg Config) (Info, error) {
	version, err := windows.Version(cfg.KernelVersion)
	if err != nil {
		return Info{}, fmt.Errorf("detecting base image: %w", err)
	}

	ltscYear, ok := ltsc[version]
	if !ok {
		return Info{}, fmt.Errorf("detecting base image: %w: %v", windows.ErrUnsupportedWindowsVersion, version)
	}

	// Normalise the architecture reported by the Docker daemon (or config) to
	// the token used in helper image names. Any architecture other than arm64
	// (including an unspecified one) resolves to x86_64, preserving the
	// historical behaviour of always serving the x86_64 Windows helper image.
	arch := cfg.Architecture
	switch arch {
	case platformAarch64, archArm64:
		arch = archArm64
	default: // incl. platformAmd64, archX8664
		arch = archX8664
	}

	// Microsoft only publishes arm64 base images for Windows Server 2025 (24H2)
	// and later. On earlier versions, fall back to x86_64 to preserve the
	// previous behaviour instead of failing.
	if arch != archX8664 && !armSupportedForWindowsVersion(version) {
		arch = archX8664
	}

	cmd := getPowerShellCmd(cfg.Shell)
	if cfg.ProxyExec {
		cmd = append([]string{"gitlab-runner-helper", "proxy-exec", "--bootstrap"}, cmd...)
	}

	return Info{
		Architecture: arch,
		Name:         GitLabRegistryName,
		Tag:          fmt.Sprintf("%s-%s-servercore%s", arch, revision, version),
		Cmd:          cmd,
		Prebuilt:     fmt.Sprintf("prebuilt-windows-servercore-%s-%s", ltscYear, arch),
	}, nil
}

// armSupportedForWindowsVersion reports whether Microsoft publishes arm64 base
// images for the given Windows version. Only Windows Server 2025 (24H2) and
// later qualify; Windows Server 2019 (1809) and 2022 (21H2) are x86_64-only.
func armSupportedForWindowsVersion(version string) bool {
	switch version {
	case windows.V1809, windows.V21H2:
		return false
	default:
		return true
	}
}

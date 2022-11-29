package helperimage

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/windows"
)

const (
	baseImage1809 = "servercore1809"
	baseImage2004 = "servercore2004"
	baseImage20H2 = "servercore20H2"
	baseImage21H1 = "servercore21H1"

	windowsSupportedArchitecture = "x86_64"
)

var helperImages = map[string]string{
	windows.V1809: baseImage1809,
	windows.V2004: baseImage2004,
	windows.V20H2: baseImage20H2,
	windows.V21H1: baseImage21H1,
	//nolint:lll
	// We can use the same base image as for V21H1 because it is built on top of the ltsc2022
	// upstream image which is compatible with any Windows 11/Windows Server 2022 host.
	// See https://gitlab.com/gitlab-org/gitlab-runner/-/blob/007a6830a1da61df2c2afb8b0ad5d75abebff8e1/ci/build_release_windows_images.ps1#L135
	// for our mapping logic and https://techcommunity.microsoft.com/t5/containers/windows-server-2022-and-beyond-for-containers/ba-p/2712487
	// for the upstream compatibility announcement.
	windows.V21H2: baseImage21H1,
}

type windowsInfo struct{}

func (w *windowsInfo) Create(revision string, cfg Config) (Info, error) {
	baseImage, err := w.baseImage(cfg.OperatingSystem)
	if err != nil {
		return Info{}, fmt.Errorf("detecting base image: %w", err)
	}

	return Info{
		Architecture:            windowsSupportedArchitecture,
		Name:                    GitLabRegistryName,
		Tag:                     fmt.Sprintf("%s-%s-%s", windowsSupportedArchitecture, revision, baseImage),
		IsSupportingLocalImport: false,
		Cmd:                     getPowerShellCmd(cfg.Shell),
	}, nil
}

func (w *windowsInfo) baseImage(operatingSystem string) (string, error) {
	version, err := windows.Version(operatingSystem)
	if err != nil {
		return "", err
	}

	baseImage, ok := helperImages[version]
	if !ok {
		return "", windows.NewUnsupportedWindowsVersionError(operatingSystem)
	}

	return baseImage, nil
}

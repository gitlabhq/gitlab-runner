package helperimage

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/windows"
)

const (
	baseImage1809 = "servercore1809"
	baseImage21H2 = "servercore21H2"

	windowsSupportedArchitecture = "x86_64"
)

var helperImages = map[string]string{
	windows.V1809: baseImage1809,
	windows.V21H2: baseImage21H2,
	windows.V24H2: baseImage21H2, // Re-use the 21H2 base image, taking advantage of the backwards compatibility of newer windows kernels
}

var prebuiltImages = map[string]string{
	baseImage1809: "servercore-ltsc2019",
	baseImage21H2: "servercore-ltsc2022",
}

type windowsInfo struct{}

func (w *windowsInfo) Create(revision string, cfg Config) (Info, error) {
	baseImage, err := w.baseImage(cfg.KernelVersion)
	if err != nil {
		return Info{}, fmt.Errorf("detecting base image: %w", err)
	}

	var prebuilt string
	if name, ok := prebuiltImages[baseImage]; ok {
		prebuilt = fmt.Sprintf("prebuilt-windows-%s-%s", name, windowsSupportedArchitecture)
	}

	return Info{
		Architecture: windowsSupportedArchitecture,
		Name:         GitLabRegistryName,
		Tag:          fmt.Sprintf("%s-%s-%s", windowsSupportedArchitecture, revision, baseImage),
		Cmd:          getPowerShellCmd(cfg.Shell),
		Prebuilt:     prebuilt,
	}, nil
}

func (w *windowsInfo) baseImage(version string) (string, error) {
	version, err := windows.Version(version)
	if err != nil {
		return "", err
	}

	baseImage, ok := helperImages[version]
	if !ok {
		return "", fmt.Errorf("%w: %v", windows.ErrUnsupportedWindowsVersion, version)
	}

	return baseImage, nil
}

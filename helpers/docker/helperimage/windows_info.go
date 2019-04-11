package helperimage

import (
	"errors"
	"fmt"
	"strings"
)

const (
	windows1809 = "1809"
	windows1803 = "1803"

	baseImage1809 = "servercore1809"
	baseImage1803 = "servercore1803"

	windowsSupportedArchitecture = "x86_64"
)

var supportedOSVersions = map[string]string{
	windows1803: baseImage1803,
	windows1809: baseImage1809,
}

var ErrUnsupportedOSVersion = errors.New("could not determine windows version")

type windowsInfo struct{}

func (w *windowsInfo) Create(revision string, cfg Config) (Info, error) {
	osVersion, err := w.osVersion(cfg.OperatingSystem)
	if err != nil {
		return Info{}, err
	}

	return Info{
		Architecture: windowsSupportedArchitecture,
		Name:         name,
		Tag:          fmt.Sprintf("%s-%s-%s", windowsSupportedArchitecture, revision, osVersion),
		IsSupportingLocalImport: false,
	}, nil

}

func (w *windowsInfo) osVersion(operatingSystem string) (string, error) {
	for osVersion, baseImage := range supportedOSVersions {
		if strings.Contains(operatingSystem, fmt.Sprintf(" %s ", osVersion)) {
			return baseImage, nil
		}
	}

	return "", ErrUnsupportedOSVersion
}

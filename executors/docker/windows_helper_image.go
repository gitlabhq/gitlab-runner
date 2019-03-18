package docker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
)

const (
	windows1809 = "1809"
	windows1803 = "1803"

	nanoserver1809 = "nanoserver1809"
	nanoserver1803 = "nanoserver1803"

	windowsSupportedArchitecture = "x86_64"
)

var supportedOSVersions = map[string]string{
	windows1803: nanoserver1803,
	windows1809: nanoserver1809,
}

var ErrUnsupportedOSVersion = errors.New("could not determine windows version")

type windowsHelperImage struct {
	operatingSystem string
}

func (*windowsHelperImage) Architecture() string {
	return windowsSupportedArchitecture
}

func (u *windowsHelperImage) Tag(revision string) (string, error) {
	osVersion, err := u.osVersion()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%s-%s", u.Architecture(), revision, osVersion), nil
}

func (u *windowsHelperImage) osVersion() (string, error) {
	for operatingSystem, osVersion := range supportedOSVersions {
		if strings.Contains(u.operatingSystem, operatingSystem) {
			return osVersion, nil
		}
	}

	return "", ErrUnsupportedOSVersion
}

func (u *windowsHelperImage) IsSupportingLocalImport() bool {
	return false
}

func newWindowsHelperImage(info types.Info) helperImage {
	return &windowsHelperImage{
		operatingSystem: info.OperatingSystem,
	}
}

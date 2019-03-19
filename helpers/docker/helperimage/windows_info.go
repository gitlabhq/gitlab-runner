package helperimage

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

type windowsInfo struct {
	operatingSystem string
}

func (*windowsInfo) Architecture() string {
	return windowsSupportedArchitecture
}

func (u *windowsInfo) Tag(revision string) (string, error) {
	osVersion, err := u.osVersion()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%s-%s", u.Architecture(), revision, osVersion), nil
}

func (u *windowsInfo) osVersion() (string, error) {
	for operatingSystem, osVersion := range supportedOSVersions {
		if strings.Contains(u.operatingSystem, operatingSystem) {
			return osVersion, nil
		}
	}

	return "", ErrUnsupportedOSVersion
}

func (u *windowsInfo) IsSupportingLocalImport() bool {
	return false
}

func newWindowsInfo(info types.Info) Info {
	return &windowsInfo{
		operatingSystem: info.OperatingSystem,
	}
}

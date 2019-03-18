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

type windowsHelperImage struct {
	osType string
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

func (u *windowsHelperImage) IsSupportingLocalImport() bool {
	return false
}

func (u *windowsHelperImage) osVersion() (string, error) {
	switch {
	case strings.Contains(u.osType, windows1809):
		return nanoserver1809, nil
	case strings.Contains(u.osType, windows1803):
		return nanoserver1803, nil
	}

	return "", errors.New("could not determine windows version")
}

func newWindowsHelperImage(info types.Info) helperImage {
	return &windowsHelperImage{
		osType: info.OSType,
	}
}

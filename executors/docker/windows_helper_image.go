package docker

import (
	"errors"
	"fmt"
	"strings"
)

const (
	windows1809 = "1809"
	windows1803 = "1803"

	nanoserver1809 = "nanoserver1809"
	nanoserver1803 = "nanoserver1803"
)

type windowsHelperImage struct {
	osType string
}

func (*windowsHelperImage) Architecture() string {
	return "x86_64"
}

func (u *windowsHelperImage) Tag(revision string) (string, error) {
	version, err := u.helperImageVersion()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%s-%s", u.Architecture(), revision, version), nil
}

func (u *windowsHelperImage) helperImageVersion() (string, error) {
	switch {
	case strings.Contains(u.osType, windows1809):
		return nanoserver1809, nil
	case strings.Contains(u.osType, windows1803):
		return nanoserver1803, nil
	}

	return "", errors.New("could not find windows version")
}

func newWindowsHelperImage(osType string) *windowsHelperImage {
	return &windowsHelperImage{
		osType: osType,
	}
}

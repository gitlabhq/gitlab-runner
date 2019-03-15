package docker

import (
	"fmt"
	"runtime"
)

type unixHelperImage struct {
	dockerArch string
}

func (u *unixHelperImage) Architecture() string {
	switch u.dockerArch {
	case "armv6l", "armv7l", "aarch64":
		return "arm"
	case "amd64":
		return "x86_64"
	}

	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	default:
		return runtime.GOARCH
	}
}

func (u *unixHelperImage) Tag(revision string) (string, error) {
	return fmt.Sprintf("%s-%s", u.Architecture(), revision), nil
}

func (u *unixHelperImage) IsSupportingLocalImport() bool {
	return true
}

func newUnixHelperImage(dockerArch string) helperImage {
	return &unixHelperImage{
		dockerArch: dockerArch,
	}
}

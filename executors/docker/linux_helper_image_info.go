package docker

import (
	"fmt"
	"runtime"

	"github.com/docker/docker/api/types"
)

type linuxHelperImageInfo struct {
	dockerArch string
}

func (u *linuxHelperImageInfo) Architecture() string {
	switch u.dockerArch {
	case "armv6l", "armv7l", "aarch64":
		return "arm"
	case "amd64":
		return "x86_64"
	}

	if u.dockerArch != "" {
		return u.dockerArch
	}

	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	default:
		return runtime.GOARCH
	}
}

func (u *linuxHelperImageInfo) Tag(revision string) (string, error) {
	return fmt.Sprintf("%s-%s", u.Architecture(), revision), nil
}

func (u *linuxHelperImageInfo) IsSupportingLocalImport() bool {
	return true
}

func newLinuxHelperImageInfo(info types.Info) helperImageInfo {
	return &linuxHelperImageInfo{
		dockerArch: info.Architecture,
	}
}

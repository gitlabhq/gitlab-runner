package helperimage

import (
	"fmt"
	"runtime"

	"github.com/docker/docker/api/types"
)

type linuxInfo struct {
	dockerArch string
}

func (u *linuxInfo) Architecture() string {
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

func (u *linuxInfo) Tag(revision string) (string, error) {
	return fmt.Sprintf("%s-%s", u.Architecture(), revision), nil
}

func (u *linuxInfo) IsSupportingLocalImport() bool {
	return true
}

func newLinuxInfo(info types.Info) Info {
	return &linuxInfo{
		dockerArch: info.Architecture,
	}
}

package helperimage

import (
	"fmt"
	"runtime"
)

const (
	platformAmd64   = "amd64"
	platformArm6vl  = "armv6l"
	platformArmv7l  = "armv7l"
	platformAarch64 = "aarch64"
	archX8664       = "x86_64"
	archArm         = "arm"
	archArm64       = "arm64"
)

var bashCmd = []string{"gitlab-runner-build"}

type linuxInfo struct{}

func (l *linuxInfo) Create(revision string, cfg Config) (Info, error) {
	arch := l.architecture(cfg.Architecture)

	return Info{
		Architecture:            arch,
		Name:                    name,
		Tag:                     fmt.Sprintf("%s-%s", arch, revision),
		IsSupportingLocalImport: true,
		Cmd:                     bashCmd,
	}, nil
}

func (l *linuxInfo) architecture(arch string) string {
	switch arch {
	case platformArm6vl, platformArmv7l:
		return archArm
	case platformAarch64:
		return archArm64
	case platformAmd64:
		return archX8664
	}

	if arch != "" {
		return arch
	}

	switch runtime.GOARCH {
	case platformAmd64:
		return archX8664
	default:
		return runtime.GOARCH
	}
}

package helperimage

import (
	"fmt"
	"runtime"
)

type linuxInfo struct{}

func (l *linuxInfo) Create(revision string, cfg Config) (Info, error) {
	arch := l.architecture(cfg.Architecture)

	return Info{
		Architecture: arch,
		Name:         name,
		Tag:          fmt.Sprintf("%s-%s", arch, revision),
		IsSupportingLocalImport: true,
	}, nil

}

func (l *linuxInfo) architecture(arch string) string {
	switch arch {
	case "armv6l", "armv7l", "aarch64":
		return "arm"
	case "amd64":
		return "x86_64"
	}

	if arch != "" {
		return arch
	}

	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	default:
		return runtime.GOARCH
	}
}

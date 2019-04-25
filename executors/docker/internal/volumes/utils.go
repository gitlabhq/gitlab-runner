package volumes

import (
	"crypto/md5"
	"fmt"
	"path"
	"strings"
)

func IsHostMountedVolume(dir string, volumes ...string) bool {
	for _, volume := range volumes {
		hostVolume := strings.Split(volume, ":")

		if len(hostVolume) < 2 {
			continue
		}

		if isParentOf(path.Clean(hostVolume[1]), path.Clean(dir)) {
			return true
		}
	}
	return false
}

func isParentOf(parent string, dir string) bool {
	for dir != "/" && dir != "." {
		if dir == parent {
			return true
		}
		dir = path.Dir(dir)
	}
	return false
}

func hashContainerPath(containerPath string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(containerPath)))
}

type ErrVolumeAlreadyDefined struct {
	containerPath string
}

func (e *ErrVolumeAlreadyDefined) Error() string {
	return fmt.Sprintf("volume for container path %q is already defined", e.containerPath)
}

func NewErrVolumeAlreadyDefined(containerPath string) *ErrVolumeAlreadyDefined {
	return &ErrVolumeAlreadyDefined{
		containerPath: containerPath,
	}
}

type pathList map[string]bool

func (m pathList) Add(containerPath string) error {
	if m[containerPath] {
		return NewErrVolumeAlreadyDefined(containerPath)
	}

	m[containerPath] = true

	return nil
}

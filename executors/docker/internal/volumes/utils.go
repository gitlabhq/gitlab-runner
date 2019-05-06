package volumes

import (
	"crypto/md5"
	"fmt"
	"path/filepath"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
)

type debugLogger interface {
	Debugln(args ...interface{})
}

func IsHostMountedVolume(volumeParser parser.Parser, dir string, volumes ...string) (bool, error) {
	for _, volume := range volumes {
		parsedVolume, err := volumeParser.ParseVolume(volume)
		if err != nil {
			return false, err
		}

		if parsedVolume.Len() < 2 {
			continue
		}

		if isParentOf(filepath.Clean(parsedVolume.Destination), filepath.Clean(dir)) {
			return true, nil
		}
	}
	return false, nil
}

func isParentOf(parent string, dir string) bool {
	for dir != "/" && dir != "." {
		if dir == parent {
			return true
		}
		dir = filepath.Dir(dir)
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

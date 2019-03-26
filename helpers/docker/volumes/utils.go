package volumes

import (
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

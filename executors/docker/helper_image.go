package docker

import (
	"github.com/docker/docker/api/types"
)

// helperImage provides information about the helper image that can be used to
// pull from Docker Hub.
type helperImage interface {
	Architecture() string
	Tag(revision string) (string, error)
	IsSupportingLocalImport() bool
}

func getHelperImage(info types.Info) helperImage {
	if info.OSType == "windows" {
		return newWindowsHelperImage(info)
	}

	return newLinuxHelperImage(info)
}

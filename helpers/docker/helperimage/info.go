package helperimage

import (
	"github.com/docker/docker/api/types"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/errors"
)

const (
	OSTypeLinux   = "linux"
	OSTypeWindows = "windows"
)

// Info provides information about the helper image that can be used to
// pull from Docker Hub.
type Info interface {
	Architecture() string
	Tag(revision string) (string, error)
	IsSupportingLocalImport() bool
}

type infoFactory func(info types.Info) Info

var supportedOsTypesFactories = map[string]infoFactory{
	OSTypeWindows: newWindowsInfo,
	OSTypeLinux:   newLinuxInfo,
}

func GetInfo(info types.Info) (Info, error) {
	factory, ok := supportedOsTypesFactories[info.OSType]
	if !ok {
		return nil, errors.NewErrOSNotSupported(info.OSType)
	}

	return factory(info), nil
}

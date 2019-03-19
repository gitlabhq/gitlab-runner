package docker

import (
	"fmt"

	"github.com/docker/docker/api/types"
)

const (
	OSTypeLinux   = "linux"
	OSTypeWindows = "windows"
)

// helperImageInfo provides information about the helper image that can be used to
// pull from Docker Hub.
type helperImageInfo interface {
	Architecture() string
	Tag(revision string) (string, error)
	IsSupportingLocalImport() bool
}

type unsupportedOSTypeError struct {
	detectedOSType string
}

func (e *unsupportedOSTypeError) Error() string {
	return fmt.Sprintf("unsupported OSType %q", e.detectedOSType)
}

func (e *unsupportedOSTypeError) OSType(osType string) *unsupportedOSTypeError {
	e.detectedOSType = osType

	return e
}

var errUnsupportedOSType = &unsupportedOSTypeError{}

type helperImageInfoFactory func(info types.Info) helperImageInfo

var supportedOsTypesFactories = map[string]helperImageInfoFactory{
	OSTypeWindows: newWindowsHelperImageInfo,
	OSTypeLinux:   newLinuxHelperImageInfo,
}

func getHelperImageInfo(info types.Info) (helperImageInfo, error) {
	osType := info.OSType
	factory, ok := supportedOsTypesFactories[osType]
	if !ok {
		return nil, errUnsupportedOSType.OSType(osType)
	}

	return factory(info), nil
}

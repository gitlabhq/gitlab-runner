package docker

import (
	"fmt"

	"github.com/docker/docker/api/types"
)

const (
	OSTypeLinux   = "linux"
	OSTypeWindows = "windows"
)

// helperImage provides information about the helper image that can be used to
// pull from Docker Hub.
type helperImage interface {
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

type helperImageFactory func(info types.Info) helperImage

var supportedOsTypesFactories = map[string]helperImageFactory{
	OSTypeWindows: newWindowsHelperImage,
	OSTypeLinux:   newLinuxHelperImage,
}

func getHelperImage(info types.Info) (helperImage, error) {
	osType := info.OSType
	factory, ok := supportedOsTypesFactories[osType]
	if !ok {
		return nil, errUnsupportedOSType.OSType(osType)
	}

	return factory(info), nil
}

package helperimage

import (
	"fmt"

	"github.com/docker/docker/api/types"
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

type unsupportedOSTypeError struct {
	detectedOSType string
}

func (e *unsupportedOSTypeError) Error() string {
	return fmt.Sprintf("unsupported OSType %q", e.detectedOSType)
}

func newUnsupportedOSTypeError(osType string) *unsupportedOSTypeError {
	return &unsupportedOSTypeError{
		detectedOSType: osType,
	}
}

type infoFactory func(info types.Info) Info

var supportedOsTypesFactories = map[string]infoFactory{
	OSTypeWindows: newWindowsInfo,
	OSTypeLinux:   newLinuxInfo,
}

func GetInfo(info types.Info) (Info, error) {
	factory, ok := supportedOsTypesFactories[info.OSType]
	if !ok {
		return nil, newUnsupportedOSTypeError(info.OSType)
	}

	return factory(info), nil
}

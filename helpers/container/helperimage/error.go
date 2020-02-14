package helperimage

import "fmt"

var unsupportedOSVersionErrorMessage = "unsupported Windows version: %v"

type ErrUnsupportedOSVersion struct {
	version string
}

func NewErrUnsupportedOSVersion(version string) error {
	return &ErrUnsupportedOSVersion{version: version}
}

func (d *ErrUnsupportedOSVersion) Error() string {
	return fmt.Sprintf(unsupportedOSVersionErrorMessage, d.version)
}

func (d *ErrUnsupportedOSVersion) Is(err error) bool {
	_, ok := err.(*ErrUnsupportedOSVersion)
	return ok
}

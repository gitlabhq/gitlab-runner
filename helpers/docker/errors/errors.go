package errors

import (
	"fmt"
)

// ErrOSNotSupported is used when docker does not support the detected OSType.
// NewErrOSNotSupported is used to initialize this type.
type ErrOSNotSupported struct {
	detectedOSType string
}

func (e *ErrOSNotSupported) Error() string {
	return fmt.Sprintf("unsupported OSType %q", e.detectedOSType)
}

func (e *ErrOSNotSupported) Is(err error) bool {
	_, ok := err.(*ErrOSNotSupported)

	return ok
}

// NewErrOSNotSupported creates a ErrOSNotSupported for the specified OSType.
func NewErrOSNotSupported(osType string) *ErrOSNotSupported {
	return &ErrOSNotSupported{
		detectedOSType: osType,
	}
}

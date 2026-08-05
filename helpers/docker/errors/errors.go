package errors

import (
	"fmt"
)

// OSNotSupportedError is used when docker does not support the detected OSType.
// NewOSNotSupportedError is used to initialize this type.
type OSNotSupportedError struct {
	detectedOSType string
}

func (e *OSNotSupportedError) Error() string {
	return fmt.Sprintf("unsupported OSType %q", e.detectedOSType)
}

func (e *OSNotSupportedError) Is(err error) bool {
	_, ok := err.(*OSNotSupportedError)

	return ok
}

// NewOSNotSupportedError creates an OSNotSupportedError for the specified OSType.
func NewOSNotSupportedError(osType string) *OSNotSupportedError {
	return &OSNotSupportedError{
		detectedOSType: osType,
	}
}

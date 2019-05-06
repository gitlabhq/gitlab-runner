package parser

import (
	"fmt"
)

type InvalidVolumeSpecError struct {
	spec string
}

func (e *InvalidVolumeSpecError) Error() string {
	return fmt.Sprintf("invalid volume specification: %q", e.spec)
}

func NewInvalidVolumeSpecErr(spec string) error {
	return &InvalidVolumeSpecError{
		spec: spec,
	}
}

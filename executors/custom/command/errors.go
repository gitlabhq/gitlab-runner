package command

import (
	"fmt"
)

type UnknownFailureError struct {
	Inner    error
	ExitCode int
}

func (e *UnknownFailureError) Error() string {
	return fmt.Sprintf(
		"unknown Custom executor executable exit code %d; executable execution terminated with: %v",
		e.ExitCode,
		e.Inner,
	)
}

package command

import (
	"fmt"
)

type ErrUnknownFailure struct {
	Inner    error
	ExitCode int
}

func (e *ErrUnknownFailure) Error() string {
	return fmt.Sprintf(
		"unknown Custom executor executable exit code %d; executable execution terminated with: %v",
		e.ExitCode,
		e.Inner,
	)
}

package git

import (
	"fmt"

	"gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog/commander"
)

type Error struct {
	command string
	inner   error
}

func NewError(command string, err error) *Error {
	return &Error{
		command: command,
		inner:   err,
	}
}

func (e *Error) Error() string {
	return fmt.Sprintf("executing `%s` git command exited with error: %v", e.command, e.inner)
}

func (e *Error) Unwrap() error {
	return e.inner
}

func (e *Error) Is(err error) bool {
	_, ok := err.(*Error)

	return ok
}

type Git interface {
	Describe(opts *DescribeOpts) (string, error)
	Log(query string, opts *LogOpts) ([]byte, error)
}

func New() Git {
	return NewWithCommanderFactory(commander.New)
}

func NewWithCommanderFactory(newCommander commander.Factory) Git {
	return &gitCommand{
		newCommander: newCommander,
	}
}

type gitCommand struct {
	newCommander commander.Factory
}

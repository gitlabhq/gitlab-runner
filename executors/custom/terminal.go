//go:build !windows

package custom

import (
	"errors"

	terminalsession "gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

func (e *executor) TerminalConnect() (terminalsession.Conn, error) {
	return nil, errors.New("not yet supported")
}

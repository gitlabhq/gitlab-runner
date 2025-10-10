package machine

import (
	"errors"

	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

func (e *machineExecutor) TerminalConnect() (terminal.Conn, error) {
	if term, ok := e.executor.(terminal.InteractiveTerminal); ok {
		return term.TerminalConnect()
	}

	return nil, errors.New("executor does not have terminal")
}

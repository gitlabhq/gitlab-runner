//go:build !integration

package machine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

func TestMachineExecutor_Connect_NoTerminal(t *testing.T) {
	e := machineExecutor{
		executor: common.NewMockExecutor(t),
	}

	conn, err := e.TerminalConnect()
	assert.Error(t, err)
	assert.Nil(t, conn)
}

type mockTerminalExecutor struct {
	*common.MockExecutor
	*terminal.MockInteractiveTerminal
}

func TestMachineExecutor_Connect_Terminal(t *testing.T) {
	mock := mockTerminalExecutor{
		MockExecutor:            common.NewMockExecutor(t),
		MockInteractiveTerminal: terminal.NewMockInteractiveTerminal(t),
	}
	e := machineExecutor{
		executor: &mock,
	}
	mock.MockInteractiveTerminal.On("TerminalConnect").Return(terminal.NewMockConn(t), nil).Once()

	conn, err := e.TerminalConnect()
	assert.NoError(t, err)
	assert.NotNil(t, conn)
}

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
		executor: &common.MockExecutor{},
	}

	conn, err := e.Connect()
	assert.Error(t, err)
	assert.Nil(t, conn)
}

type mockTerminalExecutor struct {
	common.MockExecutor
	terminal.MockInteractiveTerminal
}

func TestMachineExecutor_Connect_Terminal(t *testing.T) {
	mock := mockTerminalExecutor{}
	e := machineExecutor{
		executor: &mock,
	}
	mock.MockInteractiveTerminal.On("Connect").Return(&terminal.MockConn{}, nil).Once()

	conn, err := e.Connect()
	assert.NoError(t, err)
	assert.NotNil(t, conn)
	mock.MockInteractiveTerminal.AssertCalled(t, "Connect")
}

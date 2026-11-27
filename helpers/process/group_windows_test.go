//go:build !integration

package process

import (
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetProcessGroup(t *testing.T) {
	cmd := exec.Command("sleep", "1")

	require.Nil(t, cmd.SysProcAttr)
	setProcessGroup(cmd)

	require.NotNil(t, cmd.SysProcAttr)
	assert.Equal(t, uint32(syscall.CREATE_NEW_PROCESS_GROUP), cmd.SysProcAttr.CreationFlags)
}

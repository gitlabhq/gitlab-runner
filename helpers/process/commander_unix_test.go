// +build darwin dragonfly freebsd linux netbsd openbsd

package process

import (
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_cmd_Start(t *testing.T) {
	c := osCmd{
		internal: &exec.Cmd{
			SysProcAttr: &syscall.SysProcAttr{
				Setpgid: false,
			},
		},
	}
	require.False(t, c.internal.SysProcAttr.Setpgid)
	_ = c.Start()
	assert.True(t, c.internal.SysProcAttr.Setpgid)
}

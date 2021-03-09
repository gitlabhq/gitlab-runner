// +build darwin dragonfly freebsd linux netbsd openbsd

package process

import (
	"os/exec"
	"syscall"
)

func setProcessGroup(c *exec.Cmd, _ bool) {
	c.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

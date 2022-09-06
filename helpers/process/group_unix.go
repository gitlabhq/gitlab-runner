//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris

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

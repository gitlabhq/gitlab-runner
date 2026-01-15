//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris || zos

package process

import (
	"os"
	"os/exec"
	"syscall"
)

type osCmd struct {
	internal *exec.Cmd
	options  CommandOptions
}

func (c *osCmd) Start() error {
	setProcessGroup(c.internal)
	return c.internal.Start()
}

func (c *osCmd) Wait() error {
	return c.internal.Wait()
}

func (c *osCmd) Process() *os.Process {
	return c.internal.Process
}

func newOSCmd(c *exec.Cmd, options CommandOptions) Commander {
	return &osCmd{
		internal: c,
		options:  options,
	}
}

func setProcessGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

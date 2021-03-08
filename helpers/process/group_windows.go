package process

import (
	"os/exec"
	"syscall"
)

func setProcessGroup(c *exec.Cmd, useLegacyStrategy bool) {
	if useLegacyStrategy {
		return
	}

	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

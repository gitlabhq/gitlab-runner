// +build darwin dragonfly freebsd linux netbsd openbsd

package helpers

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

func SetProcessGroup(cmd *exec.Cmd) {
	// Create process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func KillProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	process := cmd.Process
	if process != nil {
		if process.Pid > 0 {
			unix.Kill(-process.Pid, unix.SIGKILL)
		} else {
			// doing normal kill
			process.Kill()
		}
	}
}

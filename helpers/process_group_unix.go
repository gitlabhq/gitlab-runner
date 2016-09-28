// +build darwin dragonfly freebsd linux netbsd openbsd

package helpers

import (
	"os/exec"
	"syscall"
	"time"
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

	waitCh := make(chan error)
	go func() {
		waitCh <- cmd.Wait()
	}()

	process := cmd.Process
	if process != nil {
		if process.Pid > 0 {
			syscall.Kill(-process.Pid, syscall.SIGTERM)
			select {
			case <-waitCh:
				return
			case <-time.After(time.Second * 15):
				syscall.Kill(-process.Pid, syscall.SIGKILL)
			}
		} else {
			// doing normal kill
			process.Kill()
		}
	}
}

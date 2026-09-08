//go:build !windows

package docker

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// Own process group, so an init that signals the whole group
// (dumb-init, tini -g) cannot kill docker-machine mid-operation.
func setDockerMachineProcessAttributes(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		// Once Wait has reaped docker-machine its pid, and so the pgid,
		// may be reused; os.Process tracks that. An orphaned driver
		// plugin exits by itself when the heartbeat stops.
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			return err
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}

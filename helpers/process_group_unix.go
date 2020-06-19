// +build darwin dragonfly freebsd linux netbsd openbsd

package helpers

import (
	"os/exec"
	"syscall"
)

// TODO: Remove in 14.0 https://gitlab.com/gitlab-org/gitlab-runner/issues/6413
func SetProcessGroup(cmd *exec.Cmd) {
	// Create process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// TODO: Remove in 14.0 https://gitlab.com/gitlab-org/gitlab-runner/issues/6413
func KillProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	process := cmd.Process
	if process != nil {
		if process.Pid > 0 {
			_ = syscall.Kill(-process.Pid, syscall.SIGKILL)
		} else {
			// doing normal kill
			_ = process.Kill()
		}
	}
}

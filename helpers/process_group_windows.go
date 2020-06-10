package helpers

import (
	"os/exec"
	"strconv"
)

// TODO: Remove in 14.0 https://gitlab.com/gitlab-org/gitlab-runner/issues/6413
func SetProcessGroup(cmd *exec.Cmd) {
}

// TODO: Remove in 14.0 https://gitlab.com/gitlab-org/gitlab-runner/issues/6413
func KillProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	cmd.Process.Kill()
}

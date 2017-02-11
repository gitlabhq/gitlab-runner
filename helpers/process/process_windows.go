package process

import (
	"os/exec"
	"strconv"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

func SetProcessGroup(cmd *exec.Cmd) {
}

func SetCredential(cmd *exec.Cmd, shell *common.ShellConfiguration) {
}

func KillProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	pid := cmd.Process.Pid
	log(pid, "Killing process")

	exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
	cmd.Process.Kill()
}

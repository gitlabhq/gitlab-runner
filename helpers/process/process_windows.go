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

	exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	cmd.Process.Kill()
}

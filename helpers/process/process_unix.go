// +build darwin dragonfly freebsd linux netbsd openbsd

package process

import (
	"os/exec"
	"syscall"
	"time"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

const ProcessKillWaitTime = 10 * time.Second

func SetProcessGroup(cmd *exec.Cmd) {
	prepareSysProcAttr(cmd)

	// Create process group
	cmd.SysProcAttr.Setpgid = true
}

func SetCredential(cmd *exec.Cmd, shell *common.ShellConfiguration) {
	prepareSysProcAttr(cmd)

	if shell.CommandCredential == nil {
		return
	}

	// Set Credential - run the command in context of UID and GID
	cmd.SysProcAttr.Credential = &syscall.Credential{
		Uid: shell.CommandCredential.UID,
		Gid: shell.CommandCredential.GID,
	}
}

func prepareSysProcAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
}

func KillProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	waitCh := make(chan error)
	go func() {
		waitCh <- cmd.Wait()
		close(waitCh)
	}()

	process := cmd.Process
	if process != nil {
		if process.Pid > 0 {
			syscall.Kill(-process.Pid, syscall.SIGTERM)
			select {
			case <-waitCh:
				return
			case <-time.After(ProcessKillWaitTime):
				syscall.Kill(-process.Pid, syscall.SIGKILL)
			}
		} else {
			// doing normal kill
			process.Kill()
		}
	}

	select {
	case <-waitCh:
		return
	case <-time.After(ProcessKillWaitTime):
		panic("Process couldn't be killed!")
	}
}

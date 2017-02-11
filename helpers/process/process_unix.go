// +build darwin dragonfly freebsd linux netbsd openbsd

package process

import (
	"os/exec"
	"syscall"
	"time"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

var ProcessKillWaitTime = 10 * time.Second
var ProcessLeftoversLookupWaitTime = 10 * time.Millisecond

func SetProcessGroup(cmd *exec.Cmd) {
	prepareSysProcAttr(cmd)

	// Create process group
	cmd.SysProcAttr.Setpgid = true
}

func SetCredential(cmd *exec.Cmd, shell *common.ShellConfiguration) {
	if shell.CommandCredential == nil {
		return
	}

	prepareSysProcAttr(cmd)

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
	pid := process.Pid
	if process != nil {
		log(pid, "Killing process")
		if pid > 0 {
			log(pid, "Sending SIGTERM to process group")
			syscall.Kill(-pid, syscall.SIGTERM)
			select {
			case <-waitCh:
				log(pid, "Main process exited after SIGTERM")
			case <-time.After(ProcessKillWaitTime):
				log(pid, "SIGTERM timeouted, sending SIGKILL to process group")
				syscall.Kill(-pid, syscall.SIGKILL)
			}
		} else {
			// doing normal kill
			process.Kill()
		}
	}

	if !leftoversPresent(pid) {
		return
	}

	log(pid, "Found leftovers, sending SIGKILL to process group")
	syscall.Kill(-pid, syscall.SIGKILL)

	if !leftoversPresent(pid) {
		return
	}

	panic("Process couldn't be killed!")
}

func leftoversPresent(pid int) bool {
	log(pid, "Looking for leftovers")
	time.Sleep(ProcessLeftoversLookupWaitTime)

	err := syscall.Kill(-pid, syscall.Signal(0))
	if err != nil {
		log(pid, "No leftovers, process terminated")
		return false
	}

	return true
}

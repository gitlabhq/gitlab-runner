//go:build windows

package ssh

import (
	"os/exec"
	"syscall"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
	"golang.org/x/sys/windows"
)

func runCmd(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	jobObject, err := process.CreateJobObject()
	if err != nil {
		return err
	}

	cmd.Cancel = func() error {
		return windows.CloseHandle(jobObject)
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := process.AssignPidToJobObject(cmd.Process.Pid, jobObject); err != nil {
		return err
	}

	return cmd.Wait()
}

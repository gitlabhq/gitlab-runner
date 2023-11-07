//go:build windows

package ssh

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func runCmd(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	handle, err := createJobObject()
	if err != nil {
		return err
	}

	cmd.Cancel = func() error {
		return windows.CloseHandle(windows.Handle(handle))
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	err = windows.AssignProcessToJobObject(
		windows.Handle(handle),
		windows.Handle((*process)(unsafe.Pointer(cmd.Process)).Handle))
	if err != nil {
		return fmt.Errorf("assigning process to job object: %w", err)
	}

	return cmd.Wait()
}

type process struct {
	Pid    int
	Handle uintptr
}

func createJobObject() (uintptr, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}

	_, err = windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)))

	return uintptr(handle), err
}

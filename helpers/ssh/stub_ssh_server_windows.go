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

	jobObject, err := createJobObject()
	if err != nil {
		return err
	}

	cmd.Cancel = func() error {
		return windows.CloseHandle(windows.Handle(jobObject))
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := assignProcessToJobObject(cmd.Process.Pid, jobObject); err != nil {
		return fmt.Errorf("assigning process to job object: %w", err)
	}

	return cmd.Wait()
}

func createJobObject() (uintptr, error) {
	jobObj, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("creating job object: %w", err)
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}

	if _, err = windows.SetInformationJobObject(
		jobObj,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info))); err != nil {
		return 0, fmt.Errorf("setting job object information: %w", err)
	}

	return uintptr(jobObj), nil
}

func assignProcessToJobObject(pid int, jobObject uintptr) error {
	procHandle, err := findProcessHandleFromPID(pid)
	if err != nil {
		return fmt.Errorf("failed to retrieve handle for process: %w", err)
	}

	if err = windows.AssignProcessToJobObject(windows.Handle(jobObject), windows.Handle(procHandle)); err != nil {
		return fmt.Errorf("failed to assign process to job: %w", err)
	}
	return nil
}

func findProcessHandleFromPID(pid int) (uintptr, error) {
	const da = windows.PROCESS_TERMINATE | windows.PROCESS_SET_QUOTA
	h, err := syscall.OpenProcess(da, false, uint32(pid))
	if err != nil {
		return 0, fmt.Errorf("calling OpenProcess: %w", err)
	}
	if uintptr(h) == 0 {
		return 0, fmt.Errorf("getting process handle for pid %q", pid)
	}
	return uintptr(h), nil
}

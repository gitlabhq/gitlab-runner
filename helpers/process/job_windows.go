package process

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type osCmd struct {
	internal *exec.Cmd
	options  CommandOptions

	// A job object to helper ensure processes are killed, plus a Once
	// to ensure the job object is only closed one.
	jobObject uintptr
	once      sync.Once
}

func (c *osCmd) Start() error {
	setProcessGroup(c.internal, c.options.UseWindowsLegacyProcessStrategy)

	if c.options.UseWindowsJobObject {
		jobObj, err := createJobObject()
		if err != nil {
			return fmt.Errorf("starting OS command: %w", err)
		}
		c.jobObject = jobObj
	}

	err := c.internal.Start()
	if err != nil {
		return fmt.Errorf("starting OS command: %w", err)
	}

	if c.options.UseWindowsJobObject {
		// Any failures here are ignored, since we've already started the process running.
		if err := assignProcessToJobObject(c.internal.Process.Pid, c.jobObject); err != nil {
			c.options.Logger.Warn("assigning process to job object:", err)
		}
	}
	return nil
}

func (c *osCmd) Wait() error {
	err := c.internal.Wait()
	c.closeJobObject()
	return err
}

func (c *osCmd) Process() *os.Process {
	return c.internal.Process
}

func newOSCmd(c *exec.Cmd, options CommandOptions) Commander {
	return &osCmd{
		internal: c,
		options:  options,
	}
}

func (c *osCmd) closeJobObject() {
	if !c.options.UseWindowsJobObject {
		return
	}
	c.once.Do(func() {
		windows.CloseHandle(windows.Handle(c.jobObject))
	})
}

func setProcessGroup(c *exec.Cmd, useLegacyStrategy bool) {
	if useLegacyStrategy {
		return
	}

	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
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

// Assign the process with specified PID to the specified job object. Processes created as children of that one will
// also be assigned to the job. When the last handle on the job is closed, all associated processes will be terminated.
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

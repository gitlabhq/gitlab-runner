package process

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

type osCmd struct {
	internal *exec.Cmd
	options  CommandOptions

	// A job object to helper ensure processes are killed, plus a Once
	// to ensure the job object is only closed one.
	jobObject windows.Handle
	once      sync.Once
}

func (c *osCmd) Start() error {
	setProcessGroup(c.internal, c.options.UseWindowsLegacyProcessStrategy)

	if c.options.UseWindowsJobObject {
		jobObj, err := CreateJobObject()
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
		if err := AssignPidToJobObject(c.internal.Process.Pid, c.jobObject); err != nil {
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
		windows.CloseHandle(c.jobObject)
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

func CreateJobObject() (windows.Handle, error) {
	jobObj, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("creating job object: %w", err)
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE |
				windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK, // Allow subprocesses to explicitly avoid termination using CREATE_BREAKAWAY_FROM_JOB
		},
	}

	if _, err = windows.SetInformationJobObject(
		jobObj,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info))); err != nil {
		return 0, fmt.Errorf("setting job object information: %w", err)
	}

	return jobObj, nil
}

func AssignProcessToJobObject(processHandle windows.Handle, jobObject windows.Handle) error {
	if err := windows.AssignProcessToJobObject(jobObject, processHandle); err != nil {
		return fmt.Errorf("failed to assign process to job: %w", err)
	}

	return nil
}

// Assign the process with specified PID to the specified job object. Processes created as children of that one will
// also be assigned to the job. When the last handle on the job is closed, all associated processes will be terminated.
func AssignPidToJobObject(pid int, jobObject windows.Handle) error {
	procHandle, err := FindProcessHandleFromPID(pid)
	if err != nil {
		return fmt.Errorf("failed to retrieve handle for process: %w", err)
	}
	defer windows.CloseHandle(procHandle)

	return AssignProcessToJobObject(procHandle, jobObject)
}

func FindProcessHandleFromPID(pid int) (windows.Handle, error) {
	const desiredAccess = windows.PROCESS_TERMINATE | windows.PROCESS_SET_QUOTA | windows.SYNCHRONIZE
	handle, err := windows.OpenProcess(desiredAccess, false, uint32(pid))
	if err != nil {
		return 0, fmt.Errorf("calling OpenProcess: %w", err)
	}

	return handle, nil
}

// EnsureSubprocessTerminationOnExit This ensures that all runner subprocesses are terminated if the runner process stops for any reason
// This ensures no stale CI jobs linger and use the same CI job directories
func EnsureSubprocessTerminationOnExit() error {
	const MinWindowsVersionSupportingNestedJobs uint32 = 9200 // Windows 8 / Server 2012

	// To support per-command Windows Job wrapping as well, we wrap the whole runner in a job only if nesting is supported.
	version := windows.RtlGetVersion()
	if version.BuildNumber < MinWindowsVersionSupportingNestedJobs {
		logrus.Warn("Windows version is too old, skipping process encapsulation.\nPlease upgrade to a supported windows version: https://docs.gitlab.com/runner/install/support-policy/#windows-version-support")
		return nil
	}

	jobObject, err := CreateJobObject()
	if err != nil {
		return fmt.Errorf("creating job object: %w", err)
	}

	if err := AssignProcessToJobObject(windows.CurrentProcess(), jobObject); err != nil {
		_ = windows.CloseHandle(jobObject)
		return fmt.Errorf("assigning process to job object: %w", err)
	}

	// Intentionally leak the job handle. It should only be closed on process termination.
	return nil
}

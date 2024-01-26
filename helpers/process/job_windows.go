package process

import (
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
		err := c.createJobObject()
		if err != nil {
			return err
		}
	}

	err := c.internal.Start()
	if err != nil {
		return err
	}

	if c.options.UseWindowsJobObject {
		// Any failures here are ignored, since we've already started the process running.
		c.assignProcessToJobObject()
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

// Create a Job object.
func (c *osCmd) createJobObject() error {
	hJob, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		c.options.Logger.Warn("Failed to create job object:", err)
		return err
	}

	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

	_, err = windows.SetInformationJobObject(hJob, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		c.options.Logger.Warn("Failed to set job object information:", err)
		return err
	}

	c.jobObject = uintptr(hJob)
	return nil
}

// Assign the osCmd's process to the job object. Processes created as children of
// that one will also be assigned to the job. When the last handle on the job is closed,
// all associated processes will be terminated. The handle to the job object is saved
// in the osCmd.
func (c *osCmd) assignProcessToJobObject() {
	type Process struct {
		pid    int
		handle uintptr
		isdone uint32
		sigMu  sync.RWMutex
	}

	proc := (*Process)(unsafe.Pointer(c.internal.Process))
	if proc.handle == 0 {
		c.options.Logger.Warn("Failed to retrieve handle for process")
	}

	err := windows.AssignProcessToJobObject(windows.Handle(c.jobObject), windows.Handle(proc.handle))
	if err != nil {
		c.options.Logger.Warn("Failed to assign process to job:", err)
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

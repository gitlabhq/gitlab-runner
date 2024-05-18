package process

import (
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"unsafe"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/sys/windows"
)

type windowsKiller struct {
	logger Logger
	cmd    osCmd
}

func newKiller(logger Logger, cmd Commander) killer {
	osCmd, ok := cmd.(*osCmd)
	if !ok {
		panic("Failed to convert Commander to osCmd")
	}

	return &windowsKiller{
		logger: logger,
		cmd:    *osCmd,
	}
}

func (pk *windowsKiller) Terminate() {
	if pk.cmd.Process() == nil {
		return
	}

	if err := taskTerminate(pk.cmd.Process().Pid, pk.cmd.options.UseWindowsLegacyProcessStrategy); err != nil {
		pk.logger.Warn("Failed to terminate process:", err)

		// try to kill right-after
		pk.ForceKill()
	}
}

func (pk *windowsKiller) ForceKill() {
	if pk.cmd.Process() == nil {
		return
	}

	err := taskKill(pk.cmd.Process().Pid)
	if err != nil {
		pk.logger.Warn("Failed to force-kill:", err)
	}

	pk.cmd.closeJobObject()
}

// Send a CTRL_C_EVENT signal (like SIGTERM in unix) to a console process via
// kernel32 APIs.
// See https://learn.microsoft.com/en-us/windows/console/console-functions
func taskTerminate(pid int, UseWindowsLegacyProcessStrategy bool) error {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	if err := kernel32.Load(); err != nil {
		return fmt.Errorf("failed to load kernel32: %w", err)
	}

	kernel32Function := func(methodName string) func(string, ...uintptr) error {
		return func(description string, args ...uintptr) error {
			if res1, _, callErr := kernel32.NewProc(methodName).Call(args...); res1 == 0 {
				return fmt.Errorf("failed to %s: %w", description, callErr)
			}
			return nil
		}
	}

	freeConsole := kernel32Function("FreeConsole")
	attachConsole := kernel32Function("AttachConsole")
	setConsoleCtrlHandler := kernel32Function("SetConsoleCtrlHandler")
	generateConsoleCtrlEvent := kernel32Function("GenerateConsoleCtrlEvent")

	if UseWindowsLegacyProcessStrategy {
		if err := freeConsole("detach the runner process from its console"); err != nil {
			return err
		}
		if err := attachConsole("attach to the console of the process being terminated", uintptr(pid)); err != nil {
			return err
		}
		if err := setConsoleCtrlHandler("disable Ctrl-C event handler for runner process", uintptr(unsafe.Pointer(nil)), uintptr(1)); err != nil {
			return err
		}
	}

	// always attempt to restore console and Ctrl-C handler for runner process
	// so collect any errors together instead of returning early
	var errors *multierror.Error

	if UseWindowsLegacyProcessStrategy {
		errors = multierror.Append(errors, generateConsoleCtrlEvent(
			"send Ctrl-C event to process being terminated", uintptr(windows.CTRL_C_EVENT), uintptr(pid)))
		errors = multierror.Append(errors, freeConsole(
			"detach the runner process from the console of the terminated process"))
		errors = multierror.Append(errors, attachConsole(
			"attach the runner process to the console of its parent process", uintptr(math.MaxUint32)))
		errors = multierror.Append(errors, setConsoleCtrlHandler(
			"restore Ctrl-C event handler for runner process", uintptr(unsafe.Pointer(nil)), uintptr(0)))
	} else {
		errors = multierror.Append(errors, generateConsoleCtrlEvent(
			"send Ctrl-Break event to process being terminated", uintptr(windows.CTRL_BREAK_EVENT), uintptr(pid)))
	}

	return errors.ErrorOrNil()
}

func taskKill(pid int) error {
	return exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}

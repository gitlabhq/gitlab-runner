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

	if err := taskTerminate(pk.logger, pk.cmd.Process().Pid, pk.cmd.options.UseWindowsLegacyProcessStrategy); err != nil {
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

// consoleFunc calls one kernel32 console function. The description names the
// action for the error message.
type consoleFunc func(description string, args ...uintptr) error

// consoleAPI groups the kernel32 console functions used to terminate a
// process, so the termination logic can be exercised without a real console.
type consoleAPI struct {
	freeConsole              consoleFunc
	attachConsole            consoleFunc
	setConsoleCtrlHandler    consoleFunc
	generateConsoleCtrlEvent consoleFunc
}

func newKernel32ConsoleAPI() (consoleAPI, error) {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	if err := kernel32.Load(); err != nil {
		return consoleAPI{}, fmt.Errorf("failed to load kernel32: %w", err)
	}

	kernel32Function := func(methodName string) consoleFunc {
		return func(description string, args ...uintptr) error {
			if res1, _, callErr := kernel32.NewProc(methodName).Call(args...); res1 == 0 {
				return fmt.Errorf("failed to %s: %w", description, callErr)
			}
			return nil
		}
	}

	return consoleAPI{
		freeConsole:              kernel32Function("FreeConsole"),
		attachConsole:            kernel32Function("AttachConsole"),
		setConsoleCtrlHandler:    kernel32Function("SetConsoleCtrlHandler"),
		generateConsoleCtrlEvent: kernel32Function("GenerateConsoleCtrlEvent"),
	}, nil
}

// Send a CTRL_C_EVENT signal (like SIGTERM in unix) to a console process via
// kernel32 APIs.
// See https://learn.microsoft.com/en-us/windows/console/console-functions
func taskTerminate(logger Logger, pid int, useWindowsLegacyProcessStrategy bool) error {
	api, err := newKernel32ConsoleAPI()
	if err != nil {
		return err
	}

	return terminateViaConsole(logger, api, pid, useWindowsLegacyProcessStrategy)
}

// terminateViaConsole reports an error only when the control event could not be
// delivered. Restoring the runner's own console state runs after delivery, so a
// failure there says nothing about whether the target process was signaled. The
// caller force-kills on error, so folding the restore failures into the return
// value would kill a process that is already shutting down gracefully.
func terminateViaConsole(logger Logger, api consoleAPI, pid int, useWindowsLegacyProcessStrategy bool) error {
	if !useWindowsLegacyProcessStrategy {
		return api.generateConsoleCtrlEvent(
			"send Ctrl-Break event to process being terminated", uintptr(windows.CTRL_BREAK_EVENT), uintptr(pid))
	}

	if err := api.freeConsole("detach the runner process from its console"); err != nil {
		return err
	}
	if err := api.attachConsole(
		"attach to the console of the process being terminated", uintptr(pid)); err != nil {
		return err
	}
	if err := api.setConsoleCtrlHandler(
		"disable Ctrl-C event handler for runner process", uintptr(unsafe.Pointer(nil)), uintptr(1)); err != nil {
		return err
	}

	deliveryErr := api.generateConsoleCtrlEvent(
		"send Ctrl-C event to process being terminated", uintptr(windows.CTRL_C_EVENT), uintptr(pid))

	// Restore the console and the Ctrl-C handler even when delivery failed.
	// Every restore call runs, and the failures are reported in one warning.
	restoreErr := multierror.Append(
		nil,
		api.freeConsole("detach the runner process from the console of the terminated process"),
		api.attachConsole(
			"attach the runner process to the console of its parent process", uintptr(math.MaxUint32)),
		api.setConsoleCtrlHandler(
			"restore Ctrl-C event handler for runner process", uintptr(unsafe.Pointer(nil)), uintptr(0)),
	).ErrorOrNil()
	if restoreErr != nil {
		logger.Warn("Failed to restore the console of the runner process:", restoreErr)
	}

	return deliveryErr
}

func taskKill(pid int) error {
	return exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}

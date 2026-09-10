package process

import (
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"sync"
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
	// getConsoleWindow reports an error exactly when the runner holds no
	// console, because GetConsoleWindow returns a null handle in that case.
	getConsoleWindow consoleFunc
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
		getConsoleWindow:         kernel32Function("GetConsoleWindow"),
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

// consoleMu serializes the console swaps below. FreeConsole and AttachConsole
// change which console the whole runner process is attached to, so two jobs
// that terminate at the same time would otherwise signal each other's console.
var consoleMu sync.Mutex

// terminateViaConsole reports an error only when the control event could not be
// delivered. Restoring the runner's own console state runs after delivery, so a
// failure there says nothing about whether the target process was signaled. The
// caller force-kills on error, so folding the restore failures into the return
// value would kill a process that is already shutting down gracefully.
func terminateViaConsole(logger Logger, api consoleAPI, pid int, useWindowsLegacyProcessStrategy bool) error {
	if !useWindowsLegacyProcessStrategy {
		runnerHasConsole, err := sendCtrlBreak(api, pid)
		if err == nil {
			return nil
		}

		// A runner that holds a console keeps it. The event failed for another
		// reason, such as the target process group not being attached to that
		// console, and borrowing the console of the target would trade a
		// working console for a restore that cannot be relied on: the restore
		// reattaches to the parent, which may have exited.
		if runnerHasConsole {
			return err
		}

		// GenerateConsoleCtrlEvent needs the caller to hold a console. A runner
		// that runs as a Windows service holds none, so the call above cannot
		// work there. The job process is a console application and gets a
		// console of its own, so borrow that console to reach its process
		// group.
		logger.Warn("Failed to send Ctrl-Break event, retrying through the console of the process:", err)

		return terminateViaTargetConsole(logger, api, pid)
	}

	consoleMu.Lock()
	defer consoleMu.Unlock()

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
	warnRestoreFailures(
		logger,
		api.freeConsole("detach the runner process from the console of the terminated process"),
		api.attachConsole(
			"attach the runner process to the console of its parent process", uintptr(math.MaxUint32)),
		api.setConsoleCtrlHandler(
			"restore Ctrl-C event handler for runner process", uintptr(unsafe.Pointer(nil)), uintptr(0)),
	)

	return deliveryErr
}

// sendCtrlBreak raises the event through whatever console the runner holds, and
// reports whether the runner holds a console of its own. That tells a runner
// with no console apart from one whose target is simply not on its console.
//
// Both answers are taken under consoleMu. A concurrent termination swaps the
// runner onto the console of another job, and while it holds that console
// neither the event nor the console check would be about the runner's own
// console.
func sendCtrlBreak(api consoleAPI, pid int) (bool, error) {
	consoleMu.Lock()
	defer consoleMu.Unlock()

	err := api.generateConsoleCtrlEvent(
		"send Ctrl-Break event to process being terminated", uintptr(windows.CTRL_BREAK_EVENT), uintptr(pid))
	if err == nil {
		// The event could only be raised through a console the runner holds.
		return true, nil
	}

	return api.getConsoleWindow("check whether the runner holds a console") == nil, err
}

// warnRestoreFailures reports every failed restore call in one warning. A
// failure here is never returned: the control event has already been delivered,
// so it says nothing about the process being terminated.
func warnRestoreFailures(logger Logger, errs ...error) {
	if err := multierror.Append(nil, errs...).ErrorOrNil(); err != nil {
		logger.Warn("Failed to restore the console of the runner process:", err)
	}
}

// terminateViaTargetConsole attaches the runner to the console of the process
// being terminated, signals that process group, then puts the runner back.
// The job process was created with CREATE_NEW_PROCESS_GROUP, so a Ctrl-Break
// event reaches only that group and leaves the runner alone.
func terminateViaTargetConsole(logger Logger, api consoleAPI, pid int) error {
	consoleMu.Lock()
	defer consoleMu.Unlock()

	// A runner with no console has nothing to detach from, and that is the
	// case this path exists for, so a failure here is not fatal.
	if err := api.freeConsole("detach the runner process from its console"); err != nil {
		logger.Warn("Failed to detach the runner process from its console:", err)
	}

	if err := api.attachConsole(
		"attach to the console of the process being terminated", uintptr(pid)); err != nil {
		return err
	}

	deliveryErr := api.generateConsoleCtrlEvent(
		"send Ctrl-Break event through the console of the process being terminated",
		uintptr(windows.CTRL_BREAK_EVENT), uintptr(pid))

	warnRestoreFailures(
		logger,
		api.freeConsole("detach the runner process from the console of the terminated process"),
		api.attachConsole(
			"attach the runner process to the console of its parent process", uintptr(math.MaxUint32)),
	)

	return deliveryErr
}

func taskKill(pid int) error {
	return exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}

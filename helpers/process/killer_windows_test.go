//go:build !integration

package process

import (
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/sys/windows"
)

const (
	descDetachOwnConsole    = "detach the runner process from its console"
	descAttachTargetConsole = "attach to the console of the process being terminated"
	descDisableCtrlHandler  = "disable Ctrl-C event handler for runner process"
	descSendCtrlC           = "send Ctrl-C event to process being terminated"
	descDetachTargetConsole = "detach the runner process from the console of the terminated process"
	descAttachParentConsole = "attach the runner process to the console of its parent process"
	descRestoreCtrlHandler  = "restore Ctrl-C event handler for runner process"
	descSendCtrlBreak       = "send Ctrl-Break event to process being terminated"
)

// testPID stands in for the process being terminated.
const testPID = 1234

// nullHandler is the uintptr(unsafe.Pointer(nil)) handler argument that
// SetConsoleCtrlHandler takes to address the default handler.
const nullHandler = 0

// consoleCall is one call through the fake consoleAPI. field names the
// consoleAPI member that was called, so a description cannot be recorded
// against the wrong kernel32 function.
type consoleCall struct {
	field       string
	description string
	args        []uintptr
}

// newFakeConsoleAPI records every call and fails the calls named in errs.
func newFakeConsoleAPI(calls *[]consoleCall, errs map[string]error) consoleAPI {
	record := func(field string) consoleFunc {
		return func(description string, args ...uintptr) error {
			*calls = append(*calls, consoleCall{field: field, description: description, args: args})
			return errs[description]
		}
	}

	return consoleAPI{
		freeConsole:              record("freeConsole"),
		attachConsole:            record("attachConsole"),
		setConsoleCtrlHandler:    record("setConsoleCtrlHandler"),
		generateConsoleCtrlEvent: record("generateConsoleCtrlEvent"),
	}
}

// legacyCalls is the full sequence the legacy strategy makes: take over the
// console of the target process, signal it, then restore the runner's own
// console state.
func legacyCalls() []consoleCall {
	return []consoleCall{
		{field: "freeConsole", description: descDetachOwnConsole},
		{field: "attachConsole", description: descAttachTargetConsole, args: []uintptr{testPID}},
		{field: "setConsoleCtrlHandler", description: descDisableCtrlHandler, args: []uintptr{nullHandler, 1}},
		{
			field:       "generateConsoleCtrlEvent",
			description: descSendCtrlC,
			args:        []uintptr{windows.CTRL_C_EVENT, testPID},
		},
		{field: "freeConsole", description: descDetachTargetConsole},
		{field: "attachConsole", description: descAttachParentConsole, args: []uintptr{math.MaxUint32}},
		{field: "setConsoleCtrlHandler", description: descRestoreCtrlHandler, args: []uintptr{nullHandler, 0}},
	}
}

func TestTerminateViaConsole(t *testing.T) {
	errDelivery := errors.New("delivery failed")
	errRestore := errors.New("restore failed")
	errAttach := errors.New("attach failed")

	ctrlBreakCalls := []consoleCall{{
		field:       "generateConsoleCtrlEvent",
		description: descSendCtrlBreak,
		args:        []uintptr{windows.CTRL_BREAK_EVENT, testPID},
	}}

	tests := map[string]struct {
		legacyStrategy bool
		errs           map[string]error
		expectedErr    error
		expectedCalls  []consoleCall
		expectWarning  bool
	}{
		"sends Ctrl-Break to the process group": {
			expectedCalls: ctrlBreakCalls,
		},
		"reports a failed Ctrl-Break": {
			errs:          map[string]error{descSendCtrlBreak: errDelivery},
			expectedErr:   errDelivery,
			expectedCalls: ctrlBreakCalls,
		},
		"legacy strategy sends Ctrl-C and restores the console": {
			legacyStrategy: true,
			expectedCalls:  legacyCalls(),
		},
		// A runner that runs as a Windows service has no console of its own, so
		// it cannot reattach to one after it signals the process. The process
		// was signaled, so this is not a termination failure.
		"legacy strategy accepts a failed console restore": {
			legacyStrategy: true,
			errs:           map[string]error{descAttachParentConsole: errRestore},
			expectWarning:  true,
			expectedCalls:  legacyCalls(),
		},
		"legacy strategy reports a failed Ctrl-C": {
			legacyStrategy: true,
			errs:           map[string]error{descSendCtrlC: errDelivery},
			expectedErr:    errDelivery,
			expectedCalls:  legacyCalls(),
		},
		"legacy strategy stops when it cannot attach to the target console": {
			legacyStrategy: true,
			errs:           map[string]error{descAttachTargetConsole: errAttach},
			expectedErr:    errAttach,
			expectedCalls:  legacyCalls()[:2],
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			logger := NewMockLogger(t)
			if tt.expectWarning {
				logger.On("Warn", mock.Anything, mock.Anything).Once()
			}

			var calls []consoleCall
			api := newFakeConsoleAPI(&calls, tt.errs)

			err := terminateViaConsole(logger, api, testPID, tt.legacyStrategy)

			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedCalls, calls)
		})
	}
}

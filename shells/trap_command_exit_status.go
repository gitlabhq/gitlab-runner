package shells

import (
	"encoding/json"
)

type TrapCommandExitStatus struct {
	// CommandExitCode is the exit code of the last command. **REQUIRED**.
	CommandExitCode *int `json:"command_exit_code"`
	// Script is the script which was executed as an entrypoint for the current execution step.
	// The scripts are currently named after the stage they are executed in.
	// This property is **NOT REQUIRED** and may be nil in some cases.
	// For example, when an error is reported by the log processor itself and not the script it was monitoring.
	Script *string `json:"script"`
}

func (c *TrapCommandExitStatus) hasRequiredFields() bool {
	return c != nil && c.CommandExitCode != nil
}

// TryUnmarshal tries to unmarshal a json string into its pointer receiver.
// It wil return true only if the unmarshalled struct has all of its required fields be non-nil.
// It's safe to use the struct only if this method returns true.
func (c *TrapCommandExitStatus) TryUnmarshal(line string) bool {
	err := json.Unmarshal([]byte(line), c)
	if err != nil {
		return false
	}

	if !c.hasRequiredFields() {
		return false
	}

	return true
}

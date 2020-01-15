package shells

import (
	"encoding/json"
)

type TrapCommandExitStatus struct {
	CommandExitCode *int    `json:"command_exit_code"`
	Script          *string `json:"script"`
}

func (c *TrapCommandExitStatus) isComplete() bool {
	return c != nil && c.CommandExitCode != nil && c.Script != nil
}

// TryUnmarshal tries to unmarshal a json string into its pointer receiver.
// It wil return true only if the unmarshalled struct has all of its fields be non-nil.
// It's safe to use the struct only if this method returns true.
func (c *TrapCommandExitStatus) TryUnmarshal(line string) bool {
	err := json.Unmarshal([]byte(line), c)
	if err != nil {
		return false
	}

	if !c.isComplete() {
		return false
	}

	return true
}

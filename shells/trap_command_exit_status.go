package shells

import (
	"encoding/json"
	"fmt"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// stageCommandExitStatusImpl is a private struct used to unmarshall the log line read.
// All of its fields are optional, so it can check to make sure against the required and optional ones.
// The fields are then applied to StageCommandStatus which is the exposed, ready-to-use struct.
type stageCommandExitStatusImpl struct {
	// CommandExitCode is the exit code of the last command.
	CommandExitCode *int `json:"command_exit_code"`
	// Script is the script which was executed as an entrypoint for the current execution step.
	// The scripts are currently named after the stage they are executed in.
	// This property is **NOT REQUIRED** and may be nil in some cases.
	// For example, when an error is reported by the log processor itself and not the script it was monitoring.
	Script *string `json:"script"`
}

// tryUnmarshal tries to unmarshal a json string into its pointer receiver.
// It's safe to use the struct only if this method returns no error.
func (cmd *stageCommandExitStatusImpl) tryUnmarshal(line string) error {
	return json.Unmarshal([]byte(line), cmd)
}

func (cmd stageCommandExitStatusImpl) isEmpty() bool {
	return cmd.CommandExitCode == nil && cmd.Script == nil
}

func (cmd stageCommandExitStatusImpl) applyTo(to *StageCommandStatus) {
	to.CommandExitCode = cmd.CommandExitCode
	to.Script = cmd.Script
}

type StageCommandStatus struct {
	CommandExitCode *int
	Script          *string
}

// TryUnmarshal tries to unmarshal a json string into its pointer receiver.
// It wil return true only if the unmarshalled struct has all of its required fields be non-nil.
// It's safe to use the struct only if this method returns true.
func (c *StageCommandStatus) TryUnmarshal(line string) bool {
	var status stageCommandExitStatusImpl
	err := status.tryUnmarshal(line)
	if err != nil {
		return false
	}

	if status.isEmpty() {
		return false
	}

	status.applyTo(c)

	return true
}

func (c StageCommandStatus) String() string {
	// TODO: needs to be better
	var str string
	if c.CommandExitCode != nil {
		str = fmt.Sprintf("CommandExitCode: %v", c.CommandExitCode)
	}

	if c.Script != nil {
		str = fmt.Sprintf("%s, Script: %v", str, *c.Script)
	}

	return str
}

func (c StageCommandStatus) IsExited() bool {
	return c.CommandExitCode != nil
}

func (c StageCommandStatus) BuildStage() common.BuildStage {
	if c.Script == nil {
		return ""
	}

	// TODO: windows?
	split := strings.Split(*c.Script, "/")
	stage := split[len(split)-1]

	return common.BuildStage(stage)
}

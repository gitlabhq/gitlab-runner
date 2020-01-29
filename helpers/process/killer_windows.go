package process

import (
	"os/exec"
	"strconv"
)

type windowsKiller struct {
	logger Logger
	cmd    Commander
}

func newKiller(logger Logger, cmd Commander) killer {
	return &windowsKiller{
		logger: logger,
		cmd:    cmd,
	}
}

// Terminate on windows sends a taskkill to the command and it's child processes
// forcefully `/F` since most time the process can't be killed and ends up
// erroring out.
func (pk *windowsKiller) Terminate() {
	if pk.cmd.Process() == nil {
		return
	}

	err := taskKill(pk.cmd.Process().Pid)
	if err != nil {
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
}

func taskKill(pid int) error {
	return exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}

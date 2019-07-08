package process

import (
	"os"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type windowsKiller struct {
	logger  common.BuildLogger
	process *os.Process
}

func NewKiller(logger common.BuildLogger, process *os.Process) Killer {
	return &windowsKiller{
		logger:  logger,
		process: process,
	}
}

func (pk *windowsKiller) Terminate() {
	err := pk.process.Kill()
	if err != nil {
		pk.logger.Errorln("Failed to terminate:", err)

		pk.ForceKill()
	}
}

func (pk *windowsKiller) ForceKill() {
	err := pk.process.Kill()
	if err != nil {
		pk.logger.Errorln("Failed to force-kill:", err)
	}
}

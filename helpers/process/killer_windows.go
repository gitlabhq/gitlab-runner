package process

import (
	"os"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type windowsKiller struct {
	logger  common.BuildLogger
	process *os.Process
}

func newKiller(logger common.BuildLogger, process *os.Process) killer {
	return &windowsKiller{
		logger:  logger,
		process: process,
	}
}

func (pk *windowsKiller) Terminate() {
	if pk.process == nil {
		return
	}

	err := pk.process.Kill()
	if err != nil {
		pk.logger.Errorln("Failed to terminate:", err)

		// try to kill right-after
		pk.ForceKill()
	}
}

func (pk *windowsKiller) ForceKill() {
	if pk.process == nil {
		return
	}

	err := pk.process.Kill()
	if err != nil {
		pk.logger.Errorln("Failed to force-kill:", err)
	}
}

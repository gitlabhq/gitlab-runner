// +build darwin dragonfly freebsd linux netbsd openbsd

package process

import (
	"os"
	"syscall"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type unixKiller struct {
	logger  common.BuildLogger
	process *os.Process
}

func newKiller(logger common.BuildLogger, process *os.Process) killer {
	return &unixKiller{
		logger:  logger,
		process: process,
	}
}

func (pk *unixKiller) Terminate() {
	if pk.process == nil {
		return
	}

	err := pk.process.Signal(syscall.SIGTERM)
	if err != nil {
		pk.logger.Errorln("Failed to send SIGTERM signal:", err)

		// try to kill right-after
		pk.ForceKill()
	}
}

func (pk *unixKiller) ForceKill() {
	if pk.process == nil {
		return
	}

	err := pk.process.Kill()
	if err != nil {
		pk.logger.Errorln("Failed to force-kill:", err)
	}
}

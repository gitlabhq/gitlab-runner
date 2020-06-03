package process

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

func (pk *windowsKiller) Terminate() {
	if pk.cmd.Process() == nil {
		return
	}

	err := pk.cmd.Process().Kill()
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

	err := pk.cmd.Process().Kill()
	if err != nil {
		pk.logger.Warn("Failed to force-kill:", err)
	}
}

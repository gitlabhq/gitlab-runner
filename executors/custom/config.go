package custom

import (
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

type config struct {
	*common.CustomConfig
}

func (c *config) GetConfigExecTimeout() time.Duration {
	return getDuration(c.ConfigExecTimeout, defaultConfigExecTimeout)
}

func (c *config) GetPrepareExecTimeout() time.Duration {
	return getDuration(c.PrepareExecTimeout, defaultPrepareExecTimeout)
}

func (c *config) GetCleanupScriptTimeout() time.Duration {
	return getDuration(c.CleanupExecTimeout, defaultCleanupExecTimeout)
}

func (c *config) GetGracefulKillTimeout() time.Duration {
	return getDuration(c.GracefulKillTimeout, process.GracefulTimeout)
}

func (c *config) GetForceKillTimeout() time.Duration {
	return getDuration(c.ForceKillTimeout, process.KillTimeout)
}

func getDuration(source *int, defaultValue time.Duration) time.Duration {
	if source == nil {
		return defaultValue
	}

	timeout := *source
	if timeout <= 0 {
		return defaultValue
	}

	return time.Duration(timeout) * time.Second
}

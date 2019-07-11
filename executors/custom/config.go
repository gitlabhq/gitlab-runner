package custom

import (
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
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
	return getDuration(c.GracefulKillTimeout, defaultGracefulKillTimeout)
}

func (c *config) GetForceKillTimeout() time.Duration {
	return getDuration(c.ForceKillTimeout, defaultForceKillTimeout)
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

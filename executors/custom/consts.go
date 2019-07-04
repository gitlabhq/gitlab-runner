package custom

import "time"

const defaultConfigExecTimeout = time.Hour
const defaultPrepareExecTimeout = time.Hour
const defaultCleanupExecTimeout = time.Hour

const defaultGracefulKillTimeout = 10 * time.Minute
const defaultForceKillTimeout = 10 * time.Second

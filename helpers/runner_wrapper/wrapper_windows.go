package runner_wrapper

import (
	"syscall"
)

const (
	gracefulShutdownSignal = syscall.SIGINT
)

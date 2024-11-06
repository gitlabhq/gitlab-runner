//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris

package runner_wrapper

import (
	"syscall"
)

const (
	gracefulShutdownSignal = syscall.SIGQUIT
)

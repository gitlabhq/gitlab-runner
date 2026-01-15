//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris || zos

package runner_wrapper

import (
	"syscall"
)

const (
	gracefulShutdownSignal = syscall.SIGQUIT
)

func (w *Wrapper) forcefulShutdown(p process) error {
	return p.Signal(syscall.SIGTERM)
}

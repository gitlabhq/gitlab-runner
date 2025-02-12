package runner_wrapper

import (
	"fmt"
	"syscall"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/runner_wrapper/api"
)

const (
	gracefulShutdownSignal = syscall.SIGINT
)

func (w *Wrapper) forcefulShutdown(p process) error {
	err := p.Signal(syscall.SIGINT)
	if err != nil {
		return fmt.Errorf("sending first SIGINT: %w", err)
	}

	// Windows doesn't have SIGQUIT, so graceful shutdown is initiated by sending
	// a first SIGINT signal.
	// Sending a second one switches to forceful shutdown.
	// However, when a third is sent, the Runner terminates instantly,
	// without cleaning up resources.
	// Therefore, we need to check whether the process is already in shutdown
	// (which could be done by prior calling of InitiateGracefulShutdown()) and
	// then decide whether we should send one or two SIGINTs to initiate forceful
	// shutdown.
	// If graceful was not started - we need to send two, the first will initiate graceful
	// shutdown, and the second will switch it to the forceful shutdown.
	// If graceful was already started, we just need to send SIGINT once, to switch
	// it to forceful shutdown.
	// Take a look at commands/multi.go and the comments there to fully understand
	// the shutdown strategies and difference between Windows and Unix-like OSes.
	gracefulShutdownAlreadyStarted := w.Status() == api.StatusInShutdown
	if gracefulShutdownAlreadyStarted {
		return nil
	}

	time.Sleep(10 * time.Millisecond)

	err = p.Signal(syscall.SIGINT)
	if err != nil {
		return fmt.Errorf("sending second SIGINT: %w", err)
	}

	return nil
}

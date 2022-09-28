package process

import (
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// ErrProcessNotStarted is returned when we try to manipulated/interact with a
// process that hasn't started yet (still nil).
var ErrProcessNotStarted = errors.New("process not started yet")

// GracefulTimeout is the time a Killer should wait in general to the graceful
// termination to timeout.
const GracefulTimeout = 10 * time.Minute

// KillTimeout is the time a killer should wait in general for the kill command
// to finish.
const KillTimeout = 10 * time.Second

//go:generate mockery --name=killer --inpackage
type killer interface {
	Terminate()
	ForceKill()
}

var newProcessKiller = newKiller

//go:generate mockery --name=KillWaiter --inpackage
type KillWaiter interface {
	KillAndWait(command Commander, waitCh chan error) error
}

type KillProcessError struct {
	pid int
}

func (k *KillProcessError) Error() string {
	return fmt.Sprintf("failed to kill process PID=%d, likely process is dormant", k.pid)
}

func (k *KillProcessError) Is(err error) bool {
	_, ok := err.(*KillProcessError)

	return ok
}

type osKillWait struct {
	logger Logger

	gracefulKillTimeout time.Duration
	forceKillTimeout    time.Duration
}

func NewOSKillWait(logger Logger, gracefulKillTimeout, forceKillTimeout time.Duration) KillWaiter {
	return &osKillWait{
		logger:              logger,
		gracefulKillTimeout: gracefulKillTimeout,
		forceKillTimeout:    forceKillTimeout,
	}
}

// KillAndWait will take the specified process and terminate the process and
// wait util the waitCh returns or the graceful kill timer runs out after which
// a force kill on the process would be triggered.
func (kw *osKillWait) KillAndWait(command Commander, waitCh chan error) error {
	process := command.Process()
	if process == nil {
		return ErrProcessNotStarted
	}

	log := kw.logger.WithFields(logrus.Fields{
		"PID": process.Pid,
	})

	processKiller := newProcessKiller(log, command)
	processKiller.Terminate()

	select {
	case err := <-waitCh:
		return err
	case <-time.After(kw.gracefulKillTimeout):
		processKiller.ForceKill()

		select {
		case err := <-waitCh:
			return err
		case <-time.After(kw.forceKillTimeout):
			return &KillProcessError{pid: process.Pid}
		}
	}
}

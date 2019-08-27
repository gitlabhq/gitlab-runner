package process

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// ErrProcessNotStarted is returned when we try to manipulated/interact with a
// process that hasn't started yet (still nil).
var ErrProcessNotStarted = errors.New("process not started yet")

type killer interface {
	Terminate()
	ForceKill()
}

var newProcessKiller = newKiller

type KillWaiter interface {
	KillAndWait(process *os.Process, waitCh chan error) error
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
	logger common.BuildLogger

	gracefulKillTimeout time.Duration
	forceKillTimeout    time.Duration
}

func NewOSKillWait(logger common.BuildLogger, gracefulKillTimeout time.Duration, forceKillTimeout time.Duration) KillWaiter {
	return &osKillWait{
		logger:              logger,
		gracefulKillTimeout: gracefulKillTimeout,
		forceKillTimeout:    forceKillTimeout,
	}
}

// KillAndWait will take the specified process and terminate the process and
// wait util the waitCh returns or the graceful kill timer runs out after which
// a force kill on the process would be triggered.
func (kw *osKillWait) KillAndWait(process *os.Process, waitCh chan error) error {
	if process == nil {
		return ErrProcessNotStarted
	}

	log := kw.logger.WithFields(logrus.Fields{
		"PID": process.Pid,
	})

	processKiller := newProcessKiller(log, process)
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

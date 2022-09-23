//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris

package log

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/sirupsen/logrus"
)

func watchForGoroutinesDump(logger *logrus.Logger, stopCh chan bool) (chan bool, chan bool) {
	dumpedCh := make(chan bool)
	finishedCh := make(chan bool)

	dumpStacksCh := make(chan os.Signal, 1)
	// On USR1 dump stacks of all go routines
	signal.Notify(dumpStacksCh, syscall.SIGUSR1)

	go func() {
		for {
			select {
			case <-dumpStacksCh:
				buf := make([]byte, 1<<20)
				len := runtime.Stack(buf, true)
				logger.Printf("=== received SIGUSR1 ===\n*** goroutine dump...\n%s\n*** end\n", buf[0:len])

				nonBlockingSend(dumpedCh, true)
			case <-stopCh:
				close(finishedCh)
				return
			}
		}
	}()

	return dumpedCh, finishedCh
}

func nonBlockingSend(ch chan bool, value bool) {
	select {
	case ch <- value:
	default:
	}
}

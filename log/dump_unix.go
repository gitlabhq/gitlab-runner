// +build darwin dragonfly freebsd linux netbsd openbsd

package log

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/sirupsen/logrus"
)

func watchForGoroutinesDump(stopCh chan bool) {
	dumpStacks := make(chan os.Signal, 1)

	// On USR1 dump stacks of all go routines
	signal.Notify(dumpStacks, syscall.SIGUSR1)

	for {
		select {
		case <-dumpStacks:
			buf := make([]byte, 1<<20)
			len := runtime.Stack(buf, true)
			logrus.Printf("=== received SIGUSR1 ===\n*** goroutine dump...\n%s\n*** end\n", buf[0:len])
		case <-stopCh:
			return
		}
	}
}

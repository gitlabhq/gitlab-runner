// This is a binary used to run automated tests on. It's compiled when the tests
// run and executed/stopped by the test. For more information check
// killer_test.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const SkipTerminateOption = "skip-terminate-signals"

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s duration [%s]\n", os.Args[0], SkipTerminateOption)
		os.Exit(1)
	}

	duration, err := time.ParseDuration(os.Args[1])
	if err != nil {
		panic(fmt.Sprintf("Couldn't parse duration argument: %v", err))
	}

	skipTermination := len(os.Args) > 2 && os.Args[2] == SkipTerminateOption

	ctx, cancel := context.WithCancel(context.Background())

	signalCh := make(chan os.Signal)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		fmt.Println("Waiting for signals (SIGTERM, SIGINT)...")
		sig := <-signalCh

		fmt.Printf("Received signal: %v\n", sig)

		if skipTermination {
			fmt.Printf("but ignoring it due to %q option used\n", SkipTerminateOption)
			return
		}

		fmt.Println("forcing termination...")
		cancel()
	}()

	fmt.Printf("Sleeping for %s (PID=%d)\n", duration, os.Getpid())

	select {
	case <-time.After(duration):
		fmt.Println("Sleep duration achieved")
	case <-ctx.Done():
		fmt.Println("Forced to quit by signal; terminating")
		os.Exit(1)
	}
}

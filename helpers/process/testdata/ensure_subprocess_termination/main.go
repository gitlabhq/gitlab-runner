package main

import (
	"fmt"
	"os"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "child" {
		time.Sleep(60 * time.Second)
		os.Exit(1)
	}

	if err := process.EnsureSubprocessTerminationOnExit(); err != nil {
		fmt.Fprintf(os.Stderr, "ensuring subprocess termination on exit: %v\n", err)
		os.Exit(1)
	}

	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getting executable path: %v\n", err)
		os.Exit(1)
	}

	cmd := process.NewOSCmd(executable, []string{"child"}, process.CommandOptions{})
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "starting child process: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Child PID:%d\n", cmd.Process().Pid)
	fmt.Println("READY")

	if err := cmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "child process error: %v\n", err)
		os.Exit(1)
	}
}

//go:build windows

// This is a binary used to run automated tests on. It reproduces the state of a
// runner that was started by the Windows Service Control Manager: a process
// that holds no console. For more information check
// killer_no_console_integration_test.go
//
// The binary runs in two stages. The outer stage starts the inner stage with
// DETACHED_PROCESS, so the inner stage holds no console and its parent holds no
// console either. AttachConsole(ATTACH_PARENT_PROCESS) then fails for the inner
// stage, which is what a service sees. The inner stage terminates a child
// process and writes the outcome to a file for the test to read.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

const detachedProcess = 0x00000008

type result struct {
	WaitError  string   `json:"wait_error"`
	Warnings   []string `json:"warnings"`
	HadConsole bool     `json:"had_console"`
}

func main() {
	stage := flag.String("stage", "", "outer or inner")
	sleepBinary := flag.String("sleep-binary", "", "path to the process to terminate")
	resultPath := flag.String("result", "", "path to write the outcome to")
	legacy := flag.Bool("legacy", false, "use the legacy Windows process strategy")
	flag.Parse()

	var err error
	switch *stage {
	case "outer":
		err = runOuter(*sleepBinary, *resultPath, *legacy)
	case "inner":
		err = runInner(*sleepBinary, *resultPath, *legacy)
	default:
		err = fmt.Errorf("unknown stage %q", *stage)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runOuter starts the inner stage detached, then waits for it. The outer stage
// stays alive so that the inner stage has a parent that holds no console,
// rather than no parent at all.
func runOuter(sleepBinary, resultPath string, legacy bool) error {
	cmd := exec.Command(
		os.Args[0],
		"-stage", "inner",
		"-sleep-binary", sleepBinary,
		"-result", resultPath,
		fmt.Sprintf("-legacy=%t", legacy),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess}

	// A detached process has nowhere to write to, so keep its output for the
	// test to report when an assertion fails.
	log, err := os.Create(resultPath + ".log")
	if err != nil {
		return fmt.Errorf("creating the log of the inner stage: %w", err)
	}
	defer log.Close()

	cmd.Stdout = log
	cmd.Stderr = log

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting the inner stage: %w", err)
	}

	return cmd.Wait()
}

func runInner(sleepBinary, resultPath string, legacy bool) error {
	// Read this before terminating anything: both strategies attach and detach
	// consoles, so the state afterwards says nothing about the state we started
	// in.
	hadConsole := hasConsole()

	logger := &collectingLogger{}

	command := process.NewOSCmd(sleepBinary, []string{"30s"}, process.CommandOptions{
		UseWindowsLegacyProcessStrategy: legacy,
		Logger:                          logger,
	})
	if err := command.Start(); err != nil {
		return fmt.Errorf("starting the process to terminate: %w", err)
	}

	waitCh := make(chan error)
	go func() {
		waitCh <- command.Wait()
	}()

	// Give the process time to install its signal handler.
	time.Sleep(time.Second)

	waitErr := process.NewOSKillWait(logger, 10*time.Second, 5*time.Second).KillAndWait(command, waitCh)

	outcome := result{Warnings: logger.collected(), HadConsole: hadConsole}
	if waitErr != nil {
		outcome.WaitError = waitErr.Error()
	}

	encoded, err := json.Marshal(outcome)
	if err != nil {
		return fmt.Errorf("encoding the outcome: %w", err)
	}

	// The test polls for this file, so write it whole and then move it into
	// place. A partial read would fail to parse.
	partial := resultPath + ".partial"
	if err := os.WriteFile(partial, encoded, 0o600); err != nil {
		return fmt.Errorf("writing the outcome: %w", err)
	}

	return os.Rename(partial, resultPath)
}

// hasConsole reports whether this process holds a console. The test needs it to
// hold none, otherwise it proves nothing.
func hasConsole() bool {
	handle, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()

	return handle != 0
}

type collectingLogger struct {
	mu       sync.Mutex
	warnings []string
}

func (l *collectingLogger) WithFields(fields logrus.Fields) process.Logger {
	return l
}

func (l *collectingLogger) Warn(args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.warnings = append(l.warnings, fmt.Sprint(args...))
}

func (l *collectingLogger) collected() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]string(nil), l.warnings...)
}

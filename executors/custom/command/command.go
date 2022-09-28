package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/custom/api"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

const (
	BuildFailureExitCode  = 1
	SystemFailureExitCode = 2
)

//go:generate mockery --name=Command --inpackage
type Command interface {
	Run() error
}

var newProcessKillWaiter = process.NewOSKillWait
var newCommander = process.NewOSCmd

type Options struct {
	JobResponseFile string
}

type command struct {
	context context.Context
	cmd     process.Commander

	waitCh chan error

	logger process.Logger

	gracefulKillTimeout time.Duration
	forceKillTimeout    time.Duration
}

func New(
	ctx context.Context,
	executable string,
	args []string,
	cmdOpts process.CommandOptions,
	options Options,
) Command {
	defaultVariables := map[string]string{
		"TMPDIR":                          cmdOpts.Dir,
		api.BuildFailureExitCodeVariable:  strconv.Itoa(BuildFailureExitCode),
		api.SystemFailureExitCodeVariable: strconv.Itoa(SystemFailureExitCode),
		api.JobResponseFileVariable:       options.JobResponseFile,
	}

	env := os.Environ()
	for key, value := range defaultVariables {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	cmdOpts.Env = append(env, cmdOpts.Env...)

	return &command{
		context:             ctx,
		cmd:                 newCommander(executable, args, cmdOpts),
		waitCh:              make(chan error),
		logger:              cmdOpts.Logger,
		gracefulKillTimeout: cmdOpts.GracefulKillTimeout,
		forceKillTimeout:    cmdOpts.ForceKillTimeout,
	}
}

func (c *command) Run() error {
	err := c.cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	go c.waitForCommand()

	select {
	case err = <-c.waitCh:
		return err

	case <-c.context.Done():
		return newProcessKillWaiter(c.logger, c.gracefulKillTimeout, c.forceKillTimeout).
			KillAndWait(c.cmd, c.waitCh)
	}
}

var getExitCode = func(err *exec.ExitError) int {
	return err.ExitCode()
}

func (c *command) waitForCommand() {
	err := c.cmd.Wait()

	eerr, ok := err.(*exec.ExitError)
	if ok {
		exitCode := getExitCode(eerr)
		switch {
		case exitCode == BuildFailureExitCode:
			err = &common.BuildError{Inner: eerr, ExitCode: exitCode}
		case exitCode != SystemFailureExitCode:
			err = &ErrUnknownFailure{Inner: eerr, ExitCode: exitCode}
		}
	}

	c.waitCh <- err
}

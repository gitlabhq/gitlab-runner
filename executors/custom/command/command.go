package command

import (
	"bufio"
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
	JobResponseFile   string
	BuildExitCodeFile string
}

type command struct {
	context context.Context
	cmd     process.Commander

	waitCh chan error

	logger process.Logger

	gracefulKillTimeout time.Duration
	forceKillTimeout    time.Duration

	buildCodeFile string
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
		api.BuildCodeFileVariable:         options.BuildExitCodeFile,
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
		buildCodeFile:       options.BuildExitCodeFile,
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
			err = c.parseBuildFailure(eerr)
		case exitCode != SystemFailureExitCode:
			err = &ErrUnknownFailure{Inner: eerr, ExitCode: exitCode}
		}
	}

	c.waitCh <- err
}

func (c *command) parseBuildFailure(eerr *exec.ExitError) error {
	file, err := os.Open(c.buildCodeFile)
	if err != nil {
		// If the driver has not generated a file at the prescribed location
		// we revert to the default BuildError and exitCode.
		return &common.BuildError{Inner: eerr, ExitCode: BuildFailureExitCode}
	}
	defer file.Close()

	var codeStr string
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		codeStr = scanner.Text()
		break
	}

	bErrCode, err := strconv.Atoi(codeStr)
	if err != nil {
		return &ErrUnknownFailure{Inner: eerr, ExitCode: SystemFailureExitCode}
	}

	// We want to modify the exit code found in the error message to reflect the
	// true error as defined in the file. This aims to prevent confusion users
	// would like experience when presented with the exit status in the job log.
	return &common.BuildError{Inner: fmt.Errorf("exit status %s", codeStr), ExitCode: bErrCode}
}

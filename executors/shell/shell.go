package shell

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kardianos/osext"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

var newProcessKillWaiter = process.NewOSKillWait
var newCommander = process.NewOSCmd

type executor struct {
	executors.AbstractExecutor
}

func (s *executor) Prepare(options common.ExecutorPrepareOptions) error {
	if options.User != "" {
		s.Shell().User = options.User
	}

	// expand environment variables to have current directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	mapping := func(key string) string {
		switch key {
		case "PWD":
			return wd
		default:
			return ""
		}
	}

	s.DefaultBuildsDir = os.Expand(s.DefaultBuildsDir, mapping)
	s.DefaultCacheDir = os.Expand(s.DefaultCacheDir, mapping)

	// Pass control to executor
	err = s.AbstractExecutor.Prepare(options)
	if err != nil {
		return err
	}

	s.Println("Using Shell executor...")
	return nil
}

// TODO: Remove in 14.0 https://gitlab.com/gitlab-org/gitlab-runner/issues/6413
func (s *executor) killAndWait(cmd *exec.Cmd, waitCh chan error) error {
	for {
		s.Debugln("Aborting command...")
		helpers.KillProcessGroup(cmd)
		select {
		case <-time.After(time.Second):
		case err := <-waitCh:
			return err
		}
	}
}

func (s *executor) Run(cmd common.ExecutorCommand) error {
	if s.Build.IsFeatureFlagOn(featureflags.ShellExecutorUseLegacyProcessKill) {
		return s.runLegacy(cmd)
	}

	return s.run(cmd)
}

// TODO: Remove in 14.0 https://gitlab.com/gitlab-org/gitlab-runner/issues/6413
func (s *executor) runLegacy(cmd common.ExecutorCommand) error {
	s.BuildLogger.Debugln("Using legacy command execution")
	// Create execution command
	c := exec.Command(s.BuildShell.Command, s.BuildShell.Arguments...)
	if c == nil {
		return errors.New("failed to generate execution command")
	}

	helpers.SetProcessGroup(c)
	defer helpers.KillProcessGroup(c)

	// Fill process environment variables
	c.Env = append(os.Environ(), s.BuildShell.Environment...)
	c.Stdout = s.Trace
	c.Stderr = s.Trace

	stdin, args, cleanup, err := s.shellScriptArgs(cmd, c.Args)
	if err != nil {
		return err
	}
	defer cleanup()

	c.Stdin = stdin
	c.Args = args

	// Start a process
	err = c.Start()
	if err != nil {
		return fmt.Errorf("starting process: %w", err)
	}

	// Wait for process to finish
	waitCh := make(chan error)
	go func() {
		err := c.Wait()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			err = &common.BuildError{Inner: err}
		}
		waitCh <- err
	}()

	// Support process abort
	select {
	case err = <-waitCh:
		return err
	case <-cmd.Context.Done():
		return s.killAndWait(c, waitCh)
	}
}

func (s *executor) shellScriptArgs(cmd common.ExecutorCommand, args []string) (io.Reader, []string, func(), error) {
	if !s.BuildShell.PassFile {
		return strings.NewReader(cmd.Script), args, func() {}, nil
	}

	scriptDir, err := ioutil.TempDir("", "build_script")
	if err != nil {
		return nil, nil, func() {}, fmt.Errorf("creating tmp build script dir: %w", err)
	}

	cleanup := func() {
		err := os.RemoveAll(scriptDir)
		if err != nil {
			s.BuildLogger.Warningln("Failed to remove build script directory", scriptDir, err)
		}
	}

	scriptFile := filepath.Join(scriptDir, "script."+s.BuildShell.Extension)
	err = ioutil.WriteFile(scriptFile, []byte(cmd.Script), 0700)
	if err != nil {
		return nil, nil, cleanup, fmt.Errorf("writing script file: %w", err)
	}

	return nil, append(args, scriptFile), cleanup, nil
}

func (s *executor) run(cmd common.ExecutorCommand) error {
	s.BuildLogger.Debugln("Using new shell command execution")
	cmdOpts := process.CommandOptions{
		Env:    append(os.Environ(), s.BuildShell.Environment...),
		Stdout: s.Trace,
		Stderr: s.Trace,
	}

	args := s.BuildShell.Arguments
	stdin, args, cleanup, err := s.shellScriptArgs(cmd, args)
	if err != nil {
		return err
	}
	defer cleanup()

	cmdOpts.Stdin = stdin

	// Create execution command
	c := newCommander(s.BuildShell.Command, args, cmdOpts)

	// Start a process
	err = c.Start()
	if err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Wait for process to finish
	waitCh := make(chan error)
	go func() {
		waitErr := c.Wait()
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			waitErr = &common.BuildError{Inner: waitErr}
		}
		waitCh <- waitErr
	}()

	// Support process abort
	select {
	case err = <-waitCh:
		return err
	case <-cmd.Context.Done():
		logger := common.NewProcessLoggerAdapter(s.BuildLogger)
		return newProcessKillWaiter(logger, process.GracefulTimeout, process.KillTimeout).
			KillAndWait(c, waitCh)
	}
}

func init() {
	// Look for self
	runnerCommand, err := osext.Executable()
	if err != nil {
		logrus.Warningln(err)
	}

	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: false,
		DefaultBuildsDir:              "$PWD/builds",
		DefaultCacheDir:               "$PWD/cache",
		SharedBuildsDir:               true,
		Shell: common.ShellScriptInfo{
			Shell:         common.GetDefaultShell(),
			Type:          common.LoginShell,
			RunnerCommand: runnerCommand,
		},
		ShowHostname: false,
	}

	creator := func() common.Executor {
		return &executor{
			AbstractExecutor: executors.AbstractExecutor{
				ExecutorOptions: options,
			},
		}
	}

	featuresUpdater := func(features *common.FeaturesInfo) {
		features.Variables = true
		features.Shared = true

		if runtime.GOOS != "windows" {
			features.Session = true
			features.Terminal = true
		}
	}

	common.RegisterExecutorProvider("shell", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}

package shell

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

var newProcessKillWaiter = process.NewOSKillWait
var newCommander = process.NewOSCmd

type executor struct {
	executors.AbstractExecutor
}

func (s *executor) Name() string {
	return "shell"
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

	s.Println("Using Shell (" + s.Shell().Shell + ") executor...")
	return nil
}

func (s *executor) Run(cmd common.ExecutorCommand) error {
	s.BuildLogger.Debugln("Using new shell command execution")
	cmdOpts := process.CommandOptions{
		Env:                             os.Environ(),
		Stdout:                          s.Trace,
		Stderr:                          s.Trace,
		UseWindowsLegacyProcessStrategy: s.Build.IsFeatureFlagOn(featureflags.UseWindowsLegacyProcessStrategy),
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
	waitCh := make(chan error, 1)
	go func() {
		waitErr := c.Wait()
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			waitErr = &common.BuildError{Inner: waitErr, ExitCode: exitErr.ExitCode()}
		}
		waitCh <- waitErr
	}()

	// Support process abort
	select {
	case err = <-waitCh:
		return err
	case <-cmd.Context.Done():
		logger := common.NewProcessLoggerAdapter(s.BuildLogger)
		return newProcessKillWaiter(logger, s.Config.GetGracefulKillTimeout(), s.Config.GetForceKillTimeout()).
			KillAndWait(c, waitCh)
	}
}

func (s *executor) shellScriptArgs(cmd common.ExecutorCommand, args []string) (io.Reader, []string, func(), error) {
	if !s.BuildShell.PassFile {
		return strings.NewReader(cmd.Script), args, func() {}, nil
	}

	scriptDir, err := os.MkdirTemp("", "build_script")
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
	err = os.WriteFile(scriptFile, []byte(cmd.Script), 0o700)
	if err != nil {
		return nil, nil, cleanup, fmt.Errorf("writing script file: %w", err)
	}

	return nil, append(args, scriptFile), cleanup, nil
}

func init() {
	// Look for self
	runnerCommand, err := os.Executable()
	if err != nil {
		logrus.Warningln(err)
	}

	RegisterExecutor("shell", runnerCommand)
}

func RegisterExecutor(executorName string, runnerCommandPath string) {
	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: false,
		DefaultBuildsDir:              "$PWD/builds",
		DefaultCacheDir:               "$PWD/cache",
		SharedBuildsDir:               true,
		Shell: common.ShellScriptInfo{
			Shell:         common.GetDefaultShell(),
			Type:          common.LoginShell,
			RunnerCommand: runnerCommandPath,
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

	common.RegisterExecutorProvider(executorName, executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}

package shell

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/kardianos/osext"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

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

	if s.BuildShell.PassFile {
		scriptDir, err := ioutil.TempDir("", "build_script")
		if err != nil {
			return err
		}
		defer os.RemoveAll(scriptDir)

		scriptFile := filepath.Join(scriptDir, "script."+s.BuildShell.Extension)
		err = ioutil.WriteFile(scriptFile, []byte(cmd.Script), 0700)
		if err != nil {
			return err
		}

		c.Args = append(c.Args, scriptFile)
	} else {
		c.Stdin = bytes.NewBufferString(cmd.Script)
	}

	// Start a process
	err := c.Start()
	if err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Wait for process to finish
	waitCh := make(chan error)
	go func() {
		err := c.Wait()
		if _, ok := err.(*exec.ExitError); ok {
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

package shell

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	krpty "github.com/kr/pty"

	"github.com/ayufan/gitlab-ci-multi-runner/common"
	"github.com/ayufan/gitlab-ci-multi-runner/executors"
	"github.com/ayufan/gitlab-ci-multi-runner/helpers"
	"io"
)

type ShellExecutor struct {
	executors.AbstractExecutor
	cmd       *exec.Cmd
	scriptDir string
	pty       *os.File
}

func (s *ShellExecutor) Prepare(config *common.RunnerConfig, build *common.Build) error {
	err := s.AbstractExecutor.Prepare(config, build)
	if err != nil {
		return err
	}

	s.Println("Using Shell executor...")
	return nil
}

func (s *ShellExecutor) Start() error {
	s.Debugln("Starting shell command...")

	// Create execution command
	s.cmd = exec.Command(s.ShellScript.Command, s.ShellScript.Arguments...)
	if s.cmd == nil {
		return errors.New("Failed to generate execution command")
	}

	helpers.SetProcessGroup(s.cmd)

	// Inherit environment from current process
	if !s.Config.CleanEnvironment {
		s.cmd.Env = os.Environ()
	}

	// Fill process environment variables
	s.cmd.Env = append(s.cmd.Env, s.ShellScript.Environment...)
	s.cmd.Env = append(s.cmd.Env, s.Config.Environment...)
	s.cmd.Stdout = s.BuildLog
	s.cmd.Stderr = s.BuildLog

	// Open PTY if available
	if s.Config.IsPTYEnabled() {
		pty, tty, err := krpty.Open()
		if err == nil {
			go io.Copy(s.BuildLog, pty)
			defer tty.Close()
			s.cmd.Stdout = tty
			s.cmd.Stderr = tty
			s.pty = pty
		} else if err != krpty.ErrUnsupported {
			s.Errorln("Failed to open pty", err)
		}
	}

	// Save shell script as file
	if s.ShellScript.PassFile {
		scriptDir, err := ioutil.TempDir("", "build_script")
		if err != nil {
			return err
		}
		s.scriptDir = scriptDir

		scriptFile := filepath.Join(scriptDir, "script."+s.ShellScript.Extension)
		err = ioutil.WriteFile(scriptFile, s.ShellScript.Script, 0700)
		if err != nil {
			return err
		}

		s.cmd.Args = append(s.cmd.Args, scriptFile)
	} else {
		s.cmd.Stdin = bytes.NewReader(s.ShellScript.Script)
	}

	// Start process
	err := s.cmd.Start()
	if err != nil {
		return errors.New("Failed to start process")
	}

	// Wait for process to exit
	go func() {
		s.BuildFinish <- s.cmd.Wait()
	}()
	return nil
}

func (s *ShellExecutor) Cleanup() {
	helpers.KillProcessGroup(s.cmd)

	if s.scriptDir != "" {
		os.RemoveAll(s.scriptDir)
	}

	if s.pty != nil {
		s.pty.Close()
	}

	s.AbstractExecutor.Cleanup()
}

func init() {
	common.RegisterExecutor("shell", func() common.Executor {
		return &ShellExecutor{
			AbstractExecutor: executors.AbstractExecutor{
				DefaultBuildsDir: "tmp/builds",
				DefaultShell:     common.GetDefaultShell(),
				ShowHostname:     false,
			},
		}
	})
}

package command

import (
	"os"
	"os/exec"
)

type commander interface {
	Start() error
	Wait() error
	Process() *os.Process
}

type cmd struct {
	internal *exec.Cmd
}

var newCmd = func(executable string, args []string, options CreateOptions) commander {
	c := exec.Command(executable, args...)
	c.Dir = options.Dir
	c.Env = options.Env
	c.Stdin = nil
	c.Stdout = options.Stdout
	c.Stderr = options.Stderr

	return &cmd{internal: c}
}

func (c *cmd) Start() error {
	return c.internal.Start()
}

func (c *cmd) Wait() error {
	return c.internal.Wait()
}

func (c *cmd) Process() *os.Process {
	return c.internal.Process
}

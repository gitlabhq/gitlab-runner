package process

import (
	"io"
	"os"
	"os/exec"
	"time"
)

type Commander interface {
	Start() error
	Wait() error
	Process() *os.Process
}

type CommandOptions struct {
	Dir string
	Env []string

	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	Logger Logger

	GracefulKillTimeout time.Duration
	ForceKillTimeout    time.Duration

	UseWindowsLegacyProcessStrategy bool
	UseWindowsJobObject             bool
}

// NewOSCmd creates a new implementation of Commander using the os.Cmd from
// os/exec.
func NewOSCmd(executable string, args []string, options CommandOptions) Commander {
	c := exec.Command(executable, args...)
	c.Dir = options.Dir
	c.Env = options.Env
	c.Stdin = options.Stdin
	c.Stdout = options.Stdout
	c.Stderr = options.Stderr

	return newOSCmd(c, options)
}

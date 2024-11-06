package runner_wrapper

import (
	"os"
	"os/exec"
)

//go:generate mockery --name=process --inpackage --with-expecter
type process interface {
	Signal(sig os.Signal) error
}

//go:generate mockery --name=commander --inpackage --with-expecter
type commander interface {
	Start() error
	Process() process
	Wait() error
}

type defaultCommander struct {
	cmd *exec.Cmd
}

func newDefaultCommander(path string, args []string) commander {
	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	setProcessGroup(cmd)

	return &defaultCommander{cmd: cmd}
}

func (d *defaultCommander) Start() error {
	return d.cmd.Start()
}

func (d *defaultCommander) Process() process {
	return d.cmd.Process
}

func (d *defaultCommander) Wait() error {
	return d.cmd.Wait()
}

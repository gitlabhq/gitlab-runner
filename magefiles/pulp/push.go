import (
	"fmt"
	"io"
	"os"
	"strings"
)

type (
	shRun  = func(string, ...string) error
	shExec = func(map[string]string, io.Writer, io.Writer, string, ...string) (bool, error)

	pusher interface {
		Push([]string, []string) error
	}

	basePusher struct {
		dryrun      bool
		branch      string
		concurrency int

		// testing hooks
		exec shExec
		run  shRun
	}
)

func (p *basePusher) runPulpCmd(args ...string) error {
	fmt.Println("executing", "pulp", strings.Join(args, " "))
	if p.dryrun {
		return nil
	}
	return p.run("pulp", args...)
}

func (p *basePusher) execCmd(out io.Writer, cmd string, args ...string) error {
	fmt.Println("executing", cmd, strings.Join(args, " "))
	if p.dryrun {
		return nil
	}
	_, err := p.exec(nil, out, os.Stderr, cmd, args...)
	return err
}


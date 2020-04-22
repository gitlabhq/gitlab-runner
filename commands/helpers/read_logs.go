package helpers

import (
	"fmt"

	"github.com/hpcloud/tail"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type ReadLogsCommand struct {
	Path  string `long:"path"`
	Index uint64 `long:"index"`
}

func (c *ReadLogsCommand) Execute(*cli.Context) {
	if err := c.readLogs(); err != nil {
		fmt.Println("error reading logs", err)
	}
}

func (c *ReadLogsCommand) readLogs() error {
	t, err := tail.TailFile(c.Path, tail.Config{
		ReOpen:      true,
		MustExist:   false,
		Follow:      true,
		MaxLineSize: 16 * 1024,
		Logger:      tail.DiscardingLogger,
	})
	if err != nil {
		return err
	}

	var index uint64
	for line := range t.Lines {
		if line.Err != nil {
			return err
		}

		if index < c.Index {
			index++
			continue
		}

		fmt.Println(fmt.Sprintf("%d %s", index, line.Text))
		index++
	}

	return nil
}

func init() {
	common.RegisterCommand2("read-logs", "reads logs from a file", &ReadLogsCommand{})
}

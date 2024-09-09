package commands

import (
	"bytes"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/store"
)

type StoreListJobsCommand struct {
	configOptions

	Runner string `short:"r" long:"runner" env:"RUNNER" description:"The name of the runner"`
}

func (c *StoreListJobsCommand) Execute(_ *cli.Context) {
	err := c.loadConfig()
	if err != nil {
		logrus.Fatalln("loading config", err)
	}

	var runner *common.RunnerConfig
	for _, r := range c.getConfig().Runners {
		if c.Runner == "" || r.Name == c.Runner {
			runner = r
			break
		}
	}

	if runner == nil {
		logrus.Fatalln(fmt.Sprintf("Runner %q not found", c.Runner))
	}

	provider := common.GetExecutorProvider(runner.Executor)
	if provider == nil {
		logrus.Fatalln(fmt.Sprintf("Executor %q is not known", runner.Executor))
	}

	jobStore, err := provider.GetStore(runner)
	if err != nil {
		logrus.Fatalln("getting store for runner", err)
	}

	jobs, err := jobStore.List()
	if err != nil {
		logrus.Fatalln("store list", err)
	}

	var data [][]byte
	for _, j := range jobs {
		j.JobResponse.Variables = nil
		j.JobResponse.Credentials = nil
		j.JobResponse.Token = ""

		var buf bytes.Buffer
		if err := (store.JSONJobCodec{}).Encode(&buf, j); err != nil {
			logrus.Fatalln(err)
		}

		data = append(data, buf.Bytes())
	}

	jsonData := fmt.Sprintf("[%s]", bytes.Join(data, []byte(",")))
	fmt.Println(jsonData)
}

func init() {
	common.RegisterCommand2("store-list-jobs", "List jobs in a store in a JSON format", &StoreListJobsCommand{})
}

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gitlab.com/gitlab-org/gitlab-runner/common"
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

	store, err := provider.GetStore(runner)
	if err != nil {
		logrus.Fatalln("getting store for runner", err)
	}

	jobs, err := store.List()
	if err != nil {
		logrus.Fatalln("store list", err)
	}

	if len(jobs) == 0 {
		return
	}

	for _, j := range jobs {
		j.JobResponse.Variables = nil
		j.JobResponse.Credentials = nil
		j.JobResponse.Token = ""
	}

	jobsJSON, err := json.Marshal(jobs)
	if err != nil {
		logrus.Fatalln("marshal jobs", err)
	}

	fmt.Println(string(jobsJSON))
}

func init() {
	common.RegisterCommand2("store-list-jobs", "List jobs in a store in a JSON format", &StoreListJobsCommand{})
}

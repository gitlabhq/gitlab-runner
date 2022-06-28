package commands

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type ListCommand struct {
	configOptions
}

func (c *ListCommand) Execute(context *cli.Context) {
	err := c.loadConfig()
	if err != nil {
		logrus.Warningln(err)
		return
	}

	logrus.WithFields(logrus.Fields{
		"ConfigFile": c.ConfigFile,
	}).Println("Listing configured runners")

	for _, runner := range c.getConfig().Runners {
		logrus.WithFields(logrus.Fields{
			"Executor": runner.RunnerSettings.Executor,
			"Token":    runner.RunnerCredentials.Token,
			"URL":      runner.RunnerCredentials.URL,
		}).Println(runner.Name)
	}
}

func init() {
	common.RegisterCommand2("list", "List all configured runners", &ListCommand{})
}

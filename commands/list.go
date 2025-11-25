package commands

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/internal/configfile"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type ListCommand struct {
	ConfigFile string `short:"c" long:"config" env:"CONFIG_FILE" description:"Config file"`
}

func NewListCommand() cli.Command {
	return common.NewCommand("list", "List all configured runners", &ListCommand{})
}

func (c *ListCommand) Execute(context *cli.Context) {
	cfg := configfile.New(c.ConfigFile)

	err := cfg.Load()
	if err != nil {
		logrus.Warningln(err)
		return
	}

	logrus.WithFields(logrus.Fields{
		"ConfigFile": c.ConfigFile,
	}).Println("Listing configured runners")

	for _, runner := range cfg.Config().Runners {
		logrus.WithFields(logrus.Fields{
			"Executor": runner.RunnerSettings.Executor,
			"Token":    runner.RunnerCredentials.Token,
			"URL":      runner.RunnerCredentials.URL,
		}).Println(runner.Name)
	}
}

package commands

import (
	"log"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/internal/configfile"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type ResetTokenCommand struct {
	*common.RunnerCredentials
	network common.Network

	ConfigFile string `short:"c" long:"config" env:"CONFIG_FILE" description:"Config file"`
	Name       string `short:"n" long:"name" description:"Name of the runner whose token you wish to reset (as defined in the configuration file)"`
	URL        string `short:"u" long:"url" description:"URL of the runner whose token you wish to reset (as defined in the configuration file)"`
	ID         int64  `short:"i" long:"id" description:"ID of the runner whose token you wish to reset (as defined in the configuration file)"`
	AllRunners bool   `long:"all-runners" description:"Reset all runner authentication tokens"`
	PAT        string `long:"pat" description:"Personal access token to use in lieu of runner's old authentication token"`
}

func NewResetTokenCommand(n common.Network) cli.Command {
	return common.NewCommand("reset-token", "reset a runner's token", &ResetTokenCommand{
		network: n,
	})
}

func (c *ResetTokenCommand) resetAllRunnerTokens(cfg *common.Config) {
	logrus.Warningln("Resetting all runner authentication tokens")
	for _, r := range cfg.Runners {
		if !common.ResetToken(c.network, r, "", c.PAT) {
			logrus.WithField("name", r.Name).Errorln("Failed to reset runner authentication token")
		}
	}
}

func (c *ResetTokenCommand) resetSingleRunnerToken(cfg *common.Config) bool {
	runnerCredentials, err := c.getRunnerCredentials(cfg)
	if err != nil {
		logrus.WithError(err).Fatalln("Couldn't get runner credentials")
	}

	if runnerCredentials == nil {
		logrus.Fatalln("No runner provided")
		return false
	}

	// Reset Token of the runner
	if !common.ResetToken(c.network, runnerCredentials, "", c.PAT) {
		logrus.WithFields(logrus.Fields{
			"name": c.Name,
			"id":   c.ID,
		}).Fatalln("Failed to reset runner authentication token")
		return false
	}

	return true
}

func (c *ResetTokenCommand) getRunnerCredentials(cfg *common.Config) (*common.RunnerConfig, error) {
	if c.Name != "" {
		runnerConfig, err := cfg.RunnerByName(c.Name)
		if err != nil {
			return nil, err
		}

		return runnerConfig, nil
	}

	runnerConfig, err := cfg.RunnerByURLAndID(c.URL, c.ID)
	if err != nil {
		return nil, err
	}

	return runnerConfig, nil
}

func (c *ResetTokenCommand) Execute(_context *cli.Context) {
	userModeWarning(true)

	cfg := configfile.New(c.ConfigFile)
	if err := cfg.Load(configfile.WithMutateOnLoad(func(cfg *common.Config) error {
		if c.AllRunners {
			c.resetAllRunnerTokens(cfg)
		} else {
			c.resetSingleRunnerToken(cfg)
		}

		return nil
	})); err != nil {
		logrus.WithError(err).Fatalln("Failed to load configuration")
	}

	if err := cfg.Save(); err != nil {
		logrus.WithError(err).Fatalln("Failed to update configuration")
	}

	log.Println("Updated")
}

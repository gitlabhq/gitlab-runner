package commands

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

//nolint:lll
type ResetTokenCommand struct {
	configOptions
	*common.RunnerCredentials
	network    common.Network
	Name       string `short:"n" long:"name" description:"Name of the runner whose token you wish to reset (as defined in the configuration file)"`
	URL        string `short:"u" long:"url" description:"URL of the runner whose token you wish to reset (as defined in the configuration file)"`
	ID         int64  `short:"i" long:"id" description:"ID of the runner whose token you wish to reset (as defined in the configuration file)"`
	AllRunners bool   `long:"all-runners" description:"Reset all runner tokens"`
	PAT        string `long:"pat" description:"Personal access token to use in lieu of runner's old token"`
}

func (c *ResetTokenCommand) resetAllRunnerTokens() {
	logrus.Warningln("Resetting all runner tokens")
	for _, r := range c.config.Runners {
		if !common.ResetToken(c.network, &r.RunnerCredentials, "", c.PAT) {
			logrus.WithField("name", r.Name).Errorln("Failed to reset runner token")
		}
	}
}

func (c *ResetTokenCommand) resetSingleRunnerToken() bool {
	runnerCredentials, err := c.getRunnerCredentials()
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
		}).Fatalln("Failed to reset runner token")
		return false
	}

	return true
}

func (c *ResetTokenCommand) getRunnerCredentials() (*common.RunnerCredentials, error) {
	if len(c.Name) > 0 {
		runnerConfig, err := c.RunnerByName(c.Name)
		if err != nil {
			return nil, err
		}

		return &runnerConfig.RunnerCredentials, nil
	}

	runnerConfig, err := c.RunnerByURLAndID(c.URL, c.ID)
	if err != nil {
		return nil, err
	}

	return &runnerConfig.RunnerCredentials, nil
}

func (c *ResetTokenCommand) Execute(_context *cli.Context) {
	userModeWarning(true)

	log := logrus.WithField("config-file", c.config)

	err := c.loadConfig()
	if err != nil {
		log.WithError(err).Fatalln("Failed to load configuration")
	}

	if c.AllRunners {
		c.resetAllRunnerTokens()
	} else {
		c.resetSingleRunnerToken()
	}

	// save config file
	err = c.saveConfig()
	if err != nil {
		log.WithError(err).Fatalln("Failed to update configuration")
	}
	log.Println("Updated")
}

func init() {
	common.RegisterCommand2("reset-token", "reset a runner's token", &ResetTokenCommand{
		network: network.NewGitLabClient(),
	})
}

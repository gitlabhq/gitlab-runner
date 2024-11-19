package commands

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

type UnregisterCommand struct {
	configOptions
	common.RunnerCredentials
	network    common.Network
	Name       string `toml:"name" json:"name" short:"n" long:"name" description:"Name of the runner you wish to unregister"`
	AllRunners bool   `toml:"all_runners" json:"all-runners" long:"all-runners" description:"Unregister all runners"`
}

func (c *UnregisterCommand) unregisterAllRunners() (runners []*common.RunnerConfig) {
	logrus.Warningln("Unregistering all runners")
	for _, r := range c.getConfig().Runners {
		if !c.unregisterRunner(r.RunnerCredentials) {
			logrus.Errorln("Failed to unregister runner", r.Name)
			// If unregister fails, leave the runner in the config
			runners = append(runners, r)
		}
	}
	return
}

func (c *UnregisterCommand) unregisterSingleRunner() []*common.RunnerConfig {
	var runnerConfig *common.RunnerConfig
	var err error
	switch {
	case c.Name != "" && c.Token != "":
		runnerConfig, err = c.RunnerByNameAndToken(c.Name, c.Token)
	case c.Token != "":
		runnerConfig, err = c.RunnerByToken(c.Token)
	case c.Name != "":
		runnerConfig, err = c.RunnerByName(c.Name)
	default:
		logrus.Fatalln("at least one of --name or --token must be specified")
	}
	if err != nil {
		logrus.Fatalln(err)
	}

	c.RunnerCredentials = runnerConfig.RunnerCredentials

	// Unregister given Token and URL of the runner
	if !c.unregisterRunner(c.RunnerCredentials) {
		logrus.Fatalln("Failed to unregister runner", c.Name)
	}

	var runners []*common.RunnerConfig
	for _, otherRunner := range c.getConfig().Runners {
		if otherRunner.RunnerCredentials != c.RunnerCredentials {
			runners = append(runners, otherRunner)
		}
	}
	return runners
}

func (c *UnregisterCommand) unregisterRunner(r common.RunnerCredentials) bool {
	if network.TokenIsCreatedRunnerToken(r.Token) {
		return c.network.UnregisterRunnerManager(r, c.loadedSystemIDState.GetSystemID())
	}

	return c.network.UnregisterRunner(r)
}

func (c *UnregisterCommand) Execute(context *cli.Context) {
	userModeWarning(false)

	err := c.loadConfig()
	if err != nil {
		logrus.Fatalln(err)
	}

	var runners []*common.RunnerConfig
	if c.AllRunners {
		runners = c.unregisterAllRunners()
	} else {
		runners = c.unregisterSingleRunner()
	}

	// check if anything changed
	if len(c.getConfig().Runners) == len(runners) {
		return
	}

	c.configMutex.Lock()
	c.config.Runners = runners
	c.configMutex.Unlock()

	// save config file
	err = c.saveConfig()
	if err != nil {
		logrus.Fatalln("Failed to update", c.ConfigFile, err)
	}
	logrus.Println("Updated", c.ConfigFile)
}

func init() {
	common.RegisterCommand2("unregister", "unregister specific runner", &UnregisterCommand{
		network: network.NewGitLabClient(),
	})
}

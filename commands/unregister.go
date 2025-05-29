package commands

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/internal/configfile"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

type UnregisterCommand struct {
	common.RunnerCredentials
	network common.Network

	ConfigFile string `short:"c" long:"config" env:"CONFIG_FILE" description:"Config file"`
	Name       string `toml:"name" json:"name" short:"n" long:"name" description:"Name of the runner you wish to unregister"`
	AllRunners bool   `toml:"all_runners" json:"all-runners" long:"all-runners" description:"Unregister all runners"`
}

func (c *UnregisterCommand) unregisterAllRunners(cfg *common.Config) (runners []*common.RunnerConfig) {
	logrus.Warningln("Unregistering all runners")
	for _, r := range cfg.Runners {
		if !c.unregisterRunner(r.RunnerCredentials, r.SystemID) {
			logrus.Errorln("Failed to unregister runner", r.Name)
			// If unregister fails, leave the runner in the config
			runners = append(runners, r)
		}
	}
	return
}

func (c *UnregisterCommand) unregisterSingleRunner(cfg *common.Config) []*common.RunnerConfig {
	var runnerConfig *common.RunnerConfig
	var err error
	switch {
	case c.Name != "" && c.Token != "":
		runnerConfig, err = cfg.RunnerByNameAndToken(c.Name, c.Token)
	case c.Token != "":
		runnerConfig, err = cfg.RunnerByToken(c.Token)
	case c.Name != "":
		runnerConfig, err = cfg.RunnerByName(c.Name)
	default:
		logrus.Fatalln("at least one of --name or --token must be specified")
	}
	if err != nil {
		logrus.Fatalln(err)
	}

	c.RunnerCredentials = runnerConfig.RunnerCredentials

	// Unregister given Token and URL of the runner
	if !c.unregisterRunner(c.RunnerCredentials, runnerConfig.SystemID) {
		logrus.Fatalln("Failed to unregister runner", c.Name)
	}

	var runners []*common.RunnerConfig
	for _, otherRunner := range cfg.Runners {
		if otherRunner.RunnerCredentials != c.RunnerCredentials {
			runners = append(runners, otherRunner)
		}
	}
	return runners
}

func (c *UnregisterCommand) unregisterRunner(r common.RunnerCredentials, systemID string) bool {
	if network.TokenIsCreatedRunnerToken(r.Token) {
		return c.network.UnregisterRunnerManager(r, systemID)
	}

	return c.network.UnregisterRunner(r)
}

func (c *UnregisterCommand) Execute(context *cli.Context) {
	userModeWarning(false)

	cfg := configfile.New(c.ConfigFile)

	var changed bool
	if err := cfg.Load(configfile.WithMutateOnLoad(func(cfg *common.Config) error {
		var runners []*common.RunnerConfig
		if c.AllRunners {
			runners = c.unregisterAllRunners(cfg)
		} else {
			runners = c.unregisterSingleRunner(cfg)
		}

		changed = len(cfg.Runners) != len(runners)
		if changed {
			cfg.Runners = runners
		}

		return nil
	})); err != nil {
		logrus.Fatalln(err)
	}

	// check if anything changed
	if !changed {
		return
	}

	// save config file
	if err := cfg.Save(); err != nil {
		logrus.Fatalln("Failed to update", c.ConfigFile, err)
	}
	logrus.Println("Updated", c.ConfigFile)
}

func init() {
	common.RegisterCommand2("unregister", "unregister specific runner", &UnregisterCommand{
		network: network.NewGitLabClient(),
	})
}

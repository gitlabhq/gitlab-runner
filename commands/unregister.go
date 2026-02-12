package commands

import (
	"errors"
	"fmt"

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

func NewUnregisterCommand(n common.Network) cli.Command {
	return common.NewCommand("unregister", "unregister specific runner", &UnregisterCommand{
		network: n,
	})
}

func (c *UnregisterCommand) unregisterAllRunners(cfg *common.Config) ([]*common.RunnerConfig, error) {
	logrus.Warningln("Unregistering all runners")
	var errs error
	var runners []*common.RunnerConfig

	for _, r := range cfg.Runners {
		if !c.unregisterRunner(*r, r.SystemID) {
			errs = errors.Join(errs, fmt.Errorf("failed to unregister runner %q", r.Name))
			// If unregister fails, leave the runner in the config
			runners = append(runners, r)
		}
	}
	return runners, errs
}

func (c *UnregisterCommand) unregisterSingleRunner(cfg *common.Config) ([]*common.RunnerConfig, error) {
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
		return nil, errors.New("at least one of --name or --token must be specified")
	}
	if err != nil {
		return nil, fmt.Errorf("get runner by token or name: %w", err)
	}

	c.RunnerCredentials = runnerConfig.RunnerCredentials

	// Unregister given Token and URL of the runner
	if !c.unregisterRunner(*runnerConfig, runnerConfig.SystemID) {
		return nil, fmt.Errorf("failed to unregister runner %q", c.Name)
	}

	var runners []*common.RunnerConfig
	for _, otherRunner := range cfg.Runners {
		if otherRunner.RunnerCredentials != c.RunnerCredentials {
			runners = append(runners, otherRunner)
		}
	}
	return runners, nil
}

func (c *UnregisterCommand) unregisterRunner(r common.RunnerConfig, systemID string) bool {
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
		var err error
		if c.AllRunners {
			runners, err = c.unregisterAllRunners(cfg)
			if err != nil {
				logrus.WithError(err).Errorln("Failed to unregister runners")
			}
		} else {
			runners, err = c.unregisterSingleRunner(cfg)
			if err != nil {
				return fmt.Errorf("unregister runner: %w", err)
			}
		}

		changed = len(cfg.Runners) != len(runners)
		if changed {
			cfg.Runners = runners
		}

		return nil
	})); err != nil {
		logrus.WithError(err).Fatalln("failed to unregister runner")
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

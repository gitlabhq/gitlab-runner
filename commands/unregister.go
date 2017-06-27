package commands

import (
	"github.com/codegangsta/cli"

	log "github.com/Sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/network"
)

type UnregisterCommand struct {
	configOptions
	common.RunnerCredentials
	network    common.Network
	Name       string `toml:"name" json:"name" short:"n" long:"name" description:"Name of the runner you wish to unregister"`
	AllRunners bool   `toml:"all" json:"all" short:"a" long:"all" description:"Unregister all runners"`
}

func (c *UnregisterCommand) UnregisterRunner(name string) bool {
	runnerConfig, err := c.RunnerByName(name)
	if err != nil {
		log.Fatalln(err)
		return false
	}
	if !c.network.UnregisterRunner(runnerConfig.RunnerCredentials) {
		log.Fatalln("Failed to unregister runner", name)
		return false
	}
	return true
}

func (c *UnregisterCommand) Execute(context *cli.Context) {
	userModeWarning(false)

	err := c.loadConfig()
	if err != nil {
		log.Fatalln(err)
		return
	}

	// Final runner slice
	runners := []*common.RunnerConfig{}

	if c.AllRunners {
		// Unregister all runners
		for _, r := range c.config.Runners {
			if !c.UnregisterRunner(r.Name) {
				runners = append(runners, r)
			}
		}
	} else {
		// Unregister single runner
		c.UnregisterRunner(c.Name)
		for _, otherRunner := range c.config.Runners {
			if otherRunner.RunnerCredentials == c.RunnerCredentials {
				continue
			}
			runners = append(runners, otherRunner)
		}
	}

	// check if anything changed
	if len(c.config.Runners) == len(runners) {
		return
	}

	c.config.Runners = runners

	// save config file
	err = c.saveConfig()
	if err != nil {
		log.Fatalln("Failed to update", c.ConfigFile, err)
	}
	log.Println("Updated", c.ConfigFile)
}

func init() {
	common.RegisterCommand2("unregister", "unregister specific runner", &UnregisterCommand{
		network: network.NewGitLabClient(),
	})
}

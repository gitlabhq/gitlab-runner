package commands

import (
	"github.com/codegangsta/cli"

	log "github.com/Sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/network"
)

type VerifyCommand struct {
	configOptions
	common.RunnerCredentials
	network           common.Network
	Name              string `toml:"name" json:"name" short:"n" long:"name" description:"Name of the runner you wish to verify"`
	DeleteNonExisting bool   `long:"delete" description:"Delete no longer existing runners?"`
}

func (c *VerifyCommand) Execute(context *cli.Context) {
	userModeWarning(true)

	err := c.loadConfig()
	if err != nil {
		log.Fatalln(err)
		return
	}

	hasFilters := c.Name != "" || c.RunnerCredentials.UniqueID() != ""
	// select runners to verify
	toVerify, okRunners := c.selectRunners()

	// check if there's something to verify
	if hasFilters && len(toVerify) == 0 {
		log.Fatalln("No runner matches the filtering parameters")
		return
	}

	// verify if runner exist
	for _, runner := range toVerify {
		if c.network.VerifyRunner(runner.RunnerCredentials) {
			okRunners = append(okRunners, runner)
		}
	}

	// check if anything changed
	if len(c.config.Runners) == len(okRunners) {
		return
	}

	if !c.DeleteNonExisting {
		log.Fatalln("Failed to verify runners")
		return
	}

	c.config.Runners = okRunners

	// save config file
	err = c.saveConfig()
	if err != nil {
		log.Fatalln("Failed to update", c.ConfigFile, err)
	}
	log.Println("Updated", c.ConfigFile)
}

func (c *VerifyCommand) selectRunners() (toVerify []*common.RunnerConfig, okRunners []*common.RunnerConfig) {
	for _, runner := range c.config.Runners {
		skip := false

		if len(c.Name) > 0 {
			skip = runner.Name != c.Name
		} else if c.RunnerCredentials.UniqueID() != "" {
			skip = runner.RunnerCredentials.UniqueID() != c.RunnerCredentials.UniqueID()
		}

		if skip {
			okRunners = append(okRunners, runner)
		} else {
			toVerify = append(toVerify, runner)
		}
	}

	return
}
func init() {
	common.RegisterCommand2("verify", "verify all registered runners", &VerifyCommand{
		network: network.NewGitLabClient(),
	})
}

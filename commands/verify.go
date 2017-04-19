package commands

import (
	"errors"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"

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

	// check if there's something to verify
	toVerify, okRunners, err := c.selectRunners()
	if err != nil {
		log.Fatalln(err)
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

func (c *VerifyCommand) selectRunners() (toVerify []*common.RunnerConfig, okRunners []*common.RunnerConfig, err error) {
	var selectorPresent = c.Name != "" || c.RunnerCredentials.URL != "" || c.RunnerCredentials.Token != ""

	for _, runner := range c.config.Runners {
		selected := !selectorPresent || runner.Name == c.Name || runner.RunnerCredentials.SameAs(&c.RunnerCredentials)

		if selected {
			toVerify = append(toVerify, runner)
		} else {
			okRunners = append(okRunners, runner)
		}
	}

	if selectorPresent && len(toVerify) == 0 {
		err = errors.New("No runner matches the filtering parameters")
	}
	return
}

func init() {
	common.RegisterCommand2("verify", "verify all registered runners", &VerifyCommand{
		network: network.NewGitLabClient(),
	})
}

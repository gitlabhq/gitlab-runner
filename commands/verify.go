package commands

import (
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

//nolint:lll
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
		logrus.Fatalln(err)
		return
	}

	// check if there's something to verify
	toVerify, okRunners, err := c.selectRunners()
	if err != nil {
		logrus.Fatalln(err)
		return
	}

	// verify if runner exist
	for _, runner := range toVerify {
		if c.network.VerifyRunner(runner.RunnerCredentials, runner.SystemIDState.GetSystemID()) != nil {
			okRunners = append(okRunners, runner)
		}
	}

	// check if anything changed
	if len(c.getConfig().Runners) == len(okRunners) {
		return
	}

	if !c.DeleteNonExisting {
		logrus.Fatalln("Failed to verify runners")
		return
	}

	c.configMutex.Lock()
	c.config.Runners = okRunners
	c.configMutex.Unlock()

	// save config file
	err = c.saveConfig()
	if err != nil {
		logrus.Fatalln("Failed to update", c.ConfigFile, err)
	}
	logrus.Println("Updated", c.ConfigFile)
}

func (c *VerifyCommand) selectRunners() (toVerify, okRunners []*common.RunnerConfig, err error) {
	var selectorPresent = c.Name != "" || c.RunnerCredentials.URL != "" || c.RunnerCredentials.Token != ""

	for _, runner := range c.getConfig().Runners {
		selected := !selectorPresent || runner.Name == c.Name || runner.RunnerCredentials.SameAs(&c.RunnerCredentials)

		if selected {
			toVerify = append(toVerify, runner)
		} else {
			okRunners = append(okRunners, runner)
		}
	}

	if selectorPresent && len(toVerify) == 0 {
		err = errors.New("no runner matches the filtering parameters")
	}
	return
}

func init() {
	common.RegisterCommand2("verify", "verify all registered runners", &VerifyCommand{
		network: network.NewGitLabClient(),
	})
}

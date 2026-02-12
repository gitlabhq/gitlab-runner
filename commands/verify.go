package commands

import (
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/internal/configfile"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type VerifyCommand struct {
	common.RunnerCredentials
	network common.Network

	ConfigFile        string `short:"c" long:"config" env:"CONFIG_FILE" description:"Config file"`
	Name              string `toml:"name" json:"name" short:"n" long:"name" description:"Name of the runner you wish to verify"`
	DeleteNonExisting bool   `long:"delete" description:"Delete no longer existing runners?"`
}

func NewVerifyCommand(n common.Network) cli.Command {
	return common.NewCommand("verify", "verify all registered runners", &VerifyCommand{
		network: n,
	})
}

//nolint:gocognit
func (c *VerifyCommand) Execute(context *cli.Context) {
	userModeWarning(true)

	var hasSelector = c.Name != "" ||
		c.RunnerCredentials.URL != "" ||
		c.RunnerCredentials.Token != ""

	cfg := configfile.New(c.ConfigFile)

	var unverified int
	if err := cfg.Load(configfile.WithMutateOnLoad(func(cfg *common.Config) error {
		var ok []*common.RunnerConfig
		var verified int
		for _, runner := range cfg.Runners {
			if !hasSelector || runner.Name == c.Name || runner.RunnerCredentials.SameAs(&c.RunnerCredentials) {
				verified++
				if c.network.VerifyRunner(*runner, runner.SystemID) == nil {
					unverified++
					continue
				}
			}

			ok = append(ok, runner)
		}

		// update config runners
		cfg.Runners = ok

		if hasSelector && verified == 0 {
			return errors.New("no runner matches the filtering parameters")
		}

		return nil
	})); err != nil {
		logrus.Fatalln(err)
	}

	// check if anything changed
	if unverified == 0 {
		return
	}

	if !c.DeleteNonExisting {
		logrus.Fatalln("Failed to verify runners")
		return
	}

	// save config file
	if err := cfg.Save(); err != nil {
		logrus.Fatalln("Failed to update", c.ConfigFile, err)
	}
	logrus.Println("Updated", c.ConfigFile)
}

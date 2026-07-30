package commands

import (
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/internal/configfile"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type VerifyCommand struct {
	network common.Network

	ConfigFile string `short:"c" long:"config" env:"CONFIG_FILE" description:"Config file"`
	Name       string `toml:"name" json:"name" short:"n" long:"name" description:"Name of the runner you wish to verify"`
	// URL and Token intentionally have no `env` tag: unlike register/run/unregister,
	// verify must not be silently filtered by a leftover or unrelated CI_SERVER_URL/
	// CI_SERVER_TOKEN in the process environment (e.g. injected by a Kubernetes
	// secrets sidecar) - only an explicit --url/--token should select a subset of
	// runners. See https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/577.
	URL               string `short:"u" long:"url" description:"GitLab instance URL"`
	Token             string `short:"t" long:"token" description:"Runner token"`
	DeleteNonExisting bool   `long:"delete" description:"Delete no longer existing runners?"`
}

func NewVerifyCommand(n common.Network) cli.Command {
	return common.NewCommand("verify", "verify all registered runners", &VerifyCommand{
		network:    n,
		ConfigFile: GetDefaultConfigFile(),
	})
}

//nolint:gocognit
func (c *VerifyCommand) Execute(context *cli.Context) {
	userModeWarning(true)

	var hasSelector = c.Name != "" ||
		c.URL != "" ||
		c.Token != ""

	selector := &common.RunnerCredentials{URL: c.URL, Token: c.Token}

	cfg := configfile.New(c.ConfigFile)

	var unverified int
	if err := cfg.Load(configfile.WithMutateOnLoad(func(cfg *common.Config) error {
		var ok []*common.RunnerConfig
		var verified int
		for _, runner := range cfg.Runners {
			if !hasSelector || runner.Name == c.Name || runner.RunnerCredentials.SameAs(selector) {
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

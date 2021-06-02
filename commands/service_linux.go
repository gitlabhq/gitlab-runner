package commands

import (
	"os"

	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	service_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/service"
)

func setupOSServiceConfig(c *cli.Context, config *service.Config) {
	if os.Getuid() != 0 {
		logrus.Fatal("The --user is not supported for non-root users")
	}

	user := c.String("user")
	if user != "" {
		config.Arguments = append(config.Arguments, "--user", user)
	}

	switch service.Platform() {
	case "linux-systemd":
		config.Dependencies = []string{
			"After=syslog.target network.target",
		}
		config.Option = service.KeyValue{
			"Restart": "always",
		}
	case "unix-systemv":
		script := service_helpers.SysvScript()
		if script != "" {
			config.Option = service.KeyValue{
				"SysvScript": script,
			}
		}
	}
}

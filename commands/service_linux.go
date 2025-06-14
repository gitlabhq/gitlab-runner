package commands

import (
	"github.com/kardianos/service"
	"github.com/urfave/cli"
	service_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/service"
)

func setupOSServiceConfig(c *cli.Context, config *service.Config) {
	applyStrArg(c, "user", true, func(val string) { config.Arguments = append(config.Arguments, "--user", val) })
	applyStrArg(c, "init-user", true, func(val string) { config.UserName = val })

	switch service.Platform() {
	case "linux-systemd":
		config.Dependencies = []string{
			"After=network.target",
		}
		config.Option = service.KeyValue{
			"Restart":     "always",
			"UserService": c.IsSet("user-service"),
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

package commands

import (
	"os"

	"github.com/kardianos/service"
	"github.com/urfave/cli"
)

func setupOSServiceConfig(c *cli.Context, config *service.Config) {
	config.Option = service.KeyValue{
		"KeepAlive":   true,
		"RunAtLoad":   true,
		"UserService": os.Getuid() != 0,
	}

	applyStrArg(c, "user", true, func(val string) { config.Arguments = append(config.Arguments, "--user", val) })
	applyStrArg(c, "init-user", true, func(val string) { config.UserName = val })
}

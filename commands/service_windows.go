package commands

import (
	"github.com/kardianos/service"
	"github.com/urfave/cli"
)

func setupOSServiceConfig(c *cli.Context, config *service.Config) {
	config.Option = service.KeyValue{
		"Password": c.String("password"),
	}
	config.UserName = c.String("user")
}

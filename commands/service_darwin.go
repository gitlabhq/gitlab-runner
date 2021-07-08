package commands

import (
	"os"

	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func setupOSServiceConfig(c *cli.Context, config *service.Config) {
	config.Option = service.KeyValue{
		"KeepAlive":   true,
		"RunAtLoad":   true,
		"UserService": os.Getuid() != 0,
	}

	user := c.String("user")
	if user == "" {
		return
	}

	if os.Getuid() != 0 {
		logrus.Fatal("The --user is not supported for non-root users")
	}

	config.Arguments = append(config.Arguments, "--user", user)
}

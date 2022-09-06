//go:build !linux && !darwin && !windows

package commands

import (
	"github.com/kardianos/service"
	"github.com/urfave/cli"
)

func setupOSServiceConfig(c *cli.Context, config *service.Config) {
	// not supported
}

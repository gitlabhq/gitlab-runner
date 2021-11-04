package common

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	clihelpers "gitlab.com/gitlab-org/golang-cli-helpers"
)

var commands []cli.Command

type Commander interface {
	Execute(c *cli.Context)
}

func RegisterCommand(command cli.Command) {
	logrus.Debugln("Registering", command.Name, "command...")
	commands = append(commands, command)
}

func RegisterCommand2(name, usage string, data Commander, flags ...cli.Flag) {
	RegisterCommand(cli.Command{
		Name:   name,
		Usage:  usage,
		Action: data.Execute,
		Flags:  append(flags, clihelpers.GetFlagsFromStruct(data)...),
	})
}

func GetCommands() []cli.Command {
	return commands
}

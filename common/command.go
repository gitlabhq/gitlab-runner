package common

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	clihelpers "gitlab.com/gitlab-org/golang-cli-helpers"
)

var commands []cli.Command

// Commander executes the command with the cli.Context.
type Commander interface {
	Execute(c *cli.Context)
}

// CommanderFunc allows the registration of commands without having to explicitly implement
// the Commander interface for simple functions.

type CommanderFunc func(*cli.Context)

// Execute provides default implementation for Commander interface.
func (cf CommanderFunc) Execute(c *cli.Context) {
	cf(c)
}

func registerCommand(command cli.Command) {
	logrus.Debugln("Registering", command.Name, "command...")
	commands = append(commands, command)
}

// RegisterCommand registers a command with the given name, usage, and flags.
func RegisterCommand(name, usage string, data Commander, flags ...cli.Flag) {
	registerCommand(cli.Command{
		Name:   name,
		Usage:  usage,
		Action: data.Execute,
		Flags:  append(flags, clihelpers.GetFlagsFromStruct(data)...),
	})
}

// RegisterCommandWithSubcommands registers a command with the given name, usage, data, subcommands, and flags.
func RegisterCommandWithSubcommands(name, usage string, data Commander, hidden bool, subcommands []cli.Command, flags ...cli.Flag) {
	registerCommand(cli.Command{
		Name:        name,
		Usage:       usage,
		Action:      data.Execute,
		Flags:       append(flags, clihelpers.GetFlagsFromStruct(data)...),
		Hidden:      hidden,
		Subcommands: subcommands,
	})
}

// GetCommands returns the registered commands.
func GetCommands() []cli.Command {
	return commands
}

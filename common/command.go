package common

import (
	"github.com/urfave/cli"
	clihelpers "gitlab.com/gitlab-org/golang-cli-helpers"
)

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

// NewCommand constructs a command with the given name, usage, and flags.
func NewCommand(name, usage string, data Commander, flags ...cli.Flag) cli.Command {
	return cli.Command{
		Name:   name,
		Usage:  usage,
		Action: data.Execute,
		Flags:  append(flags, clihelpers.GetFlagsFromStruct(data)...),
	}
}

// NewCommandWithSubcommands returns a command with the given name, usage, data, subcommands, and flags.
func NewCommandWithSubcommands(name, usage string, data Commander, hidden bool, subcommands []cli.Command, flags ...cli.Flag) cli.Command {
	return cli.Command{
		Name:        name,
		Usage:       usage,
		Action:      data.Execute,
		Flags:       append(flags, clihelpers.GetFlagsFromStruct(data)...),
		Hidden:      hidden,
		Subcommands: subcommands,
	}
}

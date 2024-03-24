package fleeting

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gitlab.com/gitlab-org/fleeting/fleeting-artifact/pkg/installer"

	"gitlab.com/gitlab-org/gitlab-runner/commands"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type runnerFleetingPlugin struct {
	RunnerName string
	Plugin     string
}

func getPlugins(context *cli.Context) []runnerFleetingPlugin {
	config := common.NewConfig()

	err := config.LoadConfig(context.Parent().String("config"))
	if err != nil {
		logrus.Fatalln(err)
	}

	var results []runnerFleetingPlugin
	for _, runnerCfg := range config.Runners {
		if runnerCfg.Autoscaler == nil {
			continue
		}

		results = append(results, runnerFleetingPlugin{
			RunnerName: runnerCfg.ShortDescription(),
			Plugin:     runnerCfg.Autoscaler.Plugin,
		})
	}

	return results
}

func install(clictx *cli.Context) {
	var exitCode int
	for _, plugin := range getPlugins(clictx) {
		_, err := installer.LookPath(plugin.Plugin, "")
		if !errors.Is(err, installer.ErrPluginNotFound) && !clictx.Bool("upgrade") {
			continue
		}

		if err := installer.Install(context.Background(), plugin.Plugin); err != nil {
			exitCode = 1
			fmt.Fprintf(os.Stderr, "runner: %v, plugin: %v, install/update error:: %v\n", plugin.RunnerName, plugin.Plugin, err)
			continue
		}

		path, _ := installer.LookPath(plugin.Plugin, "")
		fmt.Printf("runner: %v, plugin: %v, path: %v\n", plugin.RunnerName, plugin.Plugin, path)
	}

	os.Exit(exitCode)
}

func list(clictx *cli.Context) {
	var exitCode int
	for _, plugin := range getPlugins(clictx) {
		path, err := installer.LookPath(plugin.Plugin, "")
		if err != nil {
			exitCode = 1
			fmt.Fprintf(os.Stderr, "runner: %v, plugin: %v, error: %v\n", plugin.RunnerName, plugin.Plugin, err)
			continue
		}

		fmt.Printf("runner: %v, plugin: %v, path: %v\n", plugin.RunnerName, plugin.Plugin, path)
	}

	os.Exit(exitCode)
}

func login(clictx *cli.Context) error {
	password := clictx.String("password")

	if clictx.Bool("password-stdin") {
		pass, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Println("reading password from stdin:", err)
			os.Exit(1)
		}
		password = strings.TrimSuffix(strings.TrimSuffix(string(pass), "\n"), "\r")
	}

	via, err := installer.Login(clictx.Args().Get(0), clictx.String("username"), password)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	fmt.Println("logged in via", via)

	return nil
}

func init() {
	common.RegisterCommand(cli.Command{
		Name:  "fleeting",
		Usage: "manage fleeting plugins",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "config, c", EnvVar: "CONFIG_FILE", Value: commands.GetDefaultConfigFile()},
		},
		Subcommands: []cli.Command{
			{
				Name:   "install",
				Usage:  "install or update fleeting plugins",
				Flags:  []cli.Flag{cli.BoolFlag{Name: "upgrade"}},
				Action: install,
			},
			{
				Name:   "list",
				Usage:  "list installed plugins",
				Action: list,
			},
			{
				Name:  "login",
				Usage: "login to container registry",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "username"},
					cli.StringFlag{Name: "password"},
					cli.BoolFlag{Name: "password-stdin", Usage: "take the password from stdin"},
				},
				ArgsUsage: "[server]",
				Action:    login,
			},
		},
	})
}

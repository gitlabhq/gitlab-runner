package commands

import (
	"fmt"
	"os"
	"runtime"

	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	service_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/service"
)

const (
	defaultServiceName = "gitlab-runner"
	defaultDescription = "GitLab Runner"
)

type NullService struct {
}

func (n *NullService) Start(s service.Service) error {
	return nil
}

func (n *NullService) Stop(s service.Service) error {
	return nil
}

func runServiceInstall(s service.Service, c *cli.Context) error {
	if user := c.String("user"); user == "" && os.Getuid() == 0 {
		logrus.Fatal("Please specify user that will run gitlab-runner service")
	}

	if configFile := c.String("config"); configFile != "" {
		// try to load existing config
		config := common.NewConfig()
		err := config.LoadConfig(configFile)
		if err != nil {
			return err
		}

		// save config for the first time
		if !config.Loaded {
			err = config.SaveConfig(configFile)
			if err != nil {
				return err
			}
		}
	}
	return service.Control(s, "install")
}

func runServiceStatus(displayName string, s service.Service) {
	status, err := s.Status()

	description := ""
	switch status {
	case service.StatusRunning:
		description = "Service is running"
	case service.StatusStopped:
		description = "Service has stopped"
	default:
		description = "Service status unknown"
		if err != nil {
			description = err.Error()
		}
	}

	if status != service.StatusRunning {
		fmt.Fprintf(os.Stderr, "%s: %s\n", displayName, description)
		os.Exit(1)
	}

	fmt.Printf("%s: %s\n", displayName, description)
}

func GetServiceArguments(c *cli.Context) (arguments []string) {
	if wd := c.String("working-directory"); wd != "" {
		arguments = append(arguments, "--working-directory", wd)
	}

	if config := c.String("config"); config != "" {
		arguments = append(arguments, "--config", config)
	}

	if sn := c.String("service"); sn != "" {
		arguments = append(arguments, "--service", sn)
	}

	// syslogging doesn't make sense for systemd systems as those log straight to journald
	syslog := !c.IsSet("syslog") || c.Bool("syslog")
	if service.Platform() == "linux-systemd" && !c.IsSet("syslog") {
		syslog = false
	}

	if syslog {
		arguments = append(arguments, "--syslog")
	}

	return
}

func createServiceConfig(c *cli.Context) *service.Config {
	config := &service.Config{
		Name:        c.String("service"),
		DisplayName: c.String("service"),
		Description: defaultDescription,
		Arguments:   append([]string{"run"}, GetServiceArguments(c)...),
	}

	// setup os specific service config
	setupOSServiceConfig(c, config)

	return config
}

func RunServiceControl(c *cli.Context) {
	svcConfig := createServiceConfig(c)

	s, err := service_helpers.New(&NullService{}, svcConfig)
	if err != nil {
		logrus.Fatal(err)
	}

	switch c.Command.Name {
	case "install":
		err = runServiceInstall(s, c)
	case "status":
		runServiceStatus(svcConfig.DisplayName, s)
	default:
		err = service.Control(s, c.Command.Name)
	}

	if err != nil {
		logrus.Fatal(err)
	}
}

func GetFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:  "service, n",
			Value: defaultServiceName,
			Usage: "Specify service name to use",
		},
	}
}

func GetInstallFlags() []cli.Flag {
	installFlags := GetFlags()
	installFlags = append(
		installFlags,
		cli.StringFlag{
			Name:  "working-directory, d",
			Value: helpers.GetCurrentWorkingDirectory(),
			Usage: "Specify custom root directory where all data are stored",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: GetDefaultConfigFile(),
			Usage: "Specify custom config file",
		},
		cli.BoolFlag{
			Name:  "syslog",
			Usage: "Setup system logging integration",
		},
	)

	if runtime.GOOS == osTypeWindows {
		installFlags = append(
			installFlags,
			cli.StringFlag{
				Name:  "user, u",
				Value: "",
				Usage: "Specify user-name to secure the runner",
			},
			cli.StringFlag{
				Name:  "password, p",
				Value: "",
				Usage: "Specify user password to install service (required)",
			})
	} else if os.Getuid() == 0 {
		installFlags = append(installFlags, cli.StringFlag{
			Name:  "user, u",
			Value: "",
			Usage: "Specify user-name to secure the runner",
		})
	}

	return installFlags
}

func init() {
	flags := GetFlags()
	installFlags := GetInstallFlags()

	common.RegisterCommand(cli.Command{
		Name:   "install",
		Usage:  "install service",
		Action: RunServiceControl,
		Flags:  installFlags,
	})
	common.RegisterCommand(cli.Command{
		Name:   "uninstall",
		Usage:  "uninstall service",
		Action: RunServiceControl,
		Flags:  flags,
	})
	common.RegisterCommand(cli.Command{
		Name:   "start",
		Usage:  "start service",
		Action: RunServiceControl,
		Flags:  flags,
	})
	common.RegisterCommand(cli.Command{
		Name:   "stop",
		Usage:  "stop service",
		Action: RunServiceControl,
		Flags:  flags,
	})
	common.RegisterCommand(cli.Command{
		Name:   "restart",
		Usage:  "restart service",
		Action: RunServiceControl,
		Flags:  flags,
	})
	common.RegisterCommand(cli.Command{
		Name:   "status",
		Usage:  "get status of a service",
		Action: RunServiceControl,
		Flags:  flags,
	})
}

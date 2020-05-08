package commands

import (
	"fmt"
	"os"
	"runtime"

	service "github.com/ayufan/golang-kardianos-service"
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

func runServiceStatus(displayName string, s service.Service) error {
	err := s.Status()
	if err == nil {
		fmt.Println(displayName+":", "Service is running!")
	} else {
		fmt.Fprintln(os.Stderr, displayName+":", err)
		os.Exit(1)
	}
	return nil
}

func getServiceArguments(c *cli.Context) (arguments []string) {
	if wd := c.String("working-directory"); wd != "" {
		arguments = append(arguments, "--working-directory", wd)
	}

	if config := c.String("config"); config != "" {
		arguments = append(arguments, "--config", config)
	}

	if sn := c.String("service"); sn != "" {
		arguments = append(arguments, "--service", sn)
	}

	syslog := !c.IsSet("syslog") || c.Bool("syslog")
	if syslog {
		arguments = append(arguments, "--syslog")
	}

	return
}

func createServiceConfig(c *cli.Context) (svcConfig *service.Config) {
	svcConfig = &service.Config{
		Name:        c.String("service"),
		DisplayName: c.String("service"),
		Description: defaultDescription,
		Arguments:   []string{"run"},
	}
	svcConfig.Arguments = append(svcConfig.Arguments, getServiceArguments(c)...)

	switch runtime.GOOS {
	case osTypeLinux:
		if os.Getuid() != 0 {
			logrus.Fatal("Please run the commands as root")
		}
		if user := c.String("user"); user != "" {
			svcConfig.Arguments = append(svcConfig.Arguments, "--user", user)
		}

	case osTypeDarwin:
		svcConfig.Option = service.KeyValue{
			"KeepAlive":   true,
			"RunAtLoad":   true,
			"UserService": os.Getuid() != 0,
		}

		if user := c.String("user"); user != "" {
			if os.Getuid() == 0 {
				svcConfig.Arguments = append(svcConfig.Arguments, "--user", user)
			} else {
				logrus.Fatalln("The --user is not supported for non-root users")
			}
		}

	case osTypeWindows:
		svcConfig.Option = service.KeyValue{
			"Password": c.String("password"),
		}
		svcConfig.UserName = c.String("user")
	}
	return
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
		err = runServiceStatus(svcConfig.DisplayName, s)
	default:
		err = service.Control(s, c.Command.Name)
	}

	if err != nil {
		logrus.Fatal(err)
	}
}

func getFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:  "service, n",
			Value: defaultServiceName,
			Usage: "Specify service name to use",
		},
	}
}

func getInstallFlags() []cli.Flag {
	installFlags := getFlags()
	installFlags = append(installFlags, cli.StringFlag{
		Name:  "working-directory, d",
		Value: helpers.GetCurrentWorkingDirectory(),
		Usage: "Specify custom root directory where all data are stored",
	})
	installFlags = append(installFlags, cli.StringFlag{
		Name:  "config, c",
		Value: getDefaultConfigFile(),
		Usage: "Specify custom config file",
	})
	installFlags = append(installFlags, cli.BoolFlag{
		Name:  "syslog",
		Usage: "Setup system logging integration",
	})

	if runtime.GOOS == osTypeWindows {
		installFlags = append(installFlags, cli.StringFlag{
			Name:  "user, u",
			Value: "",
			Usage: "Specify user-name to secure the runner",
		})
		installFlags = append(installFlags, cli.StringFlag{
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
	flags := getFlags()
	installFlags := getInstallFlags()

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

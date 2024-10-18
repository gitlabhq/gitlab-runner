package commands

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/homedir"
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
	if c.String("user") == "" && c.String("init-user") == "" && os.Getuid() == 0 {
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

func getUserHomeDir(username string) string {
	u, err := user.Lookup(username)
	if err != nil {
		panic(fmt.Sprintf("Failed to get home for user %q: %s", username, err.Error()))
	}
	return u.HomeDir
}

func GetServiceArguments(c *cli.Context) (arguments []string) {
	// Update the default config-file path if it was not actually set and --init-user was specified...
	config := c.String("config")
	if !c.IsSet("config") && c.String("init-user") != "" {
		config = filepath.Join(getUserHomeDir(c.String("init-user")), "config.toml")
	}
	arguments = append(arguments, "--config", config)

	applyStrArg(c, "working-directory", false, func(val string) { arguments = append(arguments, "--working-directory", val) })
	applyStrArg(c, "service", false, func(val string) { arguments = append(arguments, "--service", val) })

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
	if c.String("user") != "" && c.String("init-user") != "" {
		logrus.Fatal("Only one of 'user' or 'init-user' can be specified.")
	}

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
			Value: homedir.GetWDOrEmpty(),
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
		installFlags = append(installFlags,
			cli.StringFlag{
				Name:  "user, u",
				Value: "",
				Usage: "Specify user-name to secure the runner",
			},
			cli.StringFlag{
				Name:  "init-user, i",
				Value: "",
				Usage: "Specify user-name to secure the runner in the init script or systemd unit file",
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

// applyStrArg applies the named string-typed runtime argument to the service configuration in whatever way the `apply`
// function dictates.
func applyStrArg(c *cli.Context, argname string, rootonly bool, apply func(val string)) {
	argval := c.String(argname)
	if argval == "" {
		return
	}

	if rootonly && os.Getuid() != 0 {
		logrus.Fatalf("The --%s is not supported for non-root users", argname)
	}

	apply(argval)
}

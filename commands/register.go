package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

type configTemplate struct {
	*common.Config

	ConfigFile string `long:"config" env:"TEMPLATE_CONFIG_FILE" description:"Path to the configuration template file"`
}

func (c *configTemplate) Enabled() bool {
	return c.ConfigFile != ""
}

func (c *configTemplate) MergeTo(config *common.RunnerConfig) error {
	err := c.loadConfigTemplate()
	if err != nil {
		return errors.Wrap(err, "couldn't load configuration template file")
	}

	if len(c.Runners) != 1 {
		return errors.New("configuration template must contain exactly one [[runners]] entry")
	}

	err = mergo.Merge(config, c.Runners[0])
	if err != nil {
		return errors.Wrap(err, "error while merging configuration with configuration template")
	}

	return nil
}

func (c *configTemplate) loadConfigTemplate() error {
	config := common.NewConfig()

	err := config.LoadConfig(c.ConfigFile)
	if err != nil {
		return err
	}

	c.Config = config

	return nil
}

type RegisterCommand struct {
	context    *cli.Context
	network    common.Network
	reader     *bufio.Reader
	registered bool

	configOptions

	ConfigTemplate configTemplate `namespace:"template"`

	TagList           string `long:"tag-list" env:"RUNNER_TAG_LIST" description:"Tag list"`
	NonInteractive    bool   `short:"n" long:"non-interactive" env:"REGISTER_NON_INTERACTIVE" description:"Run registration unattended"`
	LeaveRunner       bool   `long:"leave-runner" env:"REGISTER_LEAVE_RUNNER" description:"Don't remove runner if registration fails"`
	RegistrationToken string `short:"r" long:"registration-token" env:"REGISTRATION_TOKEN" description:"Runner's registration token"`
	RunUntagged       bool   `long:"run-untagged" env:"REGISTER_RUN_UNTAGGED" description:"Register to run untagged builds; defaults to 'true' when 'tag-list' is empty"`
	Locked            bool   `long:"locked" env:"REGISTER_LOCKED" description:"Lock Runner for current project, defaults to 'true'"`
	AccessLevel       string `long:"access-level" env:"REGISTER_ACCESS_LEVEL" description:"Set access_level of the runner to not_protected or ref_protected; defaults to not_protected"`
	MaximumTimeout    int    `long:"maximum-timeout" env:"REGISTER_MAXIMUM_TIMEOUT" description:"What is the maximum timeout (in seconds) that will be set for job when using this Runner"`
	Paused            bool   `long:"paused" env:"REGISTER_PAUSED" description:"Set Runner to be paused, defaults to 'false'"`

	// TODO: Remove in 13.0 https://gitlab.com/gitlab-org/gitlab-runner/issues/6404
	DockerServices []string `long:"docker-services" json:"docker-services" env:"DOCKER_SERVICES" description:"DEPRECATED will be remove in 13.0: Add service that is started with container from main register command"`

	common.RunnerConfig
}

type AccessLevel string

const (
	NotProtected AccessLevel = "not_protected"
	RefProtected AccessLevel = "ref_protected"
)

const (
	defaultDockerWindowCacheDir = "c:\\cache"
)

func (s *RegisterCommand) askOnce(prompt string, result *string, allowEmpty bool) bool {
	println(prompt)
	if *result != "" {
		print("["+*result, "]: ")
	}

	if s.reader == nil {
		s.reader = bufio.NewReader(os.Stdin)
	}

	data, _, err := s.reader.ReadLine()
	if err != nil {
		panic(err)
	}
	newResult := string(data)
	newResult = strings.TrimSpace(newResult)

	if newResult != "" {
		*result = newResult
		return true
	}

	if allowEmpty || *result != "" {
		return true
	}
	return false
}

func (s *RegisterCommand) ask(key, prompt string, allowEmptyOptional ...bool) string {
	allowEmpty := len(allowEmptyOptional) > 0 && allowEmptyOptional[0]

	result := s.context.String(key)
	result = strings.TrimSpace(result)

	if s.NonInteractive || prompt == "" {
		if result == "" && !allowEmpty {
			logrus.Panicln("The", key, "needs to be entered")
		}
		return result
	}

	for {
		if s.askOnce(prompt, &result, allowEmpty) {
			break
		}
	}

	return result
}

func (s *RegisterCommand) askExecutor() {
	for {
		names := common.GetExecutorNames()
		executors := strings.Join(names, ", ")
		s.Executor = s.ask("executor", "Please enter the executor: "+executors+":", true)
		if common.GetExecutorProvider(s.Executor) != nil {
			return
		}

		message := "Invalid executor specified"
		if s.NonInteractive {
			logrus.Panicln(message)
		} else {
			logrus.Errorln(message)
		}
	}
}

func (s *RegisterCommand) askDocker() {
	s.askBasicDocker("ruby:2.6")

	for _, volume := range s.Docker.Volumes {
		parts := strings.Split(volume, ":")
		if parts[len(parts)-1] == "/cache" {
			return
		}
	}
	s.Docker.Volumes = append(s.Docker.Volumes, "/cache")
}

func (s *RegisterCommand) askDockerWindows() {
	s.askBasicDocker("mcr.microsoft.com/windows/servercore:1809")

	for _, volume := range s.Docker.Volumes {
		// This does not cover all the possibilities since we don't have access
		// to volume parsing package since it's internal.
		if strings.Contains(volume, defaultDockerWindowCacheDir) {
			return
		}
	}
	s.Docker.Volumes = append(s.Docker.Volumes, defaultDockerWindowCacheDir)
}

func (s *RegisterCommand) askBasicDocker(exampleHelperImage string) {
	if s.Docker == nil {
		s.Docker = &common.DockerConfig{}
	}

	s.Docker.Image = s.ask("docker-image", fmt.Sprintf("Please enter the default Docker image (e.g. %s):", exampleHelperImage))
}

func (s *RegisterCommand) askParallels() {
	s.Parallels.BaseName = s.ask("parallels-base-name", "Please enter the Parallels VM (e.g. my-vm):")
}

func (s *RegisterCommand) askVirtualBox() {
	s.VirtualBox.BaseName = s.ask("virtualbox-base-name", "Please enter the VirtualBox VM (e.g. my-vm):")
}

func (s *RegisterCommand) askSSHServer() {
	s.SSH.Host = s.ask("ssh-host", "Please enter the SSH server address (e.g. my.server.com):")
	s.SSH.Port = s.ask("ssh-port", "Please enter the SSH server port (e.g. 22):", true)
}

func (s *RegisterCommand) askSSHLogin() {
	s.SSH.User = s.ask("ssh-user", "Please enter the SSH user (e.g. root):")
	s.SSH.Password = s.ask("ssh-password", "Please enter the SSH password (e.g. docker.io):", true)
	s.SSH.IdentityFile = s.ask("ssh-identity-file", "Please enter path to SSH identity file (e.g. /home/user/.ssh/id_rsa):", true)
}

func (s *RegisterCommand) addRunner(runner *common.RunnerConfig) {
	s.config.Runners = append(s.config.Runners, runner)
}

func (s *RegisterCommand) askRunner() {
	s.URL = s.ask("url", "Please enter the gitlab-ci coordinator URL (e.g. https://gitlab.com/):")

	if s.Token != "" {
		logrus.Infoln("Token specified trying to verify runner...")
		logrus.Warningln("If you want to register use the '-r' instead of '-t'.")
		if !s.network.VerifyRunner(s.RunnerCredentials) {
			logrus.Panicln("Failed to verify this runner. Perhaps you are having network problems")
		}
	} else {
		// we store registration token as token, since we pass that to RunnerCredentials
		s.Token = s.ask("registration-token", "Please enter the gitlab-ci token for this runner:")
		s.Name = s.ask("name", "Please enter the gitlab-ci description for this runner:")
		s.TagList = s.ask("tag-list", "Please enter the gitlab-ci tags for this runner (comma separated):", true)

		if s.TagList == "" {
			s.RunUntagged = true
		}

		parameters := common.RegisterRunnerParameters{
			Description:    s.Name,
			Tags:           s.TagList,
			Locked:         s.Locked,
			AccessLevel:    s.AccessLevel,
			RunUntagged:    s.RunUntagged,
			MaximumTimeout: s.MaximumTimeout,
			Active:         !s.Paused,
		}

		result := s.network.RegisterRunner(s.RunnerCredentials, parameters)
		if result == nil {
			logrus.Panicln("Failed to register this runner. Perhaps you are having network problems")
		}

		s.Token = result.Token
		s.registered = true
	}
}

func (s *RegisterCommand) askExecutorOptions() {
	kubernetes := s.Kubernetes
	machine := s.Machine
	docker := s.Docker
	ssh := s.SSH
	parallels := s.Parallels
	virtualbox := s.VirtualBox
	custom := s.Custom

	s.Kubernetes = nil
	s.Machine = nil
	s.Docker = nil
	s.SSH = nil
	s.Parallels = nil
	s.VirtualBox = nil
	s.Custom = nil
	s.Referees = nil

	executorFns := map[string]func(){
		"kubernetes": func() {
			s.Kubernetes = kubernetes
		},
		"docker+machine": func() {
			s.Machine = machine
			s.Docker = docker
			s.askDocker()
		},
		"docker-ssh+machine": func() {
			s.Machine = machine
			s.Docker = docker
			s.SSH = ssh
			s.askDocker()
			s.askSSHLogin()
		},
		"docker": func() {
			s.Docker = docker
			s.askDocker()
		},
		"docker-windows": func() {
			s.Docker = docker
			s.askDockerWindows()
		},
		"docker-ssh": func() {
			s.Docker = docker
			s.SSH = ssh
			s.askDocker()
			s.askSSHLogin()
		},
		"ssh": func() {
			s.SSH = ssh
			s.askSSHServer()
			s.askSSHLogin()
		},
		"parallels": func() {
			s.SSH = ssh
			s.Parallels = parallels
			s.askParallels()
			s.askSSHServer()
		},
		"virtualbox": func() {
			s.SSH = ssh
			s.VirtualBox = virtualbox
			s.askVirtualBox()
			s.askSSHLogin()
		},
		"shell": func() {
			if runtime.GOOS == "windows" && s.RunnerConfig.Shell == "" {
				s.Shell = "powershell"
			}
		},
		"custom": func() {
			s.Custom = custom
		},
	}

	executorFn, ok := executorFns[s.Executor]
	if ok {
		executorFn()
	}
}

func (s *RegisterCommand) Execute(context *cli.Context) {
	userModeWarning(true)

	s.context = context
	err := s.loadConfig()
	if err != nil {
		logrus.Panicln(err)
	}

	validAccessLevels := []AccessLevel{NotProtected, RefProtected}
	if !accessLevelValid(validAccessLevels, AccessLevel(s.AccessLevel)) {
		logrus.Panicln("Given access-level is not valid. " +
			"Please refer to gitlab-runner register -h for the correct options.")
	}

	s.askRunner()

	if !s.LeaveRunner {
		defer func() {
			// De-register runner on panic
			if r := recover(); r != nil {
				if s.registered {
					s.network.UnregisterRunner(s.RunnerCredentials)
				}

				// pass panic to next defer
				panic(r)
			}
		}()

		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt)

		go func() {
			signal := <-signals
			s.network.UnregisterRunner(s.RunnerCredentials)
			logrus.Fatalf("RECEIVED SIGNAL: %v", signal)
		}()
	}

	if s.config.Concurrent < s.Limit {
		logrus.Warningf("Specified limit (%d) larger then current concurrent limit (%d). Concurrent limit will not be enlarged.", s.Limit, s.config.Concurrent)
	}

	s.askExecutor()
	s.askExecutorOptions()

	s.transformDockerServices(s.DockerServices)

	s.mergeTemplate()

	s.addRunner(&s.RunnerConfig)
	err = s.saveConfig()
	if err != nil {
		logrus.Panicln(err)
	}

	logrus.Printf("Runner registered successfully. Feel free to start it, but if it's running already the config should be automatically reloaded!")
}

// TODO: Remove in 13.0 https://gitlab.com/gitlab-org/gitlab-runner/issues/6404
//
// transformDockerServices will take the value from `DockerServices`
// and convert the value of each entry into a `common.DockerService` definition.
//
// This is to keep backward compatibility when the user passes
// `--docker-services alpine:3.11 --docker-services ruby:3.10` we parse this
// correctly and create the service definition.
func (s *RegisterCommand) transformDockerServices(services []string) {
	for _, service := range services {
		s.Docker.Services = append(
			s.Docker.Services,
			&common.DockerService{
				Service: common.Service{Name: service},
			},
		)
	}
}

func (s *RegisterCommand) mergeTemplate() {
	if !s.ConfigTemplate.Enabled() {
		return
	}

	logrus.Infof("Merging configuration from template file %q", s.ConfigTemplate.ConfigFile)

	err := s.ConfigTemplate.MergeTo(&s.RunnerConfig)
	if err != nil {
		logrus.WithError(err).Fatal("Could not handle configuration merging from template file")
	}
}

func getHostname() string {
	hostname, _ := os.Hostname()
	return hostname
}

func newRegisterCommand() *RegisterCommand {
	return &RegisterCommand{
		RunnerConfig: common.RunnerConfig{
			Name: getHostname(),
			RunnerSettings: common.RunnerSettings{
				Kubernetes: &common.KubernetesConfig{},
				Cache:      &common.CacheConfig{},
				Machine:    &common.DockerMachine{},
				Docker:     &common.DockerConfig{},
				SSH:        &ssh.Config{},
				Parallels:  &common.ParallelsConfig{},
				VirtualBox: &common.VirtualBoxConfig{},
			},
		},
		Locked:  true,
		Paused:  false,
		network: network.NewGitLabClient(),
	}
}

func accessLevelValid(levels []AccessLevel, givenLevel AccessLevel) bool {
	if givenLevel == "" {
		return true
	}

	for _, level := range levels {
		if givenLevel == level {
			return true
		}
	}

	return false
}

func init() {
	common.RegisterCommand2("register", "register a new runner", newRegisterCommand())
}

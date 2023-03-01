package commands

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
	"gitlab.com/gitlab-org/gitlab-runner/network"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
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
		return fmt.Errorf("couldn't load configuration template file: %w", err)
	}

	if len(c.Runners) != 1 {
		return errors.New("configuration template must contain exactly one [[runners]] entry")
	}

	c.Runners[0].Token = ""
	err = mergo.Merge(config, c.Runners[0])
	if err != nil {
		return fmt.Errorf("error while merging configuration with configuration template: %w", err)
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

//nolint:lll
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
	MaintenanceNote   string `long:"maintenance-note" env:"REGISTER_MAINTENANCE_NOTE" description:"Runner's maintenance note"`

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
		s.Executor = s.ask("executor", "Enter an executor: "+executors+":", true)
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
	s.askBasicDocker("ruby:2.7")

	for _, volume := range s.Docker.Volumes {
		parts := strings.Split(volume, ":")
		if parts[len(parts)-1] == "/cache" {
			return
		}
	}
	if !s.Docker.DisableCache {
		s.Docker.Volumes = append(s.Docker.Volumes, "/cache")
	}
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

	s.Docker.Image = s.ask(
		"docker-image",
		fmt.Sprintf("Enter the default Docker image (for example, %s):", exampleHelperImage),
	)
}

func (s *RegisterCommand) askParallels() {
	s.Parallels.BaseName = s.ask("parallels-base-name", "Enter the Parallels VM (for example, my-vm):")
}

func (s *RegisterCommand) askVirtualBox() {
	s.VirtualBox.BaseName = s.ask("virtualbox-base-name", "Enter the VirtualBox VM (for example, my-vm):")
}

func (s *RegisterCommand) askSSHServer() {
	s.SSH.Host = s.ask("ssh-host", "Enter the SSH server address (for example, my.server.com):")
	s.SSH.Port = s.ask("ssh-port", "Enter the SSH server port (for example, 22):", true)
}

func (s *RegisterCommand) askSSHLogin() {
	s.SSH.User = s.ask("ssh-user", "Enter the SSH user (for example, root):")
	s.SSH.Password = s.ask(
		"ssh-password",
		"Enter the SSH password (for example, docker.io):",
		true,
	)
	s.SSH.IdentityFile = s.ask(
		"ssh-identity-file",
		"Enter the path to the SSH identity file (for example, /home/user/.ssh/id_rsa):",
		true,
	)
}

func (s *RegisterCommand) addRunner(runner *common.RunnerConfig) {
	s.configMutex.Lock()
	defer s.configMutex.Unlock()

	s.config.Runners = append(s.config.Runners, runner)
}

func (s *RegisterCommand) askRunner() {
	s.URL = s.ask("url", "Enter the GitLab instance URL (for example, https://gitlab.com/):")

	if s.Token != "" && !s.tokenIsRunnerToken() {
		logrus.Infoln("Token specified trying to verify runner...")
		logrus.Warningln("If you want to register use the '-r' instead of '-t'.")
		if s.network.VerifyRunner(s.RunnerCredentials, s.SystemIDState.GetSystemID()) == nil {
			logrus.Panicln("Failed to verify the runner. You may be having network problems.")
		}
		return
	}

	s.Token = s.ask("registration-token", "Enter the registration token:")
	s.Name = s.ask("name", "Enter a description for the runner:")
	if !s.tokenIsRunnerToken() {
		s.doLegacyRegisterRunner()
		return
	}

	// when a runner token is specified as a registration token, certain arguments are reserved to the server
	s.ensureServerConfigArgsEmpty()

	// If a runner token is specified in place of a registration token, let's accept it and process it as an
	// authentication token. This allows for an easier transition for users by simply replacing the
	// registration token with the new authentication token.
	result := s.network.VerifyRunner(s.RunnerCredentials, s.SystemIDState.GetSystemID())
	if result == nil || result.ID == 0 {
		logrus.Panicln("Failed to verify the runner.")
	}
	s.ID = result.ID
	s.TokenObtainedAt = time.Now().UTC().Truncate(time.Second)
	s.TokenExpiresAt = result.TokenExpiresAt
	s.registered = true
}

func (s *RegisterCommand) doLegacyRegisterRunner() {
	s.TagList = s.ask("tag-list", "Enter tags for the runner (comma-separated):", true)
	s.MaintenanceNote = s.ask("maintenance-note", "Enter optional maintenance note for the runner:", true)

	if s.TagList == "" {
		s.RunUntagged = true
	}

	parameters := common.RegisterRunnerParameters{
		Description:     s.Name,
		MaintenanceNote: s.MaintenanceNote,
		Tags:            s.TagList,
		Locked:          s.Locked,
		AccessLevel:     s.AccessLevel,
		RunUntagged:     s.RunUntagged,
		MaximumTimeout:  s.MaximumTimeout,
		Paused:          s.Paused,
	}

	if s.Token != "" {
		logrus.Warningf(
			"Support for registration tokens and runner parameters in the 'register' command has been deprecated in " +
				"GitLab Runner 15.6 and will be replaced with support for authentication tokens. " +
				"For more information, see https://gitlab.com/gitlab-org/gitlab/-/issues/380872",
		)
	}

	result := s.network.RegisterRunner(s.RunnerCredentials, parameters)
	// golangci-lint doesn't recognize logrus.Panicln() call as breaking the execution
	// flow which causes the following assignment to throw false-positive report for
	// 'SA5011: possible nil pointer dereference'
	// nolint:staticcheck
	if result == nil {
		logrus.Panicln("Failed to register the runner.")
	}

	s.ID = result.ID
	s.Token = result.Token
	s.TokenObtainedAt = time.Now().UTC().Truncate(time.Second)
	s.TokenExpiresAt = result.TokenExpiresAt
	s.registered = true
}

//nolint:funlen
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
			if s.RunnerConfig.Shell == "" {
				s.Shell = shells.SNPwsh
			}

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
			if runtime.GOOS == osTypeWindows && s.RunnerConfig.Shell == "" {
				s.Shell = shells.SNPwsh
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
	s.SystemIDState = s.loadedSystemIDState

	validAccessLevels := []AccessLevel{NotProtected, RefProtected}
	if !accessLevelValid(validAccessLevels, AccessLevel(s.AccessLevel)) {
		logrus.Panicln("Given access-level is not valid. " +
			"Refer to gitlab-runner register -h for the correct options.")
	}

	s.mergeTemplate()

	s.askRunner()

	if !s.LeaveRunner {
		defer s.unregisterRunner()()
	}

	config := s.getConfig()
	if config.Concurrent < s.Limit {
		logrus.Warningf(
			"The specified runner job concurrency limit (%d) is larger than current global concurrency limit (%d). "+
				"The global concurrent limit will not be increased and takes precedence.",
			s.Limit,
			config.Concurrent,
		)
	}
	if config.Concurrent < s.RequestConcurrency {
		logrus.Warningf(
			"The specified runner request concurrency (%d) is larger than the current global concurrent limit (%d). "+
				"The global concurrent limit will not be increased and takes precedence.",
			s.RequestConcurrency,
			config.Concurrent,
		)
	}

	s.askExecutor()
	s.askExecutorOptions()

	s.addRunner(&s.RunnerConfig)
	err = s.saveConfig()
	if err != nil {
		logrus.Panicln(err)
	}

	logrus.Printf(
		"Runner registered successfully. " +
			"Feel free to start it, but if it's running already the config should be automatically reloaded!\n")
	logrus.Printf("Configuration (with the authentication token) was saved in %q", s.ConfigFile)
}

func (s *RegisterCommand) unregisterRunner() func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	go func() {
		signal := <-signals
		s.network.UnregisterRunner(s.RunnerCredentials)
		logrus.Fatalf("RECEIVED SIGNAL: %v", signal)
	}()

	return func() {
		// De-register runner on panic
		if r := recover(); r != nil {
			if s.registered {
				s.network.UnregisterRunner(s.RunnerCredentials)
			}

			// pass panic to next defer
			panic(r)
		}
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

func (s *RegisterCommand) tokenIsRunnerToken() bool {
	return strings.HasPrefix(s.Token, "glrt-")
}

func (s *RegisterCommand) ensureServerConfigArgsEmpty() {
	if s.Locked && s.AccessLevel == "" && !s.RunUntagged && s.MaximumTimeout == 0 && !s.Paused &&
		s.TagList == "" && s.MaintenanceNote == "" {
		return
	}

	logrus.Fatalln(
		"Runner configuration other than name, description, and executor configuration is reserved " +
			"and cannot be specified when registering with a runner token. " +
			"This configuration is specified on the GitLab server. Please try again without specifying " +
			"those arguments.",
	)
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

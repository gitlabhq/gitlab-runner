package commands

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"dario.cat/mergo"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/internal/configfile"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/network"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
	"gitlab.com/gitlab-org/labkit/fips"

	// Force to load shell executor, executes init() on them
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/custom"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/parallels"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/shell"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/ssh"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/virtualbox"
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

type RegisterCommand struct {
	context    *cli.Context
	network    common.Network
	reader     *bufio.Reader
	registered bool
	timeNowFn  func() time.Time

	ConfigTemplate configTemplate `namespace:"template"`

	ConfigFile        string `short:"c" long:"config" env:"CONFIG_FILE" description:"Config file"`
	TagList           string `long:"tag-list" env:"RUNNER_TAG_LIST" description:"Tag list"`
	NonInteractive    bool   `short:"n" long:"non-interactive" env:"REGISTER_NON_INTERACTIVE" description:"Run registration unattended"`
	LeaveRunner       bool   `long:"leave-runner" env:"REGISTER_LEAVE_RUNNER" description:"Don't remove runner if registration fails"`
	RegistrationToken string `short:"r" long:"registration-token" env:"REGISTRATION_TOKEN" description:"Runner's registration token (deprecated, use --token)"`
	RunUntagged       bool   `long:"run-untagged" env:"REGISTER_RUN_UNTAGGED" description:"Register to run untagged builds; defaults to 'true' when 'tag-list' is empty"`
	Locked            bool   `long:"locked" env:"REGISTER_LOCKED" description:"Lock Runner for current project, defaults to 'true'"`
	AccessLevel       string `long:"access-level" env:"REGISTER_ACCESS_LEVEL" description:"Set access_level of the runner to not_protected or ref_protected; defaults to not_protected"`
	MaximumTimeout    int    `long:"maximum-timeout" env:"REGISTER_MAXIMUM_TIMEOUT" description:"What is the maximum timeout (in seconds) that will be set for job when using this Runner"`
	Paused            bool   `long:"paused" env:"REGISTER_PAUSED" description:"Set Runner to be paused, defaults to 'false'"`
	MaintenanceNote   string `long:"maintenance-note" env:"REGISTER_MAINTENANCE_NOTE" description:"Runner's maintenance note"`

	common.RunnerConfig
}

func NewRegisterCommand(n common.Network) cli.Command {
	return common.NewCommand("register", "register a new runner", newRegisterCommand(n))
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
	if err == io.EOF && !s.NonInteractive {
		logrus.Panicln("Unexpected EOF. Did you mean to use --non-interactive?")
	}
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

	for !s.askOnce(prompt, &result, allowEmpty) {
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
	s.askBasicDocker("ruby:3.3")

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

func (s *RegisterCommand) verifyRunner() {
	// If a runner authentication token is specified in place of a registration token, let's accept it and process it as
	// an authentication token. This allows for an easier transition for users by simply replacing the
	// registration token with the new authentication token.
	result := s.network.VerifyRunner(s.RunnerConfig, s.SystemID)
	if result == nil || result.ID == 0 {
		logrus.Panicln("Failed to verify the runner.")
	}
	s.ID = result.ID
	s.TokenObtainedAt = s.timeNowFn().UTC().Truncate(time.Second)
	s.TokenExpiresAt = result.TokenExpiresAt
	s.registered = true
}

func (s *RegisterCommand) askRunner(cfg *common.Config) {
	s.URL = s.ask("url", "Enter the GitLab instance URL (for example, https://gitlab.com/):")

	if s.Token != "" && !s.tokenIsRunnerToken() {
		logrus.Infoln("Token specified trying to verify runner...")
		logrus.Warningln("If you want to register use the '-r' instead of '-t'.")
		if s.network.VerifyRunner(s.RunnerConfig, s.SystemID) == nil {
			logrus.Panicln("Failed to verify the runner. You may be having network problems.")
		}
		return
	}

	if s.Token == "" || !s.tokenIsRunnerToken() {
		s.Token = s.ask("registration-token", "Enter the registration token:")
	}

	if !s.tokenIsRunnerToken() {
		s.Name = s.ask("name", "Enter a description for the runner:")
		s.doLegacyRegisterRunner()
		return
	}

	if r, err := cfg.RunnerByToken(s.Token); err == nil && r != nil {
		logrus.Warningln("A runner with this system ID and token has already been registered.")
	}

	// when a runner authentication token is specified as a registration token, certain arguments are reserved to the server
	s.ensureServerConfigArgsEmpty()

	s.verifyRunner()
	s.Name = s.ask("name", "Enter a name for the runner. This is stored only in the local config.toml file:")
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
				"For more information, see https://docs.gitlab.com/ci/runners/new_creation_workflow/",
		)
	}

	result := s.network.RegisterRunner(s.RunnerConfig, parameters)
	// golangci-lint doesn't recognize logrus.Panicln() call as breaking the execution
	// flow which causes the following assignment to throw false-positive report for
	// 'SA5011: possible nil pointer dereference'
	//nolint:staticcheck
	if result == nil {
		logrus.Panicln("Failed to register the runner.")
	}

	s.ID = result.ID
	s.Token = result.Token
	s.TokenObtainedAt = s.timeNowFn().UTC().Truncate(time.Second)
	s.TokenExpiresAt = result.TokenExpiresAt
	s.registered = true
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
		"docker": func() {
			s.Docker = docker
			s.askDocker()
		},
		"docker-autoscaler": func() {
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

// Set helper_image_flavor to ubi-fips if fips is enabled. See
// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38273
func setFipsHelperImageFlavor(cfg *common.RunnerConfig, fipsEnabled func() bool) {
	if cfg == nil || !fipsEnabled() {
		return
	}
	if cfg.Docker != nil && cfg.Docker.HelperImageFlavor == "" {
		cfg.Docker.HelperImageFlavor = "ubi-fips"
	}
	if cfg.Kubernetes != nil && cfg.Kubernetes.HelperImageFlavor == "" {
		cfg.Kubernetes.HelperImageFlavor = "ubi-fips"
	}
}

func (s *RegisterCommand) Execute(context *cli.Context) {
	userModeWarning(true)

	s.context = context
	validAccessLevels := []AccessLevel{NotProtected, RefProtected}
	if !accessLevelValid(validAccessLevels, AccessLevel(s.AccessLevel)) {
		logrus.Panicln("Given access-level is not valid. " +
			"Refer to gitlab-runner register -h for the correct options.")
	}

	s.mergeTemplate()

	cfg := configfile.New(s.ConfigFile)
	if err := cfg.Load(configfile.WithMutateOnLoad(func(config *common.Config) error {
		s.SystemID = cfg.SystemID()
		s.askRunner(config)

		if !s.LeaveRunner {
			defer s.unregisterRunnerFunc()()
		}

		s.askExecutor()
		s.askExecutorOptions()

		setFipsHelperImageFlavor(&s.RunnerConfig, fips.Enabled)

		config.Runners = append(config.Runners, &s.RunnerConfig)
		return nil
	})); err != nil {
		logrus.Panicln(err)
	}

	if err := cfg.Save(); err != nil {
		logrus.Panicln(err)
	}

	config := cfg.Config()
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

	logrus.Printf(
		"Runner registered successfully. " +
			"Feel free to start it, but if it's running already the config should be automatically reloaded!\n")
	logrus.Printf("Configuration (with the authentication token) was saved in %q", s.ConfigFile)
}

func (s *RegisterCommand) unregisterRunnerFunc() func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	go func() {
		signal := <-signals
		s.unregisterRunner()
		logrus.Fatalf("RECEIVED SIGNAL: %v", signal)
	}()

	return func() {
		// De-register runner on panic
		if r := recover(); r != nil {
			if s.registered {
				s.unregisterRunner()
			}

			// pass panic to next defer
			panic(r)
		}
	}
}

func (s *RegisterCommand) unregisterRunner() {
	if s.tokenIsRunnerToken() {
		s.network.UnregisterRunnerManager(s.RunnerConfig, s.SystemID)
	} else {
		s.network.UnregisterRunner(s.RunnerConfig)
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
	return network.TokenIsCreatedRunnerToken(s.Token)
}

func (s *RegisterCommand) ensureServerConfigArgsEmpty() {
	if s.Locked && s.AccessLevel == "" && !s.RunUntagged && s.MaximumTimeout == 0 && !s.Paused &&
		s.TagList == "" && s.MaintenanceNote == "" {
		return
	}

	if s.RegistrationToken == s.Token {
		logrus.Warningln(
			"You have specified an authentication token in the legacy parameter --registration-token. " +
				"This has triggered the 'legacy-compatible registration process' which has resulted in the " +
				"following command line parameters being ignored: --locked, --access-level, --run-untagged, " +
				"--maximum-timeout, --paused, --tag-list, and --maintenance-note. " +
				"For more information, see https://docs.gitlab.com/ci/runners/new_creation_workflow/#changes-to-the-gitlab-runner-register-command-syntax" +
				"These parameters and the legacy-compatible registration process will be removed " +
				"in a future GitLab Runner release. ",
		)
		return
	}

	logrus.Fatalln(
		"Runner configuration other than name and executor configuration is reserved (specifically --locked, " +
			"--access-level, --run-untagged, --maximum-timeout, --paused, --tag-list, and --maintenance-note) " +
			"and cannot be specified when registering with a runner authentication token. " +
			"This configuration is specified on the GitLab server. " +
			"Please try again without specifying any of those arguments. " +
			"For more information, see https://docs.gitlab.com/ci/runners/new_creation_workflow/#changes-to-the-gitlab-runner-register-command-syntax",
	)
}

func getHostname() string {
	hostname, _ := os.Hostname()
	return hostname
}

func newRegisterCommand(n common.Network) *RegisterCommand {
	return &RegisterCommand{
		RunnerConfig: common.RunnerConfig{
			Name: getHostname(),
			RunnerSettings: common.RunnerSettings{
				Kubernetes: &common.KubernetesConfig{},
				Cache:      &common.CacheConfig{},
				Machine:    &common.DockerMachine{},
				Docker:     &common.DockerConfig{},
				SSH:        &common.SshConfig{},
				Parallels:  &common.ParallelsConfig{},
				VirtualBox: &common.VirtualBoxConfig{},
			},
		},
		Locked:    true,
		Paused:    false,
		network:   n,
		timeNowFn: time.Now,
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

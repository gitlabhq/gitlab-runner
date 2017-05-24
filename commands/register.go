package commands

import (
	"bufio"
	"os"
	"os/signal"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers/ssh"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/network"
)

type RegisterCommand struct {
	context    *cli.Context
	network    common.Network
	reader     *bufio.Reader
	registered bool

	configOptions
	TagList           string `long:"tag-list" env:"RUNNER_TAG_LIST" description:"Tag list"`
	NonInteractive    bool   `short:"n" long:"non-interactive" env:"REGISTER_NON_INTERACTIVE" description:"Run registration unattended"`
	LeaveRunner       bool   `long:"leave-runner" env:"REGISTER_LEAVE_RUNNER" description:"Don't remove runner if registration fails"`
	RegistrationToken string `short:"r" long:"registration-token" env:"REGISTRATION_TOKEN" description:"Runner's registration token"`
	RunUntagged       bool   `long:"run-untagged" env:"REGISTER_RUN_UNTAGGED" description:"Register to run untagged builds; defaults to 'true' when 'tag-list' is empty"`
	Locked            bool   `long:"locked" env:"REGISTER_LOCKED" description:"Lock Runner for current project, defaults to 'false'"`

	common.RunnerConfig
}

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
			log.Panicln("The", key, "needs to be entered")
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
		names := common.GetExecutors()
		executors := strings.Join(names, ", ")
		s.Executor = s.ask("executor", "Please enter the executor: "+executors+":", true)
		if common.GetExecutor(s.Executor) != nil {
			return
		}

		message := "Invalid executor specified"
		if s.NonInteractive {
			log.Panicln(message)
		} else {
			log.Errorln(message)
		}
	}
}

func (s *RegisterCommand) askDocker() {
	if s.Docker == nil {
		s.Docker = &common.DockerConfig{}
	}
	s.Docker.Image = s.ask("docker-image", "Please enter the default Docker image (e.g. ruby:2.1):")
	s.Docker.Volumes = append(s.Docker.Volumes, "/cache")
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
		log.Infoln("Token specified trying to verify runner...")
		log.Warningln("If you want to register use the '-r' instead of '-t'.")
		if !s.network.VerifyRunner(s.RunnerCredentials) {
			log.Panicln("Failed to verify this runner. Perhaps you are having network problems")
		}
	} else {
		// we store registration token as token, since we pass that to RunnerCredentials
		s.Token = s.ask("registration-token", "Please enter the gitlab-ci token for this runner:")
		s.Name = s.ask("name", "Please enter the gitlab-ci description for this runner:")
		s.TagList = s.ask("tag-list", "Please enter the gitlab-ci tags for this runner (comma separated):", true)

		if s.TagList == "" {
			s.RunUntagged = true
		} else {
			runUntagged, err := strconv.ParseBool(s.ask("run-untagged", "Whether to run untagged builds [true/false]:", true))
			if err != nil {
				log.Panicf("Failed to parse option 'run-untagged': %v", err)
			} else {
				s.RunUntagged = runUntagged
			}
		}

		locked, err := strconv.ParseBool(s.ask("locked", "Whether to lock Runner to current project [true/false]:", false))
		if err != nil {
			log.Panicf("Failed to parse option 'locked': %v", err)
		}

		result := s.network.RegisterRunner(s.RunnerCredentials, s.Name, s.TagList, s.RunUntagged, locked)
		if result == nil {
			log.Panicln("Failed to register this runner.")
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

	s.Kubernetes = nil
	s.Machine = nil
	s.Docker = nil
	s.SSH = nil
	s.Parallels = nil
	s.VirtualBox = nil

	switch s.Executor {
	case "kubernetes":
		s.Kubernetes = kubernetes
	case "docker+machine":
		s.Machine = machine
		s.Docker = docker
		s.askDocker()
	case "docker-ssh+machine":
		s.Machine = machine
		s.Docker = docker
		s.SSH = ssh
		s.askDocker()
		s.askSSHLogin()
	case "docker":
		s.Docker = docker
		s.askDocker()
	case "docker-ssh":
		s.Docker = docker
		s.SSH = ssh
		s.askDocker()
		s.askSSHLogin()
	case "ssh":
		s.SSH = ssh
		s.askSSHServer()
		s.askSSHLogin()
	case "parallels":
		s.SSH = ssh
		s.Parallels = parallels
		s.askParallels()
		s.askSSHServer()
	case "virtualbox":
		s.SSH = ssh
		s.VirtualBox = virtualbox
		s.askVirtualBox()
		s.askSSHLogin()
	}
}

func (s *RegisterCommand) Execute(context *cli.Context) {
	userModeWarning(true)

	s.context = context
	err := s.loadConfig()
	if err != nil {
		log.Panicln(err)
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
			log.Fatalf("RECEIVED SIGNAL: %v", signal)
		}()
	}

	if s.config.Concurrent < s.Limit {
		log.Warningf("Specified limit (%d) larger then current concurrent limit (%d). Concurrent limit will not be enlarged.", s.Limit, s.config.Concurrent)
	}

	s.askExecutor()
	s.askExecutorOptions()
	s.addRunner(&s.RunnerConfig)
	s.saveConfig()

	log.Printf("Runner registered successfully. Feel free to start it, but if it's running already the config should be automatically reloaded!")
}

func getHostname() string {
	hostname, _ := os.Hostname()
	return hostname
}

func init() {
	common.RegisterCommand2("register", "register a new runner", &RegisterCommand{
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
		network: network.NewGitLabClient(),
	})
}

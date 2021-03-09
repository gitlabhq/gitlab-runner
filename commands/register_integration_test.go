package commands_test

import (
	"bufio"
	"runtime"
	"strings"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"

	clihelpers "gitlab.com/ayufan/golang-cli-helpers"
	"gitlab.com/gitlab-org/gitlab-runner/commands"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker/machine"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
)

const osTypeWindows = "windows"

func TestAskRunnerOverrideDefaultsForExecutors(t *testing.T) {
	executors := []string{
		"kubernetes",
		"docker+machine",
		"docker-ssh+machine",
		"docker",
		"docker-ssh",
		"ssh",
		"custom",
		"parallels",
		"virtualbox",
		"shell",
	}
	if runtime.GOOS == osTypeWindows {
		executors = append(executors, "docker-windows")
	}

	for _, executor := range executors {
		t.Run(executor, func(t *testing.T) { testAskRunnerOverrideDefaultsForExecutor(t, executor) })
	}
}

func testAskRunnerOverrideDefaultsForExecutor(t *testing.T, executor string) {
	basicValidation := func(s *commands.RegisterCommand) {
		assertExecutorDefaultValues(t, executor, s)
	}

	tests := map[string]struct {
		answers        []string
		arguments      []string
		validate       func(s *commands.RegisterCommand)
		expectedParams func(common.RegisterRunnerParameters) bool
	}{
		"basic answers": {
			answers: append([]string{
				"http://gitlab.example.com/",
				"test-registration-token",
				"name",
				"tag,list",
			}, executorAnswers(t, executor)...),
			validate: basicValidation,
			expectedParams: func(p common.RegisterRunnerParameters) bool {
				return p == common.RegisterRunnerParameters{
					Description: "name",
					Tags:        "tag,list",
					Locked:      true,
					Active:      true,
				}
			},
		},
		"basic arguments, accepting provided": {
			answers: make([]string, 10),
			arguments: append(
				executorCmdLineArgs(t, executor),
				"--url", "http://gitlab.example.com/",
				"-r", "test-registration-token",
				"--name", "name",
				"--tag-list", "tag,list",
				"--paused",
				"--locked=false",
			),
			validate: basicValidation,
			expectedParams: func(p common.RegisterRunnerParameters) bool {
				return p == common.RegisterRunnerParameters{
					Description: "name",
					Tags:        "tag,list",
				}
			},
		},
		"basic arguments override": {
			answers: append([]string{"", "", "new-name", "", ""}, executorOverrideAnswers(t, executor)...),
			arguments: append(
				executorCmdLineArgs(t, executor),
				"--url", "http://gitlab.example.com/",
				"-r", "test-registration-token",
				"--name", "name",
				"--tag-list", "tag,list",
				"--paused",
				"--locked=false",
			),
			validate: func(s *commands.RegisterCommand) {
				assertExecutorOverridenValues(t, executor, s)
			},
			expectedParams: func(p common.RegisterRunnerParameters) bool {
				return p == common.RegisterRunnerParameters{
					Description: "new-name",
					Tags:        "tag,list",
				}
			},
		},
		"untagged implicit": {
			answers: append([]string{
				"http://gitlab.example.com/",
				"test-registration-token",
				"name",
				"",
			}, executorAnswers(t, executor)...),
			validate: basicValidation,
			expectedParams: func(p common.RegisterRunnerParameters) bool {
				return p == common.RegisterRunnerParameters{
					Description: "name",
					RunUntagged: true,
					Locked:      true,
					Active:      true,
				}
			},
		},
		"untagged explicit": {
			answers: append([]string{
				"http://gitlab.example.com/",
				"test-registration-token",
				"name",
				"",
			}, executorAnswers(t, executor)...),
			arguments: []string{"--run-untagged"},
			validate:  basicValidation,
			expectedParams: func(p common.RegisterRunnerParameters) bool {
				return p == common.RegisterRunnerParameters{
					Description: "name",
					RunUntagged: true,
					Locked:      true,
					Active:      true,
				}
			},
		},
		"untagged explicit with tags provided": {
			answers: append([]string{
				"http://gitlab.example.com/",
				"test-registration-token",
				"name",
				"tag,list",
			}, executorAnswers(t, executor)...),
			arguments: []string{"--run-untagged"},
			validate:  basicValidation,
			expectedParams: func(p common.RegisterRunnerParameters) bool {
				return p == common.RegisterRunnerParameters{
					Description: "name",
					Tags:        "tag,list",
					RunUntagged: true,
					Locked:      true,
					Active:      true,
				}
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			network := new(common.MockNetwork)
			defer network.AssertExpectations(t)

			network.On("RegisterRunner", mock.Anything, mock.MatchedBy(tc.expectedParams)).
				Return(&common.RegisterRunnerResponse{
					Token: "test-runner-token",
				}).
				Once()

			cmd := commands.NewRegisterCommandForTest(
				bufio.NewReader(strings.NewReader(strings.Join(tc.answers, "\n")+"\n")),
				network,
			)

			app := cli.NewApp()
			app.Commands = []cli.Command{
				{
					Name:   "register",
					Action: cmd.Execute,
					Flags:  clihelpers.GetFlagsFromStruct(cmd),
				},
			}

			hook := test.NewGlobal()
			err := app.Run(append([]string{"runner", "register"}, tc.arguments...))
			output := commands.GetLogrusOutput(t, hook)

			assert.NoError(t, err)
			tc.validate(cmd)
			assert.Contains(t, output, "Runner registered successfully.")
		})
	}
}

func assertExecutorDefaultValues(t *testing.T, executor string, s *commands.RegisterCommand) {
	assert.Equal(t, "http://gitlab.example.com/", s.URL)
	assert.Equal(t, "test-runner-token", s.Token)
	assert.Equal(t, executor, s.RunnerSettings.Executor)

	switch executor {
	case "kubernetes":
		assert.NotNil(t, s.RunnerSettings.Kubernetes)
	case "custom":
		assert.NotNil(t, s.RunnerSettings.Custom)
	case "shell":
		assert.NotNil(t, s.RunnerSettings.Shell)
		if runtime.GOOS == osTypeWindows && s.RunnerConfig.Shell == "" {
			assert.Equal(t, "powershell", s.RunnerSettings.Shell)
		}
	case "docker":
		require.NotNil(t, s.RunnerSettings.Docker)
		assert.Equal(t, "busybox:latest", s.RunnerSettings.Docker.Image)
	case "docker-windows":
		require.NotNil(t, s.RunnerSettings.Docker)
		assert.Equal(t, "mcr.microsoft.com/windows/servercore:1809", s.RunnerSettings.Docker.Image)
	case "docker+machine":
		assert.NotNil(t, s.RunnerSettings.Machine)
		require.NotNil(t, s.RunnerSettings.Docker)
		assert.Equal(t, "busybox:latest", s.RunnerSettings.Docker.Image)
	case "docker-ssh":
		assertDefaultSSHLogin(t, s.RunnerSettings.SSH)
		require.NotNil(t, s.RunnerSettings.Docker)
		assert.Equal(t, "busybox:latest", s.RunnerSettings.Docker.Image)
	case "docker-ssh+machine":
		assert.NotNil(t, s.RunnerSettings.Machine)
		assertDefaultSSHLogin(t, s.RunnerSettings.SSH)
		require.NotNil(t, s.RunnerSettings.Docker)
		assert.Equal(t, "busybox:latest", s.RunnerSettings.Docker.Image)
	case "ssh":
		assertDefaultSSHLogin(t, s.RunnerSettings.SSH)
		assertDefaultSSHServer(t, s.RunnerSettings.SSH)
	case "parallels":
		assertDefaultSSHServer(t, s.RunnerSettings.SSH)
		require.NotNil(t, s.RunnerSettings.Parallels)
		assert.Equal(t, executor+"-vm-name", s.RunnerSettings.Parallels.BaseName)
	case "virtualbox":
		assertDefaultSSHLogin(t, s.RunnerSettings.SSH)
		require.NotNil(t, s.RunnerSettings.VirtualBox)
		assert.Equal(t, executor+"-vm-name", s.RunnerSettings.VirtualBox.BaseName)
	default:
		assert.FailNow(t, "no assertions found for executor", executor)
	}
}

func assertDefaultSSHLogin(t *testing.T, sshCfg *ssh.Config) {
	require.NotNil(t, sshCfg)
	assert.Equal(t, "user", sshCfg.User)
	assert.Equal(t, "password", sshCfg.Password)
	assert.Equal(t, "/home/user/.ssh/id_rsa", sshCfg.IdentityFile)
}

func assertDefaultSSHServer(t *testing.T, sshCfg *ssh.Config) {
	require.NotNil(t, sshCfg)
	assert.Equal(t, "gitlab.example.com", sshCfg.Host)
	assert.Equal(t, "22", sshCfg.Port)
}

func assertExecutorOverridenValues(t *testing.T, executor string, s *commands.RegisterCommand) {
	assert.Equal(t, "http://gitlab.example.com/", s.URL)
	assert.Equal(t, "test-runner-token", s.Token)
	assert.Equal(t, executor, s.RunnerSettings.Executor)

	switch executor {
	case "kubernetes":
		assert.NotNil(t, s.RunnerSettings.Kubernetes)
	case "custom":
		assert.NotNil(t, s.RunnerSettings.Custom)
	case "shell":
		assert.NotNil(t, s.RunnerSettings.Shell)
		if runtime.GOOS == osTypeWindows && s.RunnerConfig.Shell == "" {
			assert.Equal(t, "powershell", s.RunnerSettings.Shell)
		}
	case "docker":
		require.NotNil(t, s.RunnerSettings.Docker)
		assert.Equal(t, "nginx:latest", s.RunnerSettings.Docker.Image)
	case "docker-windows":
		require.NotNil(t, s.RunnerSettings.Docker)
		assert.Equal(t, "mcr.microsoft.com/windows/servercore:1903", s.RunnerSettings.Docker.Image)
	case "docker+machine":
		assert.NotNil(t, s.RunnerSettings.Machine)
		require.NotNil(t, s.RunnerSettings.Docker)
		assert.Equal(t, "nginx:latest", s.RunnerSettings.Docker.Image)
	case "docker-ssh":
		assertOverridenSSHLogin(t, s.RunnerSettings.SSH)
		require.NotNil(t, s.RunnerSettings.Docker)
		assert.Equal(t, "nginx:latest", s.RunnerSettings.Docker.Image)
	case "docker-ssh+machine":
		assert.NotNil(t, s.Machine)
		assertOverridenSSHLogin(t, s.RunnerSettings.SSH)
		require.NotNil(t, s.RunnerSettings.Docker)
		assert.Equal(t, "nginx:latest", s.RunnerSettings.Docker.Image)
	case "ssh":
		assertOverridenSSHLogin(t, s.RunnerSettings.SSH)
		assertOverridenSSHServer(t, s.RunnerSettings.SSH)
	case "parallels":
		assertOverridenSSHServer(t, s.RunnerSettings.SSH)
		require.NotNil(t, s.RunnerSettings.Parallels)
		assert.Equal(t, "override-"+executor+"-vm-name", s.RunnerSettings.Parallels.BaseName)
	case "virtualbox":
		assertOverridenSSHLogin(t, s.RunnerSettings.SSH)
		require.NotNil(t, s.RunnerSettings.VirtualBox)
		assert.Equal(t, "override-"+executor+"-vm-name", s.RunnerSettings.VirtualBox.BaseName)
	default:
		assert.FailNow(t, "no assertions found for executor", executor)
	}
}

func assertOverridenSSHLogin(t *testing.T, sshCfg *ssh.Config) {
	require.NotNil(t, sshCfg)
	assert.Equal(t, "root", sshCfg.User)
	assert.Equal(t, "admin", sshCfg.Password)
	assert.Equal(t, "/root/.ssh/id_rsa", sshCfg.IdentityFile)
}

func assertOverridenSSHServer(t *testing.T, sshCfg *ssh.Config) {
	require.NotNil(t, sshCfg)
	assert.Equal(t, "ssh.gitlab.example.com", sshCfg.Host)
	assert.Equal(t, "8822", sshCfg.Port)
}

func executorAnswers(t *testing.T, executor string) []string {
	values := map[string][]string{
		"kubernetes":         {executor},
		"custom":             {executor},
		"shell":              {executor},
		"docker":             {executor, "busybox:latest"},
		"docker-windows":     {executor, "mcr.microsoft.com/windows/servercore:1809"},
		"docker+machine":     {executor, "busybox:latest"},
		"docker-ssh":         {executor, "busybox:latest", "user", "password", "/home/user/.ssh/id_rsa"},
		"docker-ssh+machine": {executor, "busybox:latest", "user", "password", "/home/user/.ssh/id_rsa"},
		"ssh":                {executor, "gitlab.example.com", "22", "user", "password", "/home/user/.ssh/id_rsa"},
		"parallels":          {executor, "parallels-vm-name", "gitlab.example.com", "22"},
		"virtualbox":         {executor, "virtualbox-vm-name", "user", "password", "/home/user/.ssh/id_rsa"},
	}

	answers, ok := values[executor]
	if !ok {
		assert.FailNow(t, "No answers found for executor", executor)
	}
	return answers
}

func executorOverrideAnswers(t *testing.T, executor string) []string {
	values := map[string][]string{
		"kubernetes":         {""},
		"custom":             {""},
		"shell":              {""},
		"docker":             {"nginx:latest"},
		"docker-windows":     {"mcr.microsoft.com/windows/servercore:1903"},
		"docker+machine":     {"nginx:latest"},
		"docker-ssh":         {"nginx:latest", "root", "admin", "/root/.ssh/id_rsa"},
		"docker-ssh+machine": {"nginx:latest", "root", "admin", "/root/.ssh/id_rsa"},
		"ssh":                {"ssh.gitlab.example.com", "8822", "root", "admin", "/root/.ssh/id_rsa"},
		"parallels":          {"override-parallels-vm-name", "ssh.gitlab.example.com", "8822"},
		"virtualbox":         {"override-virtualbox-vm-name", "root", "admin", "/root/.ssh/id_rsa"},
	}

	answers, ok := values[executor]
	if !ok {
		assert.FailNow(t, "No override answers found for executor", executor)
	}
	return answers
}

func executorCmdLineArgs(t *testing.T, executor string) []string {
	values := map[string][]string{
		"kubernetes":     {"--executor", executor},
		"custom":         {"--executor", executor},
		"shell":          {"--executor", executor},
		"docker":         {"--executor", executor, "--docker-image", "busybox:latest"},
		"docker-windows": {"--executor", executor, "--docker-image", "mcr.microsoft.com/windows/servercore:1809"},
		"docker+machine": {"--executor", executor, "--docker-image", "busybox:latest"},
		"docker-ssh": {
			"--executor", executor, "--docker-image", "busybox:latest", "--ssh-user", "user",
			"--ssh-password", "password",
			"--ssh-identity-file", "/home/user/.ssh/id_rsa",
		},
		"docker-ssh+machine": {
			"--executor", executor, "--docker-image", "busybox:latest", "--ssh-user", "user",
			"--ssh-password", "password",
			"--ssh-identity-file", "/home/user/.ssh/id_rsa",
		},
		"ssh": {
			"--executor", executor, "--ssh-host", "gitlab.example.com", "--ssh-port", "22", "--ssh-user", "user",
			"--ssh-password", "password", "--ssh-identity-file", "/home/user/.ssh/id_rsa",
		},
		"parallels": {
			"--executor", executor, "--ssh-host", "gitlab.example.com", "--ssh-port", "22",
			"--parallels-base-name", "parallels-vm-name",
		},
		"virtualbox": {
			"--executor", executor, "--ssh-host", "gitlab.example.com", "--ssh-user", "user",
			"--ssh-password", "password", "--ssh-identity-file", "/home/user/.ssh/id_rsa",
			"--virtualbox-base-name", "virtualbox-vm-name",
		},
	}

	args, ok := values[executor]
	if !ok {
		assert.FailNow(t, "No command line args found for executor", executor)
	}
	return args
}

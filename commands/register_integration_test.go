//go:build integration
// +build integration

package commands_test

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker/machine"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
	clihelpers "gitlab.com/gitlab-org/golang-cli-helpers"
)

const osTypeWindows = "windows"

var spaceReplacer = strings.NewReplacer(" ", "", "\t", "")

type kv struct {
	key, value string
}

func TestAccessLevelSetting(t *testing.T) {
	tests := map[string]struct {
		accessLevel     commands.AccessLevel
		failureExpected bool
	}{
		"access level not defined": {},
		"ref_protected used": {
			accessLevel: commands.RefProtected,
		},
		"not_protected used": {
			accessLevel: commands.NotProtected,
		},
		"unknown access level": {
			accessLevel:     commands.AccessLevel("unknown"),
			failureExpected: true,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			network := new(common.MockNetwork)
			defer network.AssertExpectations(t)

			if !testCase.failureExpected {
				parametersMocker := mock.MatchedBy(func(parameters common.RegisterRunnerParameters) bool {
					return commands.AccessLevel(parameters.AccessLevel) == testCase.accessLevel
				})

				network.On("RegisterRunner", mock.Anything, parametersMocker).
					Return(&common.RegisterRunnerResponse{
						Token: "test-runner-token",
					}).
					Once()
			}

			arguments := []string{
				"--access-level", string(testCase.accessLevel),
			}

			_, output, err := testRegisterCommandRun(t, network, nil, arguments...)

			if testCase.failureExpected {
				assert.EqualError(t, err, "command error: Given access-level is not valid. "+
					"Refer to gitlab-runner register -h for the correct options.")
				assert.NotContains(t, output, "Runner registered successfully.")

				return
			}

			assert.NoError(t, err)
			assert.Contains(t, output, "Runner registered successfully.")
		})
	}
}

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

func testRegisterCommandRun(
	t *testing.T,
	network common.Network,
	env []kv,
	args ...string,
) (content, output string, err error) {
	config := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-runner-token",
		},
		RunnerSettings: common.RunnerSettings{
			Executor: "shell",
		},
	}

	for _, kv := range env {
		err := os.Setenv(kv.key, kv.value)
		if err != nil {
			return "", "", err
		}
	}

	defer func() {
		for _, kv := range env {
			_ = os.Unsetenv(kv.key)
		}
	}()

	hook := test.NewGlobal()

	defer func() {
		output = commands.GetLogrusOutput(t, hook)

		if r := recover(); r != nil {
			// log panics forces exit
			if e, ok := r.(*logrus.Entry); ok {
				err = fmt.Errorf("command error: %s", e.Message)
			}
		}
	}()

	cmd := commands.NewRegisterCommandForTest(nil, network)

	app := cli.NewApp()
	app.Commands = []cli.Command{
		{
			Name:   "register",
			Action: cmd.Execute,
			Flags:  clihelpers.GetFlagsFromStruct(cmd),
		},
	}

	configFile, err := ioutil.TempFile("", "config.toml")
	require.NoError(t, err)

	err = configFile.Close()
	require.NoError(t, err)

	defer os.Remove(configFile.Name())

	args = append([]string{
		"binary", "register",
		"-n",
		"--config", configFile.Name(),
		"--url", "http://gitlab.example.com/",
		"--registration-token", "test-registration-token",
	}, args...)
	if !contains(args, "--executor") {
		args = append(args, "--executor", config.RunnerSettings.Executor)
	}

	commandErr := app.Run(args)

	fileContent, err := ioutil.ReadFile(configFile.Name())
	require.NoError(t, err)

	err = commandErr

	return string(fileContent), "", err
}

func contains(args []string, s string) bool {
	for _, arg := range args {
		if arg == s {
			return true
		}
	}
	return false
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
				"basic notes",
			}, executorAnswers(t, executor)...),
			validate: basicValidation,
			expectedParams: func(p common.RegisterRunnerParameters) bool {
				return p == common.RegisterRunnerParameters{
					Description:     "name",
					MaintenanceNote: "basic notes",
					Tags:            "tag,list",
					Locked:          true,
					Active:          true,
				}
			},
		},
		"basic arguments, accepting provided": {
			answers: make([]string, 11),
			arguments: append(
				executorCmdLineArgs(t, executor),
				"--url", "http://gitlab.example.com/",
				"-r", "test-registration-token",
				"--name", "name",
				"--tag-list", "tag,list",
				"--maintenance-note", "maintainer notes",
				"--paused",
				"--locked=false",
			),
			validate: basicValidation,
			expectedParams: func(p common.RegisterRunnerParameters) bool {
				return p == common.RegisterRunnerParameters{
					Description:     "name",
					MaintenanceNote: "maintainer notes",
					Tags:            "tag,list",
				}
			},
		},
		"basic arguments override": {
			answers: append([]string{"", "", "new-name", "", "maintainer notes", ""}, executorOverrideAnswers(t, executor)...),
			arguments: append(
				executorCmdLineArgs(t, executor),
				"--url", "http://gitlab.example.com/",
				"-r", "test-registration-token",
				"--name", "name",
				"--maintenance-note", "notes",
				"--tag-list", "tag,list",
				"--paused",
				"--locked=false",
			),
			validate: func(s *commands.RegisterCommand) {
				assertExecutorOverridenValues(t, executor, s)
			},
			expectedParams: func(p common.RegisterRunnerParameters) bool {
				return p == common.RegisterRunnerParameters{
					Description:     "new-name",
					MaintenanceNote: "maintainer notes",
					Tags:            "tag,list",
				}
			},
		},
		"untagged implicit": {
			answers: append([]string{
				"http://gitlab.example.com/",
				"test-registration-token",
				"name",
				"",
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
				"",
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
		assert.Equal(t, "mcr.microsoft.com/windows/servercore:YYH1", s.RunnerSettings.Docker.Image)
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
		assert.Equal(t, "mcr.microsoft.com/windows/servercore:YYH2", s.RunnerSettings.Docker.Image)
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
		"docker-windows":     {executor, "mcr.microsoft.com/windows/servercore:YYH1"},
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
		"docker-windows":     {"mcr.microsoft.com/windows/servercore:YYH2"},
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
		"docker-windows": {"--executor", executor, "--docker-image", "mcr.microsoft.com/windows/servercore:YYH1"},
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

func TestExecute_MergeConfigTemplate(t *testing.T) {
	var (
		configTemplateMergeInvalidConfiguration = `- , ;`

		configTemplateMergeAdditionalConfiguration = `
[[runners]]
  [runners.kubernetes]
    [runners.kubernetes.volumes]
      [[runners.kubernetes.volumes.empty_dir]]
        name = "empty_dir"
	    mount_path = "/path/to/empty_dir"
	    medium = "Memory"`

		baseOutputConfigFmt = `concurrent = 1
check_interval = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = %q
  url = "http://gitlab.example.com/"
  token = "test-runner-token"
  executor = "shell"
  shell = "pwsh"
  [runners.custom_build_dir]
  [runners.cache]
    [runners.cache.s3]
    [runners.cache.gcs]
    [runners.cache.azure]
`
	)

	tests := map[string]struct {
		configTemplate         string
		networkAssertions      func(n *common.MockNetwork)
		errExpected            bool
		expectedFileContentFmt string
	}{
		"config template disabled": {
			configTemplate: "",
			networkAssertions: func(n *common.MockNetwork) {
				n.On("RegisterRunner", mock.Anything, mock.Anything).
					Return(&common.RegisterRunnerResponse{
						Token: "test-runner-token",
					}).
					Once()
			},
			errExpected:            false,
			expectedFileContentFmt: baseOutputConfigFmt,
		},
		"config template with no additional runner configuration": {
			configTemplate: "[[runners]]",
			networkAssertions: func(n *common.MockNetwork) {
				n.On("RegisterRunner", mock.Anything, mock.Anything).
					Return(&common.RegisterRunnerResponse{
						Token: "test-runner-token",
					}).
					Once()
			},
			errExpected:            false,
			expectedFileContentFmt: baseOutputConfigFmt,
		},
		"successful config template merge": {
			configTemplate: configTemplateMergeAdditionalConfiguration,
			networkAssertions: func(n *common.MockNetwork) {
				n.On("RegisterRunner", mock.Anything, mock.Anything).
					Return(&common.RegisterRunnerResponse{
						Token: "test-runner-token",
					}).
					Once()
			},
			errExpected: false,
			expectedFileContentFmt: `concurrent = 1
check_interval = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = %q
  url = "http://gitlab.example.com/"
  token = "test-runner-token"
  executor = "shell"
  shell = "pwsh"
  [runners.custom_build_dir]
  [runners.cache]
    [runners.cache.s3]
    [runners.cache.gcs]
    [runners.cache.azure]
`,
		},
		"incorrect config template merge": {
			configTemplate:    configTemplateMergeInvalidConfiguration,
			networkAssertions: func(n *common.MockNetwork) {},
			errExpected:       true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			var err error

			if tt.errExpected {
				helpers.MakeFatalToPanic()
			}

			cfgTpl, cleanup := commands.PrepareConfigurationTemplateFile(t, tt.configTemplate)
			defer cleanup()

			network := new(common.MockNetwork)
			defer network.AssertExpectations(t)

			args := []string{
				"--shell", shells.SNPwsh,
			}

			if tt.configTemplate != "" {
				args = append(args, "--template-config", cfgTpl)
			}

			tt.networkAssertions(network)

			fileContent, _, err := testRegisterCommandRun(t, network, nil, args...)
			if tt.errExpected {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			name, err := os.Hostname()
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf(tt.expectedFileContentFmt, name), fileContent)
		})
	}
}

func TestUnregisterOnFailure(t *testing.T) {
	tests := map[string]struct {
		leaveRunner           bool
		registrationFails     bool
		expectsLeftRegistered bool
	}{
		"registration succeeds, runner left registered": {
			leaveRunner:           false,
			registrationFails:     false,
			expectsLeftRegistered: true,
		},
		"registration fails, LeaveRunner is false, runner is unregistered": {
			leaveRunner:           false,
			registrationFails:     true,
			expectsLeftRegistered: false,
		},
		"registration fails, LeaveRunner is true, runner left registered": {
			leaveRunner:           true,
			registrationFails:     true,
			expectsLeftRegistered: true,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			network := new(common.MockNetwork)
			defer network.AssertExpectations(t)

			const token = "test-runner-token"
			network.On("RegisterRunner", mock.Anything, mock.Anything).
				Return(&common.RegisterRunnerResponse{
					Token: token,
				}).
				Once()
			if !testCase.expectsLeftRegistered {
				credsMocker := mock.MatchedBy(func(credentials common.RunnerCredentials) bool {
					return credentials.Token == token
				})
				network.On("UnregisterRunner", credsMocker).
					Return(true).
					Once()
			}

			var arguments []string
			if testCase.leaveRunner {
				arguments = append(arguments, "--leave-runner")
			}

			answers := []string{"https://gitlab.com/", token, "description", "", ""}
			if testCase.registrationFails {
				defer func() { _ = recover() }()
			} else {
				answers = append(answers, "custom") // should not result in more answers required
			}
			cmd := commands.NewRegisterCommandForTest(
				bufio.NewReader(strings.NewReader(strings.Join(answers, "\n")+"\n")),
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

			err := app.Run(append([]string{"runner", "register"}, arguments...))

			assert.False(t, testCase.registrationFails)
			assert.NoError(t, err)
		})
	}
}

func TestRegisterCommand(t *testing.T) {
	type testCase struct {
		condition       func() bool
		arguments       []string
		environment     []kv
		expectedConfigs []string
	}

	testCases := map[string]testCase{
		"feature flags are included in config": {
			arguments: []string{
				"--name", "test-runner",
				"--feature-flags", "FF_TEST_1:true",
				"--feature-flags", "FF_TEST_2:false",
			},
			expectedConfigs: []string{`[runners.feature_flags]
		   FF_TEST_1 = true
		   FF_TEST_2 = false`},
		},
		"shell defaults to pwsh on Windows with shell executor": {
			condition: func() bool { return runtime.GOOS == osTypeWindows },
			arguments: []string{
				"--name", "test-runner",
				"--executor", "shell",
			},
			expectedConfigs: []string{`shell = "pwsh"`},
		},
		"shell defaults to pwsh on Windows with docker-windows executor": {
			condition: func() bool { return runtime.GOOS == osTypeWindows },
			arguments: []string{
				"--name", "test-runner",
				"--executor", "docker-windows",
				"--docker-image", "abc",
			},
			expectedConfigs: []string{`shell = "pwsh"`},
		},
		"shell can be overridden to powershell on Windows with shell executor": {
			condition: func() bool { return runtime.GOOS == osTypeWindows },
			arguments: []string{
				"--name", "test-runner",
				"--executor", "shell",
				"--shell", "powershell",
			},
			expectedConfigs: []string{`shell = "powershell"`},
		},
		"shell can be overridden to powershell on Windows with docker-windows executor": {
			condition: func() bool { return runtime.GOOS == osTypeWindows },
			arguments: []string{
				"--name", "test-runner",
				"--executor", "docker-windows",
				"--shell", "powershell",
				"--docker-image", "abc",
			},
			expectedConfigs: []string{`shell = "powershell"`},
		},
		"kubernetes security context namespace": {
			arguments: []string{
				"--executor", "kubernetes",
			},
			environment: []kv{
				{
					key:   "KUBERNETES_BUILD_CONTAINER_SECURITY_CONTEXT_PRIVILEGED",
					value: "true",
				},
				{
					key:   "KUBERNETES_HELPER_CONTAINER_SECURITY_CONTEXT_RUN_AS_USER",
					value: "1000",
				},
				{
					key:   "KUBERNETES_SERVICE_CONTAINER_SECURITY_CONTEXT_RUN_AS_NON_ROOT",
					value: "true",
				},
				{
					key:   "KUBERNETES_SERVICE_CONTAINER_SECURITY_CONTEXT_CAPABILITIES_ADD",
					value: "NET_RAW, NET_RAW1",
				},
			},
			expectedConfigs: []string{`
		[runners.kubernetes.build_container_security_context]
			privileged = true`, `
		[runners.kubernetes.helper_container_security_context]
			run_as_user = 1000`, `
		[runners.kubernetes.service_container_security_context]
			run_as_non_root = true`, `
      	[runners.kubernetes.service_container_security_context.capabilities]
        	add = ["NET_RAW, NET_RAW1"]`,
			},
		},
		"s3 cache AuthenticationType arg": {
			arguments: []string{
				"--cache-s3-authentication_type=iam",
			},
			expectedConfigs: []string{`
		[runners.cache.s3]
			AuthenticationType = "iam"
			`},
		},
		"s3 cache AuthenticationType env": {
			environment: []kv{
				{
					key:   "CACHE_S3_AUTHENTICATION_TYPE",
					value: "iam",
				},
			},
			expectedConfigs: []string{`
		[runners.cache.s3]
			AuthenticationType = "iam"
			`},
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			if tc.condition != nil && !tc.condition() {
				t.Skip()
			}

			network := new(common.MockNetwork)
			defer network.AssertExpectations(t)

			network.On("RegisterRunner", mock.Anything, mock.Anything).
				Return(&common.RegisterRunnerResponse{
					Token: "test-runner-token",
				}).
				Once()

			gotConfig, _, err := testRegisterCommandRun(t, network, tc.environment, tc.arguments...)
			require.NoError(t, err)

			for _, expectedConfig := range tc.expectedConfigs {
				assert.Contains(t, spaceReplacer.Replace(gotConfig), spaceReplacer.Replace(expectedConfig))
			}
		})
	}
}

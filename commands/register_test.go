package commands

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	clihelpers "gitlab.com/ayufan/golang-cli-helpers"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker/machine"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
)

func setupDockerRegisterCommand(dockerConfig *common.DockerConfig) *RegisterCommand {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	ctx := cli.NewContext(cli.NewApp(), fs, nil)
	fs.String("docker-image", "ruby:2.6", "")

	s := &RegisterCommand{
		context:        ctx,
		NonInteractive: true,
	}
	s.Docker = dockerConfig

	return s
}

func TestRegisterDefaultDockerCacheVolume(t *testing.T) {
	s := setupDockerRegisterCommand(&common.DockerConfig{
		Volumes: []string{},
	})

	s.askDocker()

	assert.Equal(t, 1, len(s.Docker.Volumes))
	assert.Equal(t, "/cache", s.Docker.Volumes[0])
}

func TestDoNotRegisterDefaultDockerCacheVolumeWhenDisableCache(t *testing.T) {
	s := setupDockerRegisterCommand(&common.DockerConfig{
		Volumes:      []string{},
		DisableCache: true,
	})

	s.askDocker()

	assert.Len(t, s.Docker.Volumes, 0)
}

func TestRegisterCustomDockerCacheVolume(t *testing.T) {
	s := setupDockerRegisterCommand(&common.DockerConfig{
		Volumes: []string{"/cache"},
	})

	s.askDocker()

	assert.Equal(t, 1, len(s.Docker.Volumes))
	assert.Equal(t, "/cache", s.Docker.Volumes[0])
}

func TestRegisterCustomMappedDockerCacheVolume(t *testing.T) {
	s := setupDockerRegisterCommand(&common.DockerConfig{
		Volumes: []string{"/my/cache:/cache"},
	})

	s.askDocker()

	assert.Equal(t, 1, len(s.Docker.Volumes))
	assert.Equal(t, "/my/cache:/cache", s.Docker.Volumes[0])
}

func getLogrusOutput(t *testing.T, hook *test.Hook) string {
	buf := &bytes.Buffer{}
	for _, entry := range hook.AllEntries() {
		message, err := entry.String()
		require.NoError(t, err)

		buf.WriteString(message)
	}

	return buf.String()
}

func testRegisterCommandRun(
	t *testing.T,
	network common.Network,
	args ...string,
) (content, output string, err error) {
	hook := test.NewGlobal()

	defer func() {
		output = getLogrusOutput(t, hook)

		if r := recover(); r != nil {
			// log panics forces exit
			if e, ok := r.(*logrus.Entry); ok {
				err = fmt.Errorf("command error: %s", e.Message)
			}
		}
	}()

	cmd := newRegisterCommand()
	cmd.network = network

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
		"--executor", "shell",
	}, args...)

	comandErr := app.Run(args)

	fileContent, err := ioutil.ReadFile(configFile.Name())
	require.NoError(t, err)

	err = comandErr

	return string(fileContent), "", err
}

func TestAccessLevelSetting(t *testing.T) {
	tests := map[string]struct {
		accessLevel     AccessLevel
		failureExpected bool
	}{
		"access level not defined": {},
		"ref_protected used": {
			accessLevel: RefProtected,
		},
		"not_protected used": {
			accessLevel: NotProtected,
		},
		"unknown access level": {
			accessLevel:     AccessLevel("unknown"),
			failureExpected: true,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			network := new(common.MockNetwork)
			defer network.AssertExpectations(t)

			if !testCase.failureExpected {
				parametersMocker := mock.MatchedBy(func(parameters common.RegisterRunnerParameters) bool {
					return AccessLevel(parameters.AccessLevel) == testCase.accessLevel
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

			_, output, err := testRegisterCommandRun(t, network, arguments...)

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

func testAskRunnerOverrideDefaultsForExecutor(t *testing.T, executor string) {
	basicValidation := func(s *RegisterCommand) {
		assertExecutorDefaultValues(t, executor, s)
	}

	tests := map[string]struct {
		answers        []string
		arguments      []string
		validate       func(s *RegisterCommand)
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
			validate: func(s *RegisterCommand) {
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

			cmd := newRegisterCommand()
			cmd.reader = bufio.NewReader(strings.NewReader(strings.Join(tc.answers, "\n") + "\n"))
			cmd.network = network

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
			output := getLogrusOutput(t, hook)

			assert.NoError(t, err)
			tc.validate(cmd)
			assert.Contains(t, output, "Runner registered successfully.")
		})
	}
}

func assertExecutorDefaultValues(t *testing.T, executor string, s *RegisterCommand) {
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

func assertExecutorOverridenValues(t *testing.T, executor string, s *RegisterCommand) {
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

func TestConfigTemplate_Enabled(t *testing.T) {
	tests := map[string]struct {
		path          string
		expectedValue bool
	}{
		"configuration file defined": {
			path:          "/path/to/file",
			expectedValue: true,
		},
		"configuration file not defined": {
			path:          "",
			expectedValue: false,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			configTemplate := &configTemplate{ConfigFile: tc.path}
			assert.Equal(t, tc.expectedValue, configTemplate.Enabled())
		})
	}
}

func prepareConfigurationTemplateFile(t *testing.T, content string) (string, func()) {
	file, err := ioutil.TempFile("", "config.template.toml")
	require.NoError(t, err)

	defer func() {
		err = file.Close()
		require.NoError(t, err)
	}()

	_, err = file.WriteString(content)
	require.NoError(t, err)

	cleanup := func() {
		_ = os.Remove(file.Name())
	}

	return file.Name(), cleanup
}

var (
	configTemplateMergeToInvalidConfiguration = `- , ;`

	configTemplateMergeToEmptyConfiguration = ``

	configTemplateMergeToTwoRunnerSectionsConfiguration = `
[[runners]]
[[runners]]`

	configTemplateMergeToOverwritingConfiguration = `
[[runners]]
  token = "different_token"
  executor = "docker"
  limit = 100`

	configTemplateMergeToAdditionalConfiguration = `
[[runners]]
  [runners.kubernetes]
    [runners.kubernetes.volumes]
      [[runners.kubernetes.volumes.empty_dir]]
        name = "empty_dir"
	    mount_path = "/path/to/empty_dir"
	    medium = "Memory"`

	configTemplateMergeToBaseConfiguration = &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "test-runner-token",
		},
		RunnerSettings: common.RunnerSettings{
			Executor: "shell",
		},
	}
)

func TestConfigTemplate_MergeTo(t *testing.T) {
	//nolint:lll
	tests := map[string]struct {
		templateContent string
		config          *common.RunnerConfig

		expectedError       error
		assertConfiguration func(t *testing.T, config *common.RunnerConfig)
	}{
		"invalid template file": {
			templateContent: configTemplateMergeToInvalidConfiguration,
			config:          configTemplateMergeToBaseConfiguration,
			expectedError:   errors.New("couldn't load configuration template file: Near line 1 (last key parsed '-'): expected key separator '=', but got ',' instead"),
		},
		"no runners in template": {
			templateContent: configTemplateMergeToEmptyConfiguration,
			config:          configTemplateMergeToBaseConfiguration,
			expectedError:   errors.New("configuration template must contain exactly one [[runners]] entry"),
		},
		"multiple runners in template": {
			templateContent: configTemplateMergeToTwoRunnerSectionsConfiguration,
			config:          configTemplateMergeToBaseConfiguration,
			expectedError:   errors.New("configuration template must contain exactly one [[runners]] entry"),
		},
		"template doesn't overwrite existing settings": {
			templateContent: configTemplateMergeToOverwritingConfiguration,
			config:          configTemplateMergeToBaseConfiguration,
			assertConfiguration: func(t *testing.T, config *common.RunnerConfig) {
				assert.Equal(t, configTemplateMergeToBaseConfiguration.Token, config.RunnerCredentials.Token)
				assert.Equal(t, configTemplateMergeToBaseConfiguration.Executor, config.RunnerSettings.Executor)
				assert.Equal(t, 100, config.Limit)
			},
			expectedError: nil,
		},
		"template adds additional content": {
			templateContent: configTemplateMergeToAdditionalConfiguration,
			config:          configTemplateMergeToBaseConfiguration,
			assertConfiguration: func(t *testing.T, config *common.RunnerConfig) {
				k8s := config.RunnerSettings.Kubernetes

				require.NotNil(t, k8s)
				require.NotEmpty(t, k8s.Volumes.EmptyDirs)
				assert.Len(t, k8s.Volumes.EmptyDirs, 1)

				emptyDir := k8s.Volumes.EmptyDirs[0]
				assert.Equal(t, "empty_dir", emptyDir.Name)
				assert.Equal(t, "/path/to/empty_dir", emptyDir.MountPath)
				assert.Equal(t, "Memory", emptyDir.Medium)
			},
			expectedError: nil,
		},
		"error on merging": {
			templateContent: configTemplateMergeToAdditionalConfiguration,
			expectedError: errors.Wrap(
				mergo.ErrNotSupported,
				"error while merging configuration with configuration template",
			),
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			file, cleanup := prepareConfigurationTemplateFile(t, tc.templateContent)
			defer cleanup()

			configTemplate := &configTemplate{ConfigFile: file}
			err := configTemplate.MergeTo(tc.config)

			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())

				return
			}

			assert.NoError(t, err)
			tc.assertConfiguration(t, tc.config)
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

			answers := []string{"https://gitlab.com/", token, "description", ""}
			if testCase.registrationFails {
				defer func() { _ = recover() }()
			} else {
				answers = append(answers, "custom") // should not result in more answers required
			}
			cmd := newRegisterCommand()
			cmd.reader = bufio.NewReader(strings.NewReader(strings.Join(answers, "\n") + "\n"))
			cmd.network = network

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

func TestRegisterCommand_FeatureFlag(t *testing.T) {
	expectedConfig := `
  [runners.feature_flags]
    FF_TEST_1 = true
    FF_TEST_2 = false
`

	network := new(common.MockNetwork)
	defer network.AssertExpectations(t)

	network.On("RegisterRunner", mock.Anything, mock.Anything).
		Return(&common.RegisterRunnerResponse{
			Token: "test-runner-token",
		}).
		Once()

	arguments := []string{
		"--name", "test-runner",
		"--feature-flags", "FF_TEST_1:true",
		"--feature-flags", "FF_TEST_2:false",
	}

	gotConfig, _, err := testRegisterCommandRun(t, network, arguments...)
	require.NoError(t, err)

	assert.Contains(t, gotConfig, expectedConfig)
}

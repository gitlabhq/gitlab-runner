package commands

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"gitlab.com/ayufan/golang-cli-helpers"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func setupDockerRegisterCommand(dockerConfig *common.DockerConfig) *RegisterCommand {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	ctx := cli.NewContext(cli.NewApp(), fs, nil)
	fs.String("docker-image", "ruby:2.1", "")

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

func testRegisterCommandRun(t *testing.T, network common.Network, args ...string) (content string, output string, err error) {
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
				assert.EqualError(t, err, "command error: Given access-level is not valid. Please refer to gitlab-runner register -h for the correct options.")
				assert.NotContains(t, output, "Runner registered successfully.")

				return
			}

			assert.NoError(t, err)
			assert.Contains(t, output, "Runner registered successfully.")
		})
	}
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
			expectedError:   errors.Wrap(mergo.ErrNotSupported, "error while merging configuration with configuration template"),
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

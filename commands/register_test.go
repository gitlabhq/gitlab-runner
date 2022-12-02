//go:build !integration

package commands

import (
	"errors"
	"flag"
	"fmt"
	"testing"

	"github.com/imdario/mergo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker/machine"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes"
)

func setupDockerRegisterCommand(dockerConfig *common.DockerConfig) *RegisterCommand {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	ctx := cli.NewContext(cli.NewApp(), fs, nil)
	fs.String("docker-image", "ruby:2.7", "")

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
			expectedError:   errors.New("couldn't load configuration template file: toml: line 1: expected '.' or '=', but got ',' instead"),
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
		"template doesn't overwrite token if none provided in base": {
			templateContent: configTemplateMergeToOverwritingConfiguration,
			config:          &common.RunnerConfig{},
			assertConfiguration: func(t *testing.T, config *common.RunnerConfig) {
				assert.Equal(t, "", config.Token)
			},
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
			expectedError: fmt.Errorf(
				"error while merging configuration with configuration template: %w",
				mergo.ErrNotSupported,
			),
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			file, cleanup := PrepareConfigurationTemplateFile(t, tc.templateContent)
			defer cleanup()

			configTemplate := &configTemplate{ConfigFile: file}
			err := configTemplate.MergeTo(tc.config)

			if tc.expectedError != nil {
				assert.ErrorContains(t, err, tc.expectedError.Error())

				return
			}

			assert.NoError(t, err)
			tc.assertConfiguration(t, tc.config)
		})
	}
}

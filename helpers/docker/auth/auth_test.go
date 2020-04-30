package auth

import (
	"encoding/base64"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	imageRegistryDomain1 = "registry.domain.tld:5005/image/name:version"
	imageRegistryDomain2 = "registry2.domain.tld:5005/image/name:version"
	imageGitlabDomain    = "registry.gitlab.tld:1234/image/name:version"
)

var (
	testFileAuthConfigs       = `{"auths":{"https://registry.domain.tld:5005/v1/":{"auth":"dGVzdF91c2VyXzE6dGVzdF9wYXNzd29yZF8x"},"registry2.domain.tld:5005":{"auth":"dGVzdF91c2VyXzI6dGVzdF9wYXNzd29yZF8y"}}}`
	testVariableAuthConfigs   = `{"auths":{"https://registry.domain.tld:5005/v1/":{"auth":"dGVzdF91c2VyXzE6dGVzdF9wYXNzd29yZF8x"}}}`
	gitlabRegistryCredentials = []common.Credentials{
		{
			Type:     "registry",
			URL:      "registry.gitlab.tld:1234",
			Username: "test_user_3",
			Password: "test_password_3",
		},
	}
	registryDomainConfigVariable = &types.AuthConfig{
		Username:      "test_user",
		Password:      "test_password",
		ServerAddress: "https://registry.domain.tld:5005/v1/",
	}
	registryDomain1Config = &types.AuthConfig{
		Username:      "test_user_1",
		Password:      "test_password_1",
		ServerAddress: "https://registry.domain.tld:5005/v1/",
	}
	registryDomain2Config = &types.AuthConfig{
		Username:      "test_user_2",
		Password:      "test_password_2",
		ServerAddress: "registry2.domain.tld:5005",
	}
	registryGitlabConfig = &types.AuthConfig{
		Username:      "test_user_3",
		Password:      "test_password_3",
		ServerAddress: "registry.gitlab.tld:1234",
	}
	emptyCredentials = []common.Credentials{}
)

func TestGetConfigForImage(t *testing.T) {
	tests := map[string]struct {
		precreateConfigFile bool
		dockerAuthVariable  string
		credentials         []common.Credentials
		image               string
		expectedSource      string
		expectedConf        *types.AuthConfig
	}{
		"registry1 from file only": {
			true,
			"",
			emptyCredentials,
			imageRegistryDomain1,
			".dockercfg",
			registryDomain1Config,
		},
		"registry2 from file only": {
			true,
			"",
			emptyCredentials,
			imageRegistryDomain2,
			".dockercfg",
			registryDomain2Config,
		},
		"missing credentials, file only": {
			true,
			"",
			emptyCredentials,
			imageGitlabDomain,
			"",
			nil,
		},
		"no file and gitlab credentials. image in gitlab credentials": {
			false,
			"",
			gitlabRegistryCredentials,
			imageGitlabDomain,
			authConfigSourceNameJobPayload,
			registryGitlabConfig,
		},
		"both file and gitlab credentials. image in gitlab credentials": {
			true,
			"",
			gitlabRegistryCredentials,
			imageGitlabDomain,
			authConfigSourceNameJobPayload,
			registryGitlabConfig,
		},
		"DOCKER_AUTH_CONFIG only": {
			false,
			testVariableAuthConfigs,
			emptyCredentials,
			imageRegistryDomain1,
			authConfigSourceNameUserVariable,
			registryDomain1Config,
		},
		"DOCKER_AUTH_CONFIG overrides home dir": {
			true,
			testVariableAuthConfigs,
			emptyCredentials,
			imageRegistryDomain1,
			authConfigSourceNameUserVariable,
			registryDomain1Config,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			setupTestHomeDirectoryConfig(t, tt.precreateConfigFile)

			source, config := GetConfigForImage(tt.image, tt.dockerAuthVariable, "", tt.credentials)

			assert.Contains(t, source, tt.expectedSource)
			assert.Equal(t, tt.expectedConf, config)
		})
	}
}

func TestGetConfigs(t *testing.T) {
	setupTestHomeDirectoryConfig(t, true)
	sources, configs := GetConfigs(testVariableAuthConfigs, "", gitlabRegistryCredentials)
	assert.Len(t, sources, 3)
	assert.Equal(t, authConfigSourceNameUserVariable, sources["https://registry.domain.tld:5005/v1/"])
	assert.Equal(t, authConfigSourceNameJobPayload, sources["registry.gitlab.tld:1234"])
	assert.Contains(t, sources["registry2.domain.tld:5005"], ".dockercfg")

	assert.Equal(t, map[string]types.AuthConfig{
		"https://registry.domain.tld:5005/v1/": {
			Username:      "test_user_1",
			Password:      "test_password_1",
			ServerAddress: "https://registry.domain.tld:5005/v1/",
		},
		"registry.gitlab.tld:1234": {
			Username:      "test_user_3",
			Password:      "test_password_3",
			ServerAddress: "registry.gitlab.tld:1234",
		},
		"registry2.domain.tld:5005": {
			Username:      "test_user_2",
			Password:      "test_password_2",
			ServerAddress: "registry2.domain.tld:5005",
		},
	}, configs)
}

func setupTestHomeDirectoryConfig(t *testing.T, precreateConfigFile bool) {
	if precreateConfigFile {
		tempHomeDir, err := ioutil.TempDir("", "docker-auth-configs-test")
		require.NoError(t, err)
		dockerConfigFile := path.Join(tempHomeDir, ".dockercfg")
		err = ioutil.WriteFile(dockerConfigFile, []byte(testFileAuthConfigs), 0600)
		require.NoError(t, err)
		HomeDirectory = tempHomeDir
	} else {
		HomeDirectory = ""
	}
}

func TestSplitDockerImageName(t *testing.T) {
	remote, image := SplitDockerImageName("tutum.co/user/ubuntu")
	expectedRemote := "tutum.co"
	expectedImage := "user/ubuntu"

	if remote != expectedRemote {
		t.Error("Expected ", expectedRemote, ", got ", remote)
	}

	if image != expectedImage {
		t.Error("Expected ", expectedImage, ", got ", image)
	}
}

func TestSplitDefaultDockerImageName(t *testing.T) {
	remote, image := SplitDockerImageName("user/ubuntu")
	expectedRemote := "docker.io"
	expectedImage := "user/ubuntu"

	if remote != expectedRemote {
		t.Error("Expected ", expectedRemote, ", got ", remote)
	}

	if image != expectedImage {
		t.Error("Expected ", expectedImage, ", got ", image)
	}
}

func TestSplitDefaultIndexDockerImageName(t *testing.T) {
	remote, image := SplitDockerImageName("index.docker.io/user/ubuntu")
	expectedRemote := "docker.io"
	expectedImage := "user/ubuntu"

	if remote != expectedRemote {
		t.Error("Expected ", expectedRemote, ", got ", remote)
	}

	if image != expectedImage {
		t.Error("Expected ", expectedImage, ", got ", image)
	}
}

type configLocation struct {
	subfolder string
	filename  string
}

func TestReadDockerAuthConfigsFromHomeDir_NoUsername(t *testing.T) {
	expectedUsername := "test_username"
	expectedPassword := "test_password"
	expectedServerAddr := "https://index.docker.io/v1/"

	auth := base64.StdEncoding.EncodeToString([]byte(expectedUsername + ":" + expectedPassword))

	config := []byte(`{"auths": {"` + expectedServerAddr + `": {"auth": "` + auth + `"}}}`)

	tests := map[string]struct {
		homeDirProvided     bool
		configContent       []byte
		configLocation      configLocation
		expectedAuthConfigs map[string]types.AuthConfig
		expectedError       error
	}{
		"Home dir value is blank": {
			homeDirProvided:     false,
			expectedAuthConfigs: nil,
			expectedError:       errNoHomeDir,
		},
		"No configs": {
			homeDirProvided:     true,
			configLocation:      configLocation{},
			expectedAuthConfigs: map[string]types.AuthConfig{},
			expectedError:       nil,
		},
		"Config: $HOME/.dockercfg": {
			homeDirProvided: true,
			configContent:   config,
			configLocation: configLocation{
				subfolder: "",
				filename:  ".dockercfg",
			},
			expectedAuthConfigs: map[string]types.AuthConfig{
				expectedServerAddr: {
					Username:      expectedUsername,
					Password:      expectedPassword,
					ServerAddress: expectedServerAddr,
				},
			},
			expectedError: nil,
		},
		"Config: $HOME/.docker/config.json": {
			homeDirProvided: true,
			configContent:   config,
			configLocation: configLocation{
				subfolder: ".docker",
				filename:  "config.json",
			},
			expectedAuthConfigs: map[string]types.AuthConfig{
				expectedServerAddr: {
					Username:      expectedUsername,
					Password:      expectedPassword,
					ServerAddress: expectedServerAddr,
				},
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			expectedConfigFile := ""
			HomeDirectory = ""

			if test.homeDirProvided {
				dir, err := ioutil.TempDir("", "docker_config_test")
				require.NoErrorf(t, err, "failed to create temp directory: %s", err)
				defer os.RemoveAll(dir)

				HomeDirectory = dir
				configDir := HomeDirectory

				if test.configLocation.subfolder != "" {
					configDir = filepath.Join(dir, test.configLocation.subfolder)
					err := os.MkdirAll(configDir, 0777)
					require.NoErrorf(t, err, "failed to create docker config subfolder: %s", err)
				}

				if test.configLocation.filename != "" {
					f := filepath.Join(configDir, test.configLocation.filename)
					err := ioutil.WriteFile(f, test.configContent, 0666)
					require.NoError(t, err, "failed to write test docker config: %s", err)

					expectedConfigFile = f
				}
			}

			configFile, authConfigs, err := ReadDockerAuthConfigsFromHomeDir("")

			assert.True(t, errors.Is(err, test.expectedError), "expected err %T, but got %T", test.expectedError, err)
			assert.Equal(t, expectedConfigFile, configFile)
			assert.Equal(t, test.expectedAuthConfigs, authConfigs, "Configs should be equal")
		})
	}
}

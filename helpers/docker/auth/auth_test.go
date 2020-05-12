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
	registryDomain1Config = types.AuthConfig{
		Username:      "test_user_1",
		Password:      "test_password_1",
		ServerAddress: "https://registry.domain.tld:5005/v1/",
	}
	registryDomain2Config = types.AuthConfig{
		Username:      "test_user_2",
		Password:      "test_password_2",
		ServerAddress: "registry2.domain.tld:5005",
	}
	registryGitlabConfig = types.AuthConfig{
		Username:      "test_user_3",
		Password:      "test_password_3",
		ServerAddress: "registry.gitlab.tld:1234",
	}
	emptyCredentials = []common.Credentials{}
)

func TestGetConfigForImage(t *testing.T) {
	tests := map[string]struct {
		precreateConfigFile  bool
		dockerAuthVariable   string
		credentials          []common.Credentials
		image                string
		expectedResultGetter func(homeDir string) *RegistryInfo
	}{
		"registry1 from file only": {
			precreateConfigFile: true,
			dockerAuthVariable:  "",
			credentials:         emptyCredentials,
			image:               imageRegistryDomain1,
			expectedResultGetter: func(homeDir string) *RegistryInfo {
				return &RegistryInfo{
					Source:     filepath.Join(homeDir, ".dockercfg"),
					AuthConfig: registryDomain1Config,
				}
			},
		},
		"registry2 from file only": {
			precreateConfigFile: true,
			dockerAuthVariable:  "",
			credentials:         emptyCredentials,
			image:               imageRegistryDomain2,
			expectedResultGetter: func(homeDir string) *RegistryInfo {
				return &RegistryInfo{
					Source:     filepath.Join(homeDir, ".dockercfg"),
					AuthConfig: registryDomain2Config,
				}
			},
		},
		"missing credentials, file only": {
			precreateConfigFile: true,
			dockerAuthVariable:  "",
			credentials:         emptyCredentials,
			image:               imageGitlabDomain,
			expectedResultGetter: func(homeDir string) *RegistryInfo {
				return &RegistryInfo{}
			},
		},
		"no file and gitlab credentials. image in gitlab credentials": {
			precreateConfigFile: false,
			dockerAuthVariable:  "",
			credentials:         gitlabRegistryCredentials,
			image:               imageGitlabDomain,
			expectedResultGetter: func(homeDir string) *RegistryInfo {
				return &RegistryInfo{
					Source:     authConfigSourceNameJobPayload,
					AuthConfig: registryGitlabConfig,
				}
			},
		},
		"both file and gitlab credentials. image in gitlab credentials": {
			precreateConfigFile: true,
			dockerAuthVariable:  "",
			credentials:         gitlabRegistryCredentials,
			image:               imageGitlabDomain,
			expectedResultGetter: func(homeDir string) *RegistryInfo {
				return &RegistryInfo{
					Source:     authConfigSourceNameJobPayload,
					AuthConfig: registryGitlabConfig,
				}
			},
		},
		"DOCKER_AUTH_CONFIG only": {
			precreateConfigFile: false,
			dockerAuthVariable:  testVariableAuthConfigs,
			credentials:         emptyCredentials,
			image:               imageRegistryDomain1,
			expectedResultGetter: func(homeDir string) *RegistryInfo {
				return &RegistryInfo{
					Source:     authConfigSourceNameUserVariable,
					AuthConfig: registryDomain1Config,
				}
			},
		},
		"DOCKER_AUTH_CONFIG overrides home dir": {
			precreateConfigFile: true,
			dockerAuthVariable:  testVariableAuthConfigs,
			credentials:         emptyCredentials,
			image:               imageRegistryDomain1,
			expectedResultGetter: func(homeDir string) *RegistryInfo {
				return &RegistryInfo{
					Source:     authConfigSourceNameUserVariable,
					AuthConfig: registryDomain1Config,
				}
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			cleanup := setupTestHomeDirectoryConfig(t, tt.precreateConfigFile)
			defer cleanup()

			result := GetConfigForImage(tt.image, tt.dockerAuthVariable, "", tt.credentials)

			assert.Equal(t, tt.expectedResultGetter(HomeDirectory), result)
		})
	}
}

func TestGetConfigs(t *testing.T) {
	cleanup := setupTestHomeDirectoryConfig(t, true)
	defer cleanup()
	result := GetConfigs(testVariableAuthConfigs, "", gitlabRegistryCredentials)

	assert.Equal(t, map[string]*RegistryInfo{
		"https://registry.domain.tld:5005/v1/": {
			Source: authConfigSourceNameUserVariable,
			AuthConfig: types.AuthConfig{
				Username:      "test_user_1",
				Password:      "test_password_1",
				ServerAddress: "https://registry.domain.tld:5005/v1/",
			},
		},
		"registry.gitlab.tld:1234": {
			Source: authConfigSourceNameJobPayload,
			AuthConfig: types.AuthConfig{
				Username:      "test_user_3",
				Password:      "test_password_3",
				ServerAddress: "registry.gitlab.tld:1234",
			},
		},
		"registry2.domain.tld:5005": {
			Source: filepath.Join(HomeDirectory, ".dockercfg"),
			AuthConfig: types.AuthConfig{
				Username:      "test_user_2",
				Password:      "test_password_2",
				ServerAddress: "registry2.domain.tld:5005",
			},
		},
	}, result)
}

func setupTestHomeDirectoryConfig(t *testing.T, precreateConfigFile bool) func() {
	oldHomeDirectory := HomeDirectory

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

	return func() {
		if precreateConfigFile {
			os.RemoveAll(HomeDirectory)
		}
		HomeDirectory = oldHomeDirectory
	}
}

func TestSplitDockerImageName(t *testing.T) {
	remote, image := splitDockerImageName("tutum.co/user/ubuntu")
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
	remote, image := splitDockerImageName("user/ubuntu")
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
	remote, image := splitDockerImageName("index.docker.io/user/ubuntu")
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

package auth

import (
	"encoding/base64"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func addGitLabRegistryCredentials(e *executor) {
	e.Build.Credentials = []common.Credentials{
		{
			Type:     "registry",
			URL:      "registry.gitlab.tld:1234",
			Username: "gitlab-ci-token",
			Password: e.Build.Token,
		},
	}
}

func addRemoteVariableCredentials(e *executor) {
	e.Build.Variables = common.JobVariables{
		common.JobVariable{
			Key:   "DOCKER_AUTH_CONFIG",
			Value: testVariableAuthConfigs,
		},
	}
}

func addLocalVariableCredentials(e *executor) {
	e.Build.Runner.Environment = []string{
		"DOCKER_AUTH_CONFIG=" + testVariableAuthConfigs,
	}
}

func assertEmptyCredentials(t *testing.T, ac *types.AuthConfig, messageElements ...string) {
	if ac != nil {
		assert.Empty(t, ac.ServerAddress, "ServerAddress for %v", messageElements)
		assert.Empty(t, ac.Username, "Username for %v", messageElements)
		assert.Empty(t, ac.Password, "Password for %v", messageElements)
	}
}

func assertCredentials(t *testing.T, serverAddress, username, password string, ac *types.AuthConfig, messageElements ...string) {
	assert.Equal(t, serverAddress, ac.ServerAddress, "ServerAddress for %v", messageElements)
	assert.Equal(t, username, ac.Username, "Username for %v", messageElements)
	assert.Equal(t, password, ac.Password, "Password for %v", messageElements)
}

func getTestAuthConfig(t *testing.T, e *executor, imageName string) *types.AuthConfig {
	ac := e.getAuthConfig(imageName)

	return ac
}

func testVariableAuthConfig(t *testing.T, e *executor) {
	t.Run("withoutGitLabRegistry", func(t *testing.T) {
		ac := getTestAuthConfig(t, e, "registry.domain.tld:5005/image/name:version")
		assertCredentials(t, "https://registry.domain.tld:5005/v1/", "test_user", "test_password", ac, "registry.domain.tld:5005/image/name:version")

		ac = getTestAuthConfig(t, e, "registry2.domain.tld:5005/image/name:version")
		assertCredentials(t, "registry2.domain.tld:5005", "test_user", "test_password", ac, "registry2.domain.tld:5005/image/name:version")

		ac = getTestAuthConfig(t, e, "registry.gitlab.tld:1234/image/name:version")
		assertEmptyCredentials(t, ac, "registry.gitlab.tld:1234")
	})

	t.Run("withGitLabRegistry", func(t *testing.T) {
		addGitLabRegistryCredentials(e)

		ac := getTestAuthConfig(t, e, "registry.domain.tld:5005/image/name:version")
		assertCredentials(t, "https://registry.domain.tld:5005/v1/", "test_user", "test_password", ac, "registry.domain.tld:5005/image/name:version")

		ac = getTestAuthConfig(t, e, "registry2.domain.tld:5005/image/name:version")
		assertCredentials(t, "registry2.domain.tld:5005", "test_user", "test_password", ac, "registry2.domain.tld:5005/image/name:version")

		ac = getTestAuthConfig(t, e, "registry.gitlab.tld:1234/image/name:version")
		assertCredentials(t, "registry.gitlab.tld:1234", "gitlab-ci-token", "abcd123456", ac, "registry.gitlab.tld:1234")
	})
}

func TestGetRemoteVariableAuthConfig(t *testing.T) {
	e := getAuthConfigTestExecutor(t, true)
	addRemoteVariableCredentials(e)

	testVariableAuthConfig(t, e)
}

func TestGetLocalVariableAuthConfig(t *testing.T) {
	e := getAuthConfigTestExecutor(t, true)
	addLocalVariableCredentials(e)

	testVariableAuthConfig(t, e)
}

func TestGetDefaultAuthConfig(t *testing.T) {
	t.Run("withoutGitLabRegistry", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, false)

		ac := getTestAuthConfig(t, e, "docker:dind")
		assertEmptyCredentials(t, ac, "docker:dind")

		ac = getTestAuthConfig(t, e, "registry.gitlab.tld:1234/image/name:version")
		assertEmptyCredentials(t, ac, "registry.gitlab.tld:1234")

		ac = getTestAuthConfig(t, e, "registry.domain.tld:5005/image/name:version")
		assertEmptyCredentials(t, ac, "registry.domain.tld:5005/image/name:version")
	})

	t.Run("withGitLabRegistry", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, false)
		addGitLabRegistryCredentials(e)

		ac := getTestAuthConfig(t, e, "docker:dind")
		assertEmptyCredentials(t, ac, "docker:dind")

		ac = getTestAuthConfig(t, e, "registry.domain.tld:5005/image/name:version")
		assertEmptyCredentials(t, ac, "registry.domain.tld:5005/image/name:version")

		ac = getTestAuthConfig(t, e, "registry.gitlab.tld:1234/image/name:version")
		assertCredentials(t, "registry.gitlab.tld:1234", "gitlab-ci-token", "abcd123456", ac, "registry.gitlab.tld:1234")
	})
}

func TestAuthConfigOverwritingOrder(t *testing.T) {
	testVariableAuthConfigs = `{"auths":{"registry.gitlab.tld:1234":{"auth":"ZnJvbV92YXJpYWJsZTpwYXNzd29yZA=="}}}`
	testFileAuthConfigs = `{"auths":{"registry.gitlab.tld:1234":{"auth":"ZnJvbV9maWxlOnBhc3N3b3Jk"}}}`

	imageName := "registry.gitlab.tld:1234/image/name:latest"

	t.Run("gitlabRegistryOnly", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, false)
		addGitLabRegistryCredentials(e)

		ac := getTestAuthConfig(t, e, imageName)
		assertCredentials(t, "registry.gitlab.tld:1234", "gitlab-ci-token", e.Build.Token, ac, imageName)
	})

	t.Run("withConfigFromRemoteVariable", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, false)
		addGitLabRegistryCredentials(e)
		addRemoteVariableCredentials(e)

		ac := getTestAuthConfig(t, e, imageName)
		assertCredentials(t, "registry.gitlab.tld:1234", "from_variable", "password", ac, imageName)
	})

	t.Run("withConfigFromLocalVariable", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, false)
		addGitLabRegistryCredentials(e)
		addLocalVariableCredentials(e)

		ac := getTestAuthConfig(t, e, imageName)
		assertCredentials(t, "registry.gitlab.tld:1234", "from_variable", "password", ac, imageName)
	})

	t.Run("withConfigFromFile", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, true)
		addGitLabRegistryCredentials(e)

		ac := getTestAuthConfig(t, e, imageName)
		assertCredentials(t, "registry.gitlab.tld:1234", "from_file", "password", ac, imageName)
	})

	t.Run("withConfigFromVariableAndFromFile", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, true)
		addGitLabRegistryCredentials(e)
		addRemoteVariableCredentials(e)

		ac := getTestAuthConfig(t, e, imageName)
		assertCredentials(t, "registry.gitlab.tld:1234", "from_variable", "password", ac, imageName)
	})

	t.Run("withConfigFromLocalAndRemoteVariable", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, true)
		addGitLabRegistryCredentials(e)
		addRemoteVariableCredentials(e)
		testVariableAuthConfigs = `{"auths":{"registry.gitlab.tld:1234":{"auth":"ZnJvbV9sb2NhbF92YXJpYWJsZTpwYXNzd29yZA=="}}}`
		addLocalVariableCredentials(e)

		ac := getTestAuthConfig(t, e, imageName)
		assertCredentials(t, "registry.gitlab.tld:1234", "from_variable", "password", ac, imageName)
	})
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

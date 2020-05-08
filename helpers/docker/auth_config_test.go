package docker

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
)

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

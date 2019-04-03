package commands

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"testing"

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

func mockEnv(t *testing.T, key string, value string) func() {
	err := os.Setenv(key, value)
	require.NoError(t, err, "Variable %q not set properly", key)

	return func() {
		err := os.Unsetenv(key)
		assert.NoError(t, err, "Variable %q not unset properly", key)
	}
}

func testRegisterCommandRun(t *testing.T, network common.Network, args ...string) {
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

	args = append([]string{"binary", "register"}, args...)
	app.Run(args)
}

func testRegisterCommandDeprecatedOptions(t *testing.T, args ...string) (string, string) {
	hook := test.NewGlobal()

	network := new(common.MockNetwork)
	defer network.AssertExpectations(t)

	network.On("RegisterRunner", mock.Anything, mock.Anything).Once().Return(&common.RegisterRunnerResponse{
		Token: "test-runner-token",
	})

	configFile, err := ioutil.TempFile("", "config.toml")
	require.NoError(t, err)

	configFile.Close()
	defer os.Remove(configFile.Name())

	arguments := []string{
		"-n",
		"--config", configFile.Name(),
		"--url", "http://gitlab.example.com/",
		"--registration-token", "test-registration-token",
		"--executor", "shell",
		"--cache-type", "s3",
	}
	arguments = append(arguments, args...)

	testRegisterCommandRun(t, network, arguments...)

	content, err := ioutil.ReadFile(configFile.Name())
	require.NoError(t, err)

	return string(content), getLogrusOutput(t, hook)
}

// TODO: Remove in 12.0
func TestRegisterCacheDeprecatedOptions_CLIOptions(t *testing.T) {
	content, output := testRegisterCommandDeprecatedOptions(
		t,
		"--cache-s3-cache-path", "test_path",
		"--cache-cache-shared",
	)

	assert.Contains(t, content, `
  [runners.cache]
    Type = "s3"
    Path = "test_path"
    Shared = true
`)

	assert.Contains(t, output, "'--cache-s3-cache-path' command line option and `$S3_CACHE_PATH` environment variables are deprecated and will be removed in GitLab Runner 12.0. Please use '--cache-path' or '$CACHE_PATH' instead")
	assert.Contains(t, output, "'--cache-cache-shared' command line is deprecated and will be removed in GitLab Runner 12.0. Please use '--cache-shared' instead")
}

// TODO: Remove in 12.0
func TestRegisterCacheDeprecatedOptions_EnvVariables(t *testing.T) {
	defer mockEnv(t, "S3_CACHE_PATH", "test_path")()
	defer mockEnv(t, "S3_SERVER_ADDRESS", "server_address")()
	defer mockEnv(t, "S3_ACCESS_KEY", "access_key")()
	defer mockEnv(t, "S3_SECRET_KEY", "secret_key")()
	defer mockEnv(t, "S3_BUCKET_NAME", "bucket_name")()
	defer mockEnv(t, "S3_BUCKET_LOCATION", "bucket_location")()
	defer mockEnv(t, "S3_CACHE_INSECURE", "1")()

	content, output := testRegisterCommandDeprecatedOptions(t)

	assert.Contains(t, content, `
  [runners.cache]
    Type = "s3"
    Path = "test_path"
    [runners.cache.s3]
      ServerAddress = "server_address"
      AccessKey = "access_key"
      SecretKey = "secret_key"
      BucketName = "bucket_name"
      BucketLocation = "bucket_location"
      Insecure = true
    [runners.cache.gcs]
`)

	assert.Contains(t, output, "'--cache-s3-cache-path' command line option and `$S3_CACHE_PATH` environment variables are deprecated and will be removed in GitLab Runner 12.0. Please use '--cache-path' or '$CACHE_PATH' instead")
	assert.Contains(t, output, "S3_SERVER_ADDRESS environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use CACHE_S3_SERVER_ADDRESS instead")
	assert.Contains(t, output, "S3_ACCESS_KEY environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use CACHE_S3_ACCESS_KEY instead")
	assert.Contains(t, output, "S3_SECRET_KEY environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use CACHE_S3_SECRET_KEY instead")
	assert.Contains(t, output, "S3_BUCKET_NAME environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use CACHE_S3_BUCKET_NAME instead")
	assert.Contains(t, output, "S3_BUCKET_LOCATION environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use CACHE_S3_BUCKET_LOCATION instead")
	assert.Contains(t, output, "S3_CACHE_INSECURE environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use CACHE_S3_INSECURE instead")
}

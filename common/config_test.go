package common

import (
	"bytes"
	"os"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"gitlab.com/ayufan/golang-cli-helpers"
)

func getLogrusOutput(t *testing.T, hook *test.Hook) string {
	buf := &bytes.Buffer{}
	for _, entry := range hook.AllEntries() {
		message, err := entry.String()
		require.NoError(t, err)
		buf.WriteString(message)
	}

	return buf.String()
}

// TODO: Remove in 12.0
func TestCacheConfig_DeprecatedSupport_TOML(t *testing.T) {
	hook := test.NewGlobal()
	content := `
[cache]
  ServerAddress = "server_address"
  AccessKey = "access_key"
  SecretKey = "secret_key"
  BucketName = "bucket_name"
  BucketLocation = "bucket_location"
  Insecure = true
`
	test := struct {
		Cache CacheConfig `toml:"cache,omitempty"`
	}{}

	_, err := toml.Decode(content, &test)
	require.NoError(t, err)

	assert.Equal(t, "server_address", test.Cache.GetServerAddress())
	assert.Equal(t, "access_key", test.Cache.GetAccessKey())
	assert.Equal(t, "secret_key", test.Cache.GetSecretKey())
	assert.Equal(t, "bucket_name", test.Cache.GetBucketName())
	assert.Equal(t, "bucket_location", test.Cache.GetBucketLocation())
	assert.True(t, test.Cache.GetInsecure())

	output := getLogrusOutput(t, hook)
	assert.Contains(t, output, "[runners.cache] ServerAddress setting is deprecated and will be removed in GitLab Runner 12.0. Please use [runners.cache.s3] ServerAddress instead")
	assert.Contains(t, output, "[runners.cache] AccessKey setting is deprecated and will be removed in GitLab Runner 12.0. Please use [runners.cache.s3] AccessKey instead")
	assert.Contains(t, output, "[runners.cache] SecretKey setting is deprecated and will be removed in GitLab Runner 12.0. Please use [runners.cache.s3] SecretKey instead")
	assert.Contains(t, output, "[runners.cache] BucketName setting is deprecated and will be removed in GitLab Runner 12.0. Please use [runners.cache.s3] BucketName instead")
	assert.Contains(t, output, "[runners.cache] BucketLocation setting is deprecated and will be removed in GitLab Runner 12.0. Please use [runners.cache.s3] BucketLocation instead")
	assert.Contains(t, output, "[runners.cache] Insecure setting is deprecated and will be removed in GitLab Runner 12.0. Please use [runners.cache.s3] Insecure instead")
}

type cacheConfigDeprecatedCommand struct {
	Cache CacheConfig `namespace:"cache"`

	test func(c *cacheConfigDeprecatedCommand)
}

func (c *cacheConfigDeprecatedCommand) Execute(cliCtx *cli.Context) {
	c.test(c)
}

func testCacheConfigDeprecatedSupportRunCommand(test func(c *cacheConfigDeprecatedCommand), args ...string) {
	cmd := &cacheConfigDeprecatedCommand{test: test}

	app := cli.NewApp()
	app.Commands = []cli.Command{
		{
			Name:   "test",
			Action: cmd.Execute,
			Flags:  clihelpers.GetFlagsFromStruct(cmd),
		},
	}

	args = append([]string{"binary", "test"}, args...)
	app.Run(args)
}

// TODO: Remove in 12.0
func TestCacheConfig_DeprecatedSupport_CliOptions(t *testing.T) {
	hook := test.NewGlobal()
	test := func(c *cacheConfigDeprecatedCommand) {
		assert.Equal(t, "test_path", c.Cache.GetPath())
		assert.True(t, c.Cache.GetShared())
	}

	testCacheConfigDeprecatedSupportRunCommand(test, "--cache-s3-cache-path", "test_path", "--cache-cache-shared")

	output := getLogrusOutput(t, hook)
	assert.Contains(t, output, "'--cache-s3-cache-path' command line option and `$S3_CACHE_PATH` environment variables are deprecated and will be removed in GitLab Runner 12.0. Please use '--cache-path' or '$CACHE_PATH' instead")
	assert.Contains(t, output, "'--cache-cache-shared' command line is deprecated and will be removed in GitLab Runner 12.0. Please use '--cache-shared' instead")
}

func mockEnv(t *testing.T, key string, value string) func() {
	err := os.Setenv(key, value)
	require.NoError(t, err, "Variable %q not set properly", key)

	return func() {
		err := os.Unsetenv(key)
		assert.NoError(t, err, "Variable %q not unset properly", key)
	}
}

// TODO: Remove in 12.0
func TestCacheConfig_DeprecatedSupport_EnvVariables(t *testing.T) {
	hook := test.NewGlobal()
	test := func(c *cacheConfigDeprecatedCommand) {
		assert.Equal(t, "test_path", c.Cache.GetPath())
		assert.Equal(t, "server_address", c.Cache.GetServerAddress())
		assert.Equal(t, "access_key", c.Cache.GetAccessKey())
		assert.Equal(t, "secret_key", c.Cache.GetSecretKey())
		assert.Equal(t, "bucket_name", c.Cache.GetBucketName())
		assert.Equal(t, "bucket_location", c.Cache.GetBucketLocation())
		assert.True(t, c.Cache.GetInsecure())
	}

	defer mockEnv(t, "S3_CACHE_PATH", "test_path")()
	defer mockEnv(t, "S3_SERVER_ADDRESS", "server_address")()
	defer mockEnv(t, "S3_ACCESS_KEY", "access_key")()
	defer mockEnv(t, "S3_SECRET_KEY", "secret_key")()
	defer mockEnv(t, "S3_BUCKET_NAME", "bucket_name")()
	defer mockEnv(t, "S3_BUCKET_LOCATION", "bucket_location")()
	defer mockEnv(t, "S3_CACHE_INSECURE", "1")()

	testCacheConfigDeprecatedSupportRunCommand(test)

	output := getLogrusOutput(t, hook)
	assert.Contains(t, output, "'--cache-s3-cache-path' command line option and `$S3_CACHE_PATH` environment variables are deprecated and will be removed in GitLab Runner 12.0. Please use '--cache-path' or '$CACHE_PATH' instead")
	assert.Contains(t, output, "S3_SERVER_ADDRESS environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use CACHE_S3_SERVER_ADDRESS instead")
	assert.Contains(t, output, "S3_ACCESS_KEY environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use CACHE_S3_ACCESS_KEY instead")
	assert.Contains(t, output, "S3_SECRET_KEY environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use CACHE_S3_SECRET_KEY instead")
	assert.Contains(t, output, "S3_BUCKET_NAME environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use CACHE_S3_BUCKET_NAME instead")
	assert.Contains(t, output, "S3_BUCKET_LOCATION environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use CACHE_S3_BUCKET_LOCATION instead")
	assert.Contains(t, output, "S3_CACHE_INSECURE environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use CACHE_S3_INSECURE instead")
}

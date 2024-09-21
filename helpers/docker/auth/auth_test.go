//go:build !integration

package auth

import (
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/cli/cli/config/types"
	"github.com/sirupsen/logrus/hooks/test"
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
	testFileAuthConfigs = `{"auths":{` +
		`"https://registry.domain.tld:5005/v1/":{"auth":"dGVzdF91c2VyXzE6dGVzdF9wYXNzd29yZF8x"},` +
		`"registry2.domain.tld:5005":{"auth":"dGVzdF91c2VyXzI6dGVzdF9wYXNzd29yZF8y"}}}`
	testFileAuthConfigsWithPathTraversalFormat = `{"auths":{` +
		`"https://registry.domain.tld:5005/v1/":{"auth":"dGVzdF91c2VyXzE6dGVzdF9wYXNzd29yZF8x"},` +
		`"registry2.domain.tld:5005":{"auth":"dGVzdF91c2VyXzI6dGVzdF9wYXNzd29yZF8y"}},%s}`
	testDockerAuthConfigs = `{"auths":{` +
		`"https://registry.domain.tld:5005/v1/":{"auth":"dGVzdF91c2VyXzE6dGVzdF9wYXNzd29yZF8x"}}}`
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
	registryScriptConfig = types.AuthConfig{
		Username: "script_user_1",
		Password: "script_password_1",
	}
	registryGitlabConfig = types.AuthConfig{
		Username:      "test_user_3",
		Password:      "test_password_3",
		ServerAddress: "registry.gitlab.tld:1234",
	}
	emptyCredentials []common.Credentials
)

func TestGetConfigForImage(t *testing.T) {
	tests := map[string]struct {
		configFileContents string
		dockerAuthValue    string
		credentials        []common.Credentials
		image              string
		assertResult       func(*RegistryInfo, error)
	}{
		"registry1 from file only": {
			configFileContents: testFileAuthConfigs,
			dockerAuthValue:    "",
			credentials:        emptyCredentials,
			image:              imageRegistryDomain1,
			assertResult: func(result *RegistryInfo, err error) {
				expectedResult := &RegistryInfo{
					Source:     filepath.Join(HomeDirectory, ".dockercfg"),
					AuthConfig: registryDomain1Config,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
			},
		},
		"registry2 from file only": {
			configFileContents: testFileAuthConfigs,
			dockerAuthValue:    "",
			credentials:        emptyCredentials,
			image:              imageRegistryDomain2,
			assertResult: func(result *RegistryInfo, err error) {
				expectedResult := &RegistryInfo{
					Source:     filepath.Join(HomeDirectory, ".dockercfg"),
					AuthConfig: registryDomain2Config,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
			},
		},
		"registry2 from file only overrides credential store with path traversal attempt": {
			configFileContents: fmt.Sprintf(
				testFileAuthConfigsWithPathTraversalFormat,
				fmt.Sprintf(`"credsStore" : "%s"`, getPathWithPathTraversalAttempt(t)),
			),
			dockerAuthValue: "",
			credentials:     emptyCredentials,
			image:           imageRegistryDomain2,
			assertResult: func(result *RegistryInfo, err error) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)
			},
		},
		"registry2 from file only overrides credential helper with path traversal attempt": {
			configFileContents: fmt.Sprintf(
				testFileAuthConfigsWithPathTraversalFormat,
				fmt.Sprintf(`"credHelpers" : {"%s" : "%s"}`, imageRegistryDomain2, getPathWithPathTraversalAttempt(t)),
			),
			dockerAuthValue: "",
			credentials:     emptyCredentials,
			image:           imageRegistryDomain2,
			assertResult: func(result *RegistryInfo, err error) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)
			},
		},
		"missing credentials, file only": {
			configFileContents: testFileAuthConfigs,
			dockerAuthValue:    "",
			credentials:        emptyCredentials,
			image:              imageGitlabDomain,
			assertResult: func(result *RegistryInfo, err error) {
				assert.NoError(t, err)
				assert.Nil(t, result)
			},
		},
		"no file and gitlab credentials, image in gitlab credentials": {
			configFileContents: "",
			dockerAuthValue:    "",
			credentials:        gitlabRegistryCredentials,
			image:              imageGitlabDomain,
			assertResult: func(result *RegistryInfo, err error) {
				expectedResult := &RegistryInfo{
					Source:     authConfigSourceNameJobPayload,
					AuthConfig: registryGitlabConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
			},
		},
		"both file and gitlab credentials, image in gitlab credentials": {
			configFileContents: testFileAuthConfigs,
			dockerAuthValue:    "",
			credentials:        gitlabRegistryCredentials,
			image:              imageGitlabDomain,
			assertResult: func(result *RegistryInfo, err error) {
				expectedResult := &RegistryInfo{
					Source:     authConfigSourceNameJobPayload,
					AuthConfig: registryGitlabConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
			},
		},
		"DOCKER_AUTH_CONFIG only": {
			configFileContents: "",
			dockerAuthValue:    testDockerAuthConfigs,
			credentials:        emptyCredentials,
			image:              imageRegistryDomain1,
			assertResult: func(result *RegistryInfo, err error) {
				expectedResult := &RegistryInfo{
					Source:     authConfigSourceNameUserVariable,
					AuthConfig: registryDomain1Config,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
			},
		},
		"DOCKER_AUTH_CONFIG overrides home dir": {
			configFileContents: testFileAuthConfigs,
			dockerAuthValue:    testDockerAuthConfigs,
			credentials:        emptyCredentials,
			image:              imageRegistryDomain1,
			assertResult: func(result *RegistryInfo, err error) {
				expectedResult := &RegistryInfo{
					Source:     authConfigSourceNameUserVariable,
					AuthConfig: registryDomain1Config,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credential store": {
			configFileContents: "",
			dockerAuthValue:    fmt.Sprintf(`{"credsStore" : "%s"}`, getValidCredentialHelperSuffix(t)),
			credentials:        emptyCredentials,
			image:              imageRegistryDomain2,
			assertResult: func(result *RegistryInfo, err error) {
				expectedResult := &RegistryInfo{
					Source:     authConfigSourceNameUserVariable,
					AuthConfig: registryScriptConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credential helper path": {
			configFileContents: "",
			dockerAuthValue:    getDockerAuthForCredentialHelperPathPath(t, imageRegistryDomain2),
			credentials:        emptyCredentials,
			image:              imageRegistryDomain2,
			assertResult: func(result *RegistryInfo, err error) {
				expectedResult := &RegistryInfo{
					Source:     authConfigSourceNameUserVariable,
					AuthConfig: registryScriptConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credential store with path traversal": {
			configFileContents: "",
			dockerAuthValue:    getDockerAuthForCredentialStorePathWithPathTraversal(t),
			credentials:        emptyCredentials,
			image:              imageRegistryDomain2,
			assertResult: func(result *RegistryInfo, err error) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credentials helper with path traversal entry": {
			configFileContents: "",
			dockerAuthValue: getDockerAuthForCredentialHelperPathWithPathTraversal(
				t,
				registryDomain2Config.ServerAddress,
			),
			credentials: emptyCredentials,
			image:       imageRegistryDomain2,
			assertResult: func(result *RegistryInfo, err error) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credentials helper with path traversal entry and falls back to config file": {
			configFileContents: testFileAuthConfigs,
			dockerAuthValue: getDockerAuthForCredentialHelperPathWithPathTraversal(
				t,
				"registry.domain.tld:5005",
			),
			credentials: emptyCredentials,
			image:       imageRegistryDomain1,
			assertResult: func(result *RegistryInfo, err error) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credentials helper with path traversal entry and another valid entry": {
			configFileContents: "",
			dockerAuthValue:    getDockerAuthForCredentialHelperPathWithPathTraversalAndGoodFallback(t),
			credentials:        emptyCredentials,
			image:              imageRegistryDomain2,
			assertResult: func(result *RegistryInfo, err error) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)
			},
		},
	}

	dir, err := os.Getwd()
	require.NoError(t, err)

	// Add testdata directory to PATH so that docker-credential-* scripts are picked up
	pathSep := ":"
	if runtime.GOOS == "windows" {
		pathSep = ";"
	}
	originalPATH := os.Getenv("PATH")
	err = os.Setenv("PATH", filepath.Join(dir, "testdata")+pathSep+originalPATH)
	require.NoError(t, err)

	logger, _ := test.NewNullLogger()

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			cleanup := setupTestHomeDirectoryConfig(t, tt.configFileContents)
			defer cleanup()

			result, err := ResolveConfigForImage(tt.image, tt.dockerAuthValue, "", tt.credentials, logger)
			tt.assertResult(result, err)
		})
	}

	// Reset PATH
	err = os.Setenv("PATH", originalPATH)
	assert.NoError(t, err)
}

// getDockerAuthForCredentialStorePathWithPathTraversal returns a DOCKER_AUTH_VALUE
// value containing a credsStore value containing a path traversal attempt which should cause an error
func getDockerAuthForCredentialStorePathWithPathTraversal(t *testing.T) string {
	return fmt.Sprintf(`{"credsStore" : "%s"}`, getPathWithPathTraversalAttempt(t))
}

// getDockerAuthForCredentialHelperPathPath returns a DOCKER_AUTH_VALUE
// value containing a valid credsStore value
func getDockerAuthForCredentialHelperPathPath(t *testing.T, domain string) string {
	return fmt.Sprintf(`{"credHelpers" : {"%s" : "%s"}}`, domain, getValidCredentialHelperSuffix(t))
}

// getDockerAuthForCredentialHelperPathWithPathTraversal returns a DOCKER_AUTH_VALUE
// value containing a credHelpers map with a single entry: a path traversal attempt which should cause an error
func getDockerAuthForCredentialHelperPathWithPathTraversal(t *testing.T, domain string) string {
	return fmt.Sprintf(
		`{"credHelpers" : {"%s" : "%s"}}`,
		domain,
		getPathWithPathTraversalAttempt(t),
	)
}

// getPathWithPathTraversalAttempt returns a relative path to an executable which exists on the host
// OS, to test path traversal attempts in credential helpers
func getPathWithPathTraversalAttempt(t *testing.T) string {
	dir, err := os.Getwd()
	require.NoError(t, err)

	credHelperPath, err := filepath.Rel(dir, `/usr/bin/sudo`)
	if runtime.GOOS == "windows" {
		credHelperPath, err = filepath.Rel(dir, `C:\Windows\notepad.exe`)
		credHelperPath = strings.ReplaceAll(credHelperPath, `\`, `\\`)
	}

	require.NoError(t, err)

	return credHelperPath
}

// getDockerAuthForCredentialHelperPathWithPathTraversalAndGoodFallback returns a DOCKER_AUTH_VALUE
// value containing a credHelpers map with two entries: the first is a path traversal attempt
// which should cause an error, and the second is a valid entry
func getDockerAuthForCredentialHelperPathWithPathTraversalAndGoodFallback(t *testing.T) string {
	return fmt.Sprintf(
		`{"credHelpers" : {"registry.domain.tld:5006" : "%s", "%s" : "%s"}}`,
		getPathWithPathTraversalAttempt(t),
		registryDomain2Config.ServerAddress,
		getValidCredentialHelperSuffix(t),
	)
}

func getValidCredentialHelperSuffix(t *testing.T) string {
	switch runtime.GOOS {
	case "windows":
		return "windows.cmd"
	case "linux", "darwin":
		return "bin.sh"
	default:
		assert.FailNow(t, "credentials helper and credentials store tests are not supported on %q", runtime.GOOS)
	}
	return ""
}

func setupTestHomeDirectoryConfig(t *testing.T, configFileContents string) func() {
	oldHomeDirectory := HomeDirectory

	if configFileContents != "" {
		tempHomeDir := t.TempDir()
		dockerConfigFile := path.Join(tempHomeDir, ".dockercfg")
		err := os.WriteFile(dockerConfigFile, []byte(configFileContents), 0o600)
		require.NoError(t, err)
		HomeDirectory = tempHomeDir
	} else {
		HomeDirectory = ""
	}

	return func() {
		HomeDirectory = oldHomeDirectory
	}
}

func TestDockerImage(t *testing.T) {
	path := dockerImageNamePath("foo.bar:123/asdf/baz:latest")
	assert.Equal(t, "foo.bar:123/asdf/baz", path)

	path = dockerImageNamePath("foo.bar/asdf/baz:latest")
	assert.Equal(t, "foo.bar/asdf/baz", path)

	path = dockerImageNamePath("foo.bar/asdf/baz")
	assert.Equal(t, "foo.bar/asdf/baz", path)

	path = dockerImageNamePath("registry.local/ns/image")
	assert.Equal(t, "registry.local/ns/image", path)

	path = dockerImageNamePath("foo.bar:123/asdf/baz")
	assert.Equal(t, "foo.bar:123/asdf/baz", path)

	path = dockerImageNamePath("FOO.BAR:123/With/Case")
	assert.Equal(t, "foo.bar:123/With/Case", path)
}

func TestConvertToRegistryPath(t *testing.T) {
	path := convertToRegistryPath("my.hostname")
	assert.Equal(t, "my.hostname", path)

	path = convertToRegistryPath("my.hostname/with/path")
	assert.Equal(t, "my.hostname/with/path", path)

	path = convertToRegistryPath("MY.HOSTNAME/With/Path/CASE")
	assert.Equal(t, "my.hostname/With/Path/CASE", path)

	path = convertToRegistryPath("my.hostname/with/tag/image:latest")
	assert.Equal(t, "my.hostname/with/tag/image", path)

	path = convertToRegistryPath("https://index.docker.io/v1/")
	assert.Equal(t, "docker.io", path)

	path = convertToRegistryPath("HTTPS://INDEX.DOCKER.IO/V1/")
	assert.Equal(t, "docker.io", path)
}

func TestPaths(t *testing.T) {
	testDockerAuthConfigsPaths := `{"auths":{` +
		`"registry.local":{"auth":"dGVzdF91c2VyXzE6dGVzdF9wYXNzd29yZF8x"},` +
		`"registry.local/ns":{"auth":"dGVzdF91c2VyXzI6dGVzdF9wYXNzd29yZF8y"},` +
		`"registry.local/ns/some/image":{"auth":"dGVzdF91c2VyXzM6dGVzdF9wYXNzd29yZF8z"}` +
		`}}`

	logger, _ := test.NewNullLogger()

	result, err := ResolveConfigForImage("registry.local/foo/image:3",
		testDockerAuthConfigsPaths, "", nil, logger)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Equal(t, "$DOCKER_AUTH_CONFIG", result.Source)
	assert.Equal(t, "test_user_1", result.AuthConfig.Username)
	assert.Equal(t, "test_password_1", result.AuthConfig.Password)

	result, err = ResolveConfigForImage("registry.local/ns/image:5",
		testDockerAuthConfigsPaths, "", nil, logger)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Equal(t, "$DOCKER_AUTH_CONFIG", result.Source)
	assert.Equal(t, "test_user_2", result.AuthConfig.Username)
	assert.Equal(t, "test_password_2", result.AuthConfig.Password)

	result, err = ResolveConfigForImage("registry.local/ns/some/image:l",
		testDockerAuthConfigsPaths, "", nil, logger)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Equal(t, "test_user_3", result.AuthConfig.Username)
	assert.Equal(t, "test_password_3", result.AuthConfig.Password)

	result, err = ResolveConfigForImage("no_auth_configured/image:l",
		testDockerAuthConfigsPaths, "", nil, logger)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestGetConfigs(t *testing.T) {
	cleanup := setupTestHomeDirectoryConfig(t, testFileAuthConfigs)
	defer cleanup()
	logger, _ := test.NewNullLogger()
	result, err := ResolveConfigs(testDockerAuthConfigs, "", gitlabRegistryCredentials, logger)
	assert.NoError(t, err)

	assert.Equal(t, map[string]RegistryInfo{
		"registry.domain.tld:5005": {
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

func TestGetConfigs_DuplicatedRegistryCredentials(t *testing.T) {
	registryCredentials := []common.Credentials{
		{
			Type:     "registry",
			URL:      "registry.domain.tld:5005",
			Username: "test_user_1",
			Password: "test_password_1",
		},
	}

	cleanup := setupTestHomeDirectoryConfig(t, testFileAuthConfigs)
	defer cleanup()

	logger, _ := test.NewNullLogger()
	result, err := ResolveConfigs("", "", registryCredentials, logger)
	assert.NoError(t, err)

	expectedResult := map[string]RegistryInfo{
		"registry.domain.tld:5005": {
			Source: filepath.Join(HomeDirectory, ".dockercfg"),
			AuthConfig: types.AuthConfig{
				Username:      "test_user_1",
				Password:      "test_password_1",
				ServerAddress: "https://registry.domain.tld:5005/v1/",
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
	}

	assert.Equal(t, expectedResult, result)
}

func TestDockerImageNamePath(t *testing.T) {
	path := dockerImageNamePath("tutum.co/user/ubuntu")
	assert.Equal(t, "tutum.co/user/ubuntu", path)
}

func TestDockerImageNamePathTag(t *testing.T) {
	path := dockerImageNamePath("tutum.co/user/ubuntu:latest")
	assert.Equal(t, "tutum.co/user/ubuntu", path)
}

func TestDockerImageNamePathPort(t *testing.T) {
	path := dockerImageNamePath("cr.internal:5000/user/ubuntu")
	assert.Equal(t, "cr.internal:5000/user/ubuntu", path)
}

func TestDefaultDockerImageNamePath(t *testing.T) {
	path := dockerImageNamePath("user/ubuntu")
	assert.Equal(t, "docker.io/user/ubuntu", path)
}

func TestDefaultIndexDockerImageNamePath(t *testing.T) {
	path := dockerImageNamePath("index.docker.io/user/ubuntu")
	assert.Equal(t, "docker.io/user/ubuntu", path)
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
				dir := t.TempDir()

				HomeDirectory = dir
				configDir := HomeDirectory

				if test.configLocation.subfolder != "" {
					configDir = filepath.Join(dir, test.configLocation.subfolder)
					err := os.MkdirAll(configDir, 0o777)
					require.NoErrorf(t, err, "failed to create docker config subfolder: %s", err)
				}

				if test.configLocation.filename != "" {
					f := filepath.Join(configDir, test.configLocation.filename)
					err := os.WriteFile(f, test.configContent, 0o666)
					require.NoError(t, err, "failed to write test docker config: %s", err)

					expectedConfigFile = f
				}
			}

			configFile, authConfigs, err := readDockerConfigsFromHomeDir("")

			assert.ErrorIs(t, err, test.expectedError)
			assert.Equal(t, expectedConfigFile, configFile)
			assert.Equal(t, test.expectedAuthConfigs, authConfigs, "Configs should be equal")
		})
	}
}

//go:build !integration

package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
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
	testFileAuthConfigs = createTestDockerConfig([]testRegistryConfig{
		{registry: "https://registry.domain.tld:5005/v1/", user: "test_user_1", pass: "test_password_1"},
		{registry: "registry2.domain.tld:5005", user: "test_user_2", pass: "test_password_2"},
	})
	testDockerAuthConfigs = createTestDockerConfig([]testRegistryConfig{
		{registry: "https://registry.domain.tld:5005/v1/", user: "test_user_1", pass: "test_password_1"},
	})
	testFileAuthConfigsWithPathTraversalFormat = `{"auths":{` +
		`"https://registry.domain.tld:5005/v1/":{"auth":"dGVzdF91c2VyXzE6dGVzdF9wYXNzd29yZF8x"},` +
		`"registry2.domain.tld:5005":{"auth":"dGVzdF91c2VyXzI6dGVzdF9wYXNzd29yZF8y"}},%s}`
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
)

func TestGetConfigForImage(t *testing.T) {
	tests := map[string]struct {
		configFileContents string
		dockerAuthValue    string
		jobCredentials     []common.Credentials
		image              string
		assertResult       func(*testing.T, *RegistryInfo, error)
	}{
		"registry1 from file only": {
			configFileContents: testFileAuthConfigs,
			image:              imageRegistryDomain1,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
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
			image:              imageRegistryDomain2,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
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
			image: imageRegistryDomain2,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
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
			image: imageRegistryDomain2,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)
			},
		},
		"missing credentials, file only": {
			configFileContents: testFileAuthConfigs,
			image:              imageGitlabDomain,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
				assert.NoError(t, err)
				assert.Nil(t, result)
			},
		},
		"no file and gitlab credentials, image in gitlab credentials": {
			jobCredentials: gitlabRegistryCredentials,
			image:          imageGitlabDomain,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
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
			jobCredentials:     gitlabRegistryCredentials,
			image:              imageGitlabDomain,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
				expectedResult := &RegistryInfo{
					Source:     authConfigSourceNameJobPayload,
					AuthConfig: registryGitlabConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
			},
		},
		"DOCKER_AUTH_CONFIG only": {
			dockerAuthValue: testDockerAuthConfigs,
			image:           imageRegistryDomain1,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
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
			image:              imageRegistryDomain1,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
				expectedResult := &RegistryInfo{
					Source:     authConfigSourceNameUserVariable,
					AuthConfig: registryDomain1Config,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credential store": {
			dockerAuthValue: fmt.Sprintf(`{"credsStore" : "%s"}`, getValidCredentialHelperSuffix(t)),
			image:           imageRegistryDomain2,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
				authConfig := registryScriptConfig
				authConfig.ServerAddress = "https://registry2.domain.tld:5005/v1/"

				expectedResult := &RegistryInfo{
					Source:     authConfigSourceNameUserVariable,
					AuthConfig: authConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credential helper path": {
			dockerAuthValue: getDockerAuthForCredentialHelperPathPath(t, imageRegistryDomain2),
			image:           imageRegistryDomain2,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
				authConfig := registryScriptConfig
				authConfig.ServerAddress = "registry2.domain.tld:5005/image/name:version"
				expectedResult := &RegistryInfo{
					Source:     authConfigSourceNameUserVariable,
					AuthConfig: authConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credential store with path traversal": {
			dockerAuthValue: getDockerAuthForCredentialStorePathWithPathTraversal(t),
			image:           imageRegistryDomain2,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credentials helper with path traversal entry": {
			dockerAuthValue: getDockerAuthForCredentialHelperPathWithPathTraversal(
				t,
				registryDomain2Config.ServerAddress,
			),
			image: imageRegistryDomain2,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
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
			image: imageRegistryDomain1,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credentials helper with path traversal entry and another valid entry": {
			dockerAuthValue: getDockerAuthForCredentialHelperPathWithPathTraversalAndGoodFallback(t),
			image:           imageRegistryDomain2,
			assertResult: func(t *testing.T, result *RegistryInfo, err error) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)
			},
		},
	}

	dir, err := os.Getwd()
	require.NoError(t, err)

	// Prepend testdata directory to PATH so that docker-credential-* scripts are picked up
	prependToPath(t, filepath.Join(dir, "testdata"))

	logger, _ := test.NewNullLogger()

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			setupTestHomeDirectoryConfig(t, tt.configFileContents)

			regInfo, err := ResolveConfigForImage(tt.image, tt.dockerAuthValue, "", tt.jobCredentials, logger)
			tt.assertResult(t, regInfo, err)
		})
	}
}

func TestConvertToRegistryPath(t *testing.T) {
	tests := map[string]string{
		"my.hostname":                            "my.hostname",
		"my.hostname/with/path":                  "my.hostname/with/path",
		"my.HOSTNAME/With/Path/CASE":             "my.hostname/With/Path/CASE",
		"my.hostname/with/tag/image:latest":      "my.hostname/with/tag/image",
		"my.hostname:5000/with/tag/image:latest": "my.hostname:5000/with/tag/image",
		"http://index.docker.io/v1/":             "docker.io",
		"https://index.docker.io/v1/":            "docker.io",
		"HTTP://INDEX.DOCKER.IO/V1/":             "docker.io",
		"HTTPS://INDEX.DOCKER.IO/V1/":            "docker.io",
		"HTTPS://INDEX.DOCKER.IO/V1/blibb":       "docker.io",
		"https://my.hostname/v1/something":       "my.hostname",
	}

	for imageRef, expected := range tests {
		t.Run(imageRef, func(t *testing.T) {
			actual := convertToRegistryPath(imageRef)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestCredsForImagesWithDifferentPaths(t *testing.T) {
	testDockerAuthConfigs := createTestDockerConfig([]testRegistryConfig{
		{registry: "registry.local", user: "test_user_1", pass: "test_password_1"},
		{registry: "registry.local/ns", user: "test_user_2", pass: "test_password_2"},
		{registry: "registry.local/ns/some/image", user: "test_user_3", pass: "test_password_3"},
	})

	logger, _ := test.NewNullLogger()

	tests := map[string]struct {
		jobCreds         []common.Credentials
		expectNoResult   bool
		expectedSource   string
		expectedUsername string
		expectedPassword string
	}{
		"registry.local/foo/image:3": {
			expectedSource:   "$DOCKER_AUTH_CONFIG",
			expectedUsername: "test_user_1",
			expectedPassword: "test_password_1",
		},
		"registry.local/ns/image:5": {
			expectedSource:   "$DOCKER_AUTH_CONFIG",
			expectedUsername: "test_user_2",
			expectedPassword: "test_password_2",
		},
		"registry.local/ns/some/image:l": {
			expectedSource:   "$DOCKER_AUTH_CONFIG",
			expectedUsername: "test_user_3",
			expectedPassword: "test_password_3",
		},
		"no_auth_configured/image:l": {
			expectNoResult: true,
		},
		"registry.local/ns/blipp/image:foo": {
			// there are job creds, but for the same path we already have a $DOCKER_AUTH_CONFIG, $DOCKER_AUTH_CONFIG wins
			jobCreds: []common.Credentials{{
				Type:     "registry",
				Username: "job-cred-user",
				Password: "job-cred-pass",
				URL:      "registry.local/ns",
			}},
			expectedSource:   "$DOCKER_AUTH_CONFIG",
			expectedUsername: "test_user_2",
			expectedPassword: "test_password_2",
		},
		"registry.local/ns/blipp/image:bar": {
			// there are job creds which have a more specific match for the image ref than auths in $DOCKER_AUTH_CONFIG
			jobCreds: []common.Credentials{{
				Type:     "registry",
				Username: "job-cred-user",
				Password: "job-cred-pass",
				URL:      "registry.local/ns/blipp",
			}},
			expectedSource:   "job payload (GitLab Registry)",
			expectedUsername: "job-cred-user",
			expectedPassword: "job-cred-pass",
		},
	}

	for imageRef, test := range tests {
		t.Run(imageRef, func(t *testing.T) {
			resolved, err := ResolveConfigForImage(imageRef, testDockerAuthConfigs, "", test.jobCreds, logger)
			require.NoError(t, err, "resolving creds for image ref")

			if test.expectNoResult {
				assert.Nil(t, resolved)
			} else {
				assert.Equal(t, test.expectedSource, resolved.Source)
				assert.Equal(t, test.expectedUsername, resolved.AuthConfig.Username)
				assert.Equal(t, test.expectedPassword, resolved.AuthConfig.Password)
			}
		})
	}
}

func TestGetConfigs(t *testing.T) {
	setupTestHomeDirectoryConfig(t, testFileAuthConfigs)
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

	setupTestHomeDirectoryConfig(t, testFileAuthConfigs)

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

func TestDockerImagePathNormalization(t *testing.T) {
	tests := map[string]string{
		"tutum.co/user/ubuntu":         "tutum.co/user/ubuntu",
		"tutum.co/user/ubuntu:latest":  "tutum.co/user/ubuntu",
		"cr.internal:5000/user/ubuntu": "cr.internal:5000/user/ubuntu",
		"user/ubuntu":                  "docker.io/user/ubuntu",
		"index.docker.io/user/ubuntu":  "docker.io/user/ubuntu",
		"docker.io/user/ubuntu":        "docker.io/user/ubuntu",

		"foo.bar:123/asdf/baz:latest": "foo.bar:123/asdf/baz",
		"foo.bar/asdf/baz:latest":     "foo.bar/asdf/baz",
		"foo.bar/asdf/baz":            "foo.bar/asdf/baz",
		"registry.local/ns/image":     "registry.local/ns/image",
		"foo.bar:123/asdf/baz":        "foo.bar:123/asdf/baz",
		"FOO.BAR:123/With/Case":       "foo.bar:123/With/Case",

		"DOCKER.io/user/ubuntu":       "docker.io/user/ubuntu",
		"index.DOCKER.io/user/ubuntu": "docker.io/user/ubuntu",
		"InDex.DOCKER.io/user/ubuntu": "docker.io/user/ubuntu",
		"localhost/test:xxx":          "localhost/test",
		"LOCALHOST/test:xxx":          "localhost/test",
		"notLocalhost/test":           "docker.io/notLocalhost/test",
		"localhost:1234/test":         "localhost:1234/test",
	}

	for imageRef, expected := range tests {
		actual := normalizeImageRef(imageRef)
		assert.Equal(t, expected, actual)
	}
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

// prependToPath sets a new PATH, prepending paths to the currently set PATH. PATH is set via t.Setenv, thus it's
// automatically reverted after the test.
func prependToPath(t *testing.T, paths ...string) {
	newPath := slices.Clone(paths)
	if path, ok := os.LookupEnv("PATH"); ok {
		newPath = append(newPath, path)
	}
	t.Setenv("PATH", strings.Join(newPath, string(filepath.ListSeparator)))
}

func setupTestHomeDirectoryConfig(t *testing.T, configFileContents string) {
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

	t.Cleanup(func() {
		HomeDirectory = oldHomeDirectory
	})
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

type testRegistryConfig struct {
	registry string
	user     string
	pass     string
}

func createTestDockerConfig(regs []testRegistryConfig) string {
	config := map[string]map[string]map[string]string{
		"auths": map[string]map[string]string{},
	}

	for _, creds := range regs {
		config["auths"][creds.registry] = map[string]string{
			"auth": base64.StdEncoding.EncodeToString([]byte(creds.user + ":" + creds.pass)),
		}
	}

	json, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		// this should never happen, as map[string]string can always be marshalled
		panic("cannot marshal docker config: " + err.Error())
	}

	return string(json)
}

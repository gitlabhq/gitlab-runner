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
		checks             func(*testing.T, *RegistryInfo, error, string, *fakeLogger)
	}{
		"registry1 from file only": {
			configFileContents: testFileAuthConfigs,
			image:              imageRegistryDomain1,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				expectedResult := &RegistryInfo{
					RegistryPath: "registry.domain.tld:5005",
					Source:       filepath.Join(homeDir, ".dockercfg"),
					AuthConfig:   registryDomain1Config,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)

				dockerConf := filepath.Join(homeDir, ".dockercfg")
				logger.ExpectLogs(t, [][]any{
					{fmt.Sprintf(`Loaded Docker credentials, source = %q, hostnames = [registry.domain.tld:5005 registry2.domain.tld:5005], error = <nil>`, dockerConf)},
				})
			},
		},
		"registry2 from file only": {
			configFileContents: testFileAuthConfigs,
			image:              imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				expectedResult := &RegistryInfo{
					RegistryPath: "registry2.domain.tld:5005",
					Source:       filepath.Join(homeDir, ".dockercfg"),
					AuthConfig:   registryDomain2Config,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)

				dockerConf := filepath.Join(homeDir, ".dockercfg")
				logger.ExpectLogs(t, [][]any{
					{fmt.Sprintf(`Loaded Docker credentials, source = %q, hostnames = [registry.domain.tld:5005 registry2.domain.tld:5005], error = <nil>`, dockerConf)},
				})
			},
		},
		"registry2 from file only overrides credential store with path traversal attempt": {
			configFileContents: fmt.Sprintf(
				testFileAuthConfigsWithPathTraversalFormat,
				fmt.Sprintf(`"credsStore" : "%s"`, getPathWithPathTraversalAttempt(t)),
			),
			image: imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)

				logger.ExpectLogs(t, nil)
			},
		},
		"registry2 from file only overrides credential helper with path traversal attempt": {
			configFileContents: fmt.Sprintf(
				testFileAuthConfigsWithPathTraversalFormat,
				fmt.Sprintf(`"credHelpers" : {"%s" : "%s"}`, imageRegistryDomain2, getPathWithPathTraversalAttempt(t)),
			),
			image: imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)

				logger.ExpectLogs(t, nil)
			},
		},
		"missing credentials, file only": {
			configFileContents: testFileAuthConfigs,
			image:              imageGitlabDomain,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				assert.NoError(t, err)
				assert.Nil(t, result)

				dockerConf := filepath.Join(homeDir, ".dockercfg")
				logger.ExpectLogs(t, [][]any{
					{fmt.Sprintf(`Loaded Docker credentials, source = %q, hostnames = [registry.domain.tld:5005 registry2.domain.tld:5005], error = <nil>`, dockerConf)},
				})
			},
		},
		"no file and gitlab credentials, image in gitlab credentials": {
			jobCredentials: gitlabRegistryCredentials,
			image:          imageGitlabDomain,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				expectedResult := &RegistryInfo{
					RegistryPath: "registry.gitlab.tld:1234",
					Source:       authConfigSourceNameJobPayload,
					AuthConfig:   registryGitlabConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)

				logger.ExpectLogs(t, [][]any{
					{`Loaded Docker credentials, source = "job payload (GitLab Registry)", hostnames = [registry.gitlab.tld:1234], error = <nil>`},
				})
			},
		},
		"both file and gitlab credentials, image in gitlab credentials": {
			configFileContents: testFileAuthConfigs,
			jobCredentials:     gitlabRegistryCredentials,
			image:              imageGitlabDomain,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				expectedResult := &RegistryInfo{
					RegistryPath: "registry.gitlab.tld:1234",
					Source:       authConfigSourceNameJobPayload,
					AuthConfig:   registryGitlabConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)

				dockerConf := filepath.Join(homeDir, ".dockercfg")
				logger.ExpectLogs(t, [][]any{
					{fmt.Sprintf(`Loaded Docker credentials, source = %q, hostnames = [registry.domain.tld:5005 registry2.domain.tld:5005], error = <nil>`, dockerConf)},
					{`Loaded Docker credentials, source = "job payload (GitLab Registry)", hostnames = [registry.gitlab.tld:1234], error = <nil>`},
				})
			},
		},
		"DOCKER_AUTH_CONFIG only": {
			dockerAuthValue: testDockerAuthConfigs,
			image:           imageRegistryDomain1,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				expectedResult := &RegistryInfo{
					RegistryPath: "registry.domain.tld:5005",
					Source:       authConfigSourceNameUserVariable,
					AuthConfig:   registryDomain1Config,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)

				logger.ExpectLogs(t, [][]any{
					{`Loaded Docker credentials, source = "$DOCKER_AUTH_CONFIG", hostnames = [registry.domain.tld:5005], error = <nil>`},
				})
			},
		},
		"DOCKER_AUTH_CONFIG overrides home dir": {
			configFileContents: testFileAuthConfigs,
			dockerAuthValue:    testDockerAuthConfigs,
			image:              imageRegistryDomain1,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				expectedResult := &RegistryInfo{
					RegistryPath: "registry.domain.tld:5005",
					Source:       authConfigSourceNameUserVariable,
					AuthConfig:   registryDomain1Config,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)

				dockerConf := filepath.Join(homeDir, ".dockercfg")
				logger.ExpectLogs(t, [][]any{
					{`Loaded Docker credentials, source = "$DOCKER_AUTH_CONFIG", hostnames = [registry.domain.tld:5005], error = <nil>`},
					{fmt.Sprintf(`Not adding Docker credentials: credentials for "registry.domain.tld:5005" already set from "$DOCKER_AUTH_CONFIG", ignoring credentials from %q`, dockerConf)},
					{fmt.Sprintf(`Loaded Docker credentials, source = %q, hostnames = [registry.domain.tld:5005 registry2.domain.tld:5005], error = <nil>`, dockerConf)},
				})
			},
		},
		"DOCKER_AUTH_CONFIG overrides credential store": {
			dockerAuthValue: fmt.Sprintf(`{"credsStore" : "%s"}`, getValidCredentialHelperSuffix(t)),
			image:           imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				authConfig := registryScriptConfig
				authConfig.ServerAddress = "https://registry2.domain.tld:5005/v1/"

				expectedResult := &RegistryInfo{
					RegistryPath: "registry2.domain.tld:5005",
					Source:       authConfigSourceNameUserVariable,
					AuthConfig:   authConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)

				logger.ExpectLogs(t, [][]any{
					{`Loaded Docker credentials, source = "$DOCKER_AUTH_CONFIG", hostnames = [registry2.domain.tld:5005], error = <nil>`},
				})
			},
		},
		"DOCKER_AUTH_CONFIG overrides credential helper path": {
			dockerAuthValue: getDockerAuthForCredentialHelperPathPath(t, imageRegistryDomain2),
			image:           imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				authConfig := registryScriptConfig
				authConfig.ServerAddress = "registry2.domain.tld:5005/image/name:version"
				expectedResult := &RegistryInfo{
					RegistryPath: "registry2.domain.tld:5005/image/name",
					Source:       authConfigSourceNameUserVariable,
					AuthConfig:   authConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)

				logger.ExpectLogs(t, [][]any{
					{`Loaded Docker credentials, source = "$DOCKER_AUTH_CONFIG", hostnames = [registry2.domain.tld:5005/image/name], error = <nil>`},
				})
			},
		},
		"DOCKER_AUTH_CONFIG overrides credential store with path traversal": {
			dockerAuthValue: getDockerAuthForCredentialStorePathWithPathTraversal(t),
			image:           imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)

				logger.ExpectLogs(t, nil)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credentials helper with path traversal entry": {
			dockerAuthValue: getDockerAuthForCredentialHelperPathWithPathTraversal(
				t,
				registryDomain2Config.ServerAddress,
			),
			image: imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)

				logger.ExpectLogs(t, nil)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credentials helper with path traversal entry and falls back to config file": {
			configFileContents: testFileAuthConfigs,
			dockerAuthValue: getDockerAuthForCredentialHelperPathWithPathTraversal(
				t,
				"registry.domain.tld:5005",
			),
			image: imageRegistryDomain1,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)

				logger.ExpectLogs(t, nil)
			},
		},
		"DOCKER_AUTH_CONFIG overrides credentials helper with path traversal entry and another valid entry": {
			dockerAuthValue: getDockerAuthForCredentialHelperPathWithPathTraversalAndGoodFallback(t),
			image:           imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)

				logger.ExpectLogs(t, nil)
			},
		},
	}

	dir, err := os.Getwd()
	require.NoError(t, err)

	// Prepend testdata directory to PATH so that docker-credential-* scripts are picked up
	prependToPath(t, filepath.Join(dir, "testdata"))

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			home := setupTestHomeDirectoryConfig(t, tt.configFileContents)
			resolver := NewResolver()
			resolver.HomeDirGetter = func() string { return home }
			logger := &fakeLogger{}

			regInfo, err := resolver.ConfigForImage(tt.image, tt.dockerAuthValue, "", tt.jobCredentials, logger)
			tt.checks(t, regInfo, err, home, logger)
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

	tests := map[string]struct {
		jobCreds         []common.Credentials
		expectNoResult   bool
		expectedSource   string
		expectedUsername string
		expectedPassword string
		expectedLogs     [][]any
	}{
		"registry.local/foo/image:3": {
			expectedSource:   "$DOCKER_AUTH_CONFIG",
			expectedUsername: "test_user_1",
			expectedPassword: "test_password_1",
			expectedLogs: [][]any{
				{`Loaded Docker credentials, source = "$DOCKER_AUTH_CONFIG", hostnames = [registry.local registry.local/ns registry.local/ns/some/image], error = <nil>`},
			},
		},
		"registry.local/ns/image:5": {
			expectedSource:   "$DOCKER_AUTH_CONFIG",
			expectedUsername: "test_user_2",
			expectedPassword: "test_password_2",
			expectedLogs: [][]any{
				{`Loaded Docker credentials, source = "$DOCKER_AUTH_CONFIG", hostnames = [registry.local registry.local/ns registry.local/ns/some/image], error = <nil>`},
			},
		},
		"registry.local/ns/some/image:l": {
			expectedSource:   "$DOCKER_AUTH_CONFIG",
			expectedUsername: "test_user_3",
			expectedPassword: "test_password_3",
			expectedLogs: [][]any{
				{`Loaded Docker credentials, source = "$DOCKER_AUTH_CONFIG", hostnames = [registry.local registry.local/ns registry.local/ns/some/image], error = <nil>`},
			},
		},
		"no_auth_configured/image:l": {
			expectNoResult: true,
			expectedLogs: [][]any{
				{`Loaded Docker credentials, source = "$DOCKER_AUTH_CONFIG", hostnames = [registry.local registry.local/ns registry.local/ns/some/image], error = <nil>`},
			},
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
			expectedLogs: [][]any{
				{`Loaded Docker credentials, source = "$DOCKER_AUTH_CONFIG", hostnames = [registry.local registry.local/ns registry.local/ns/some/image], error = <nil>`},
				{`Not adding Docker credentials: credentials for "registry.local/ns" already set from "$DOCKER_AUTH_CONFIG", ignoring credentials from "job payload (GitLab Registry)"`},
				{`Loaded Docker credentials, source = "job payload (GitLab Registry)", hostnames = [registry.local/ns], error = <nil>`},
			},
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
			expectedLogs: [][]any{
				{`Loaded Docker credentials, source = "$DOCKER_AUTH_CONFIG", hostnames = [registry.local registry.local/ns registry.local/ns/some/image], error = <nil>`},
				{`Loaded Docker credentials, source = "job payload (GitLab Registry)", hostnames = [registry.local/ns/blipp], error = <nil>`},
			},
		},
	}

	for imageRef, test := range tests {
		t.Run(imageRef, func(t *testing.T) {
			logger := &fakeLogger{}
			resolver := NewResolver()
			resolver.HomeDirGetter = func() string { return "" }

			resolved, err := resolver.ConfigForImage(imageRef, testDockerAuthConfigs, "", test.jobCreds, logger)
			require.NoError(t, err, "resolving creds for image ref")

			if test.expectNoResult {
				assert.Nil(t, resolved)
			} else {
				assert.Equal(t, test.expectedSource, resolved.Source)
				assert.Equal(t, test.expectedUsername, resolved.AuthConfig.Username)
				assert.Equal(t, test.expectedPassword, resolved.AuthConfig.Password)
			}

			logger.ExpectLogs(t, test.expectedLogs)
		})
	}
}

func TestGetConfigs(t *testing.T) {
	home := setupTestHomeDirectoryConfig(t, testFileAuthConfigs)
	resolver := NewResolver()
	resolver.HomeDirGetter = func() string { return home }
	logger := &fakeLogger{}

	result, err := resolver.AllConfigs(testDockerAuthConfigs, "", gitlabRegistryCredentials, logger)
	assert.NoError(t, err)

	assert.Equal(t, RegistryInfos{
		{
			RegistryPath: "registry.domain.tld:5005",
			Source:       authConfigSourceNameUserVariable,
			AuthConfig: types.AuthConfig{
				Username:      "test_user_1",
				Password:      "test_password_1",
				ServerAddress: "https://registry.domain.tld:5005/v1/",
			},
		},
		{
			RegistryPath: "registry2.domain.tld:5005",
			Source:       filepath.Join(home, ".dockercfg"),
			AuthConfig: types.AuthConfig{
				Username:      "test_user_2",
				Password:      "test_password_2",
				ServerAddress: "registry2.domain.tld:5005",
			},
		},
		{
			RegistryPath: "registry.gitlab.tld:1234",
			Source:       authConfigSourceNameJobPayload,
			AuthConfig: types.AuthConfig{
				Username:      "test_user_3",
				Password:      "test_password_3",
				ServerAddress: "registry.gitlab.tld:1234",
			},
		},
	}, result)

	dockerConf := filepath.Join(home, ".dockercfg")
	logger.ExpectLogs(t, [][]any{
		{`Loaded Docker credentials, source = "$DOCKER_AUTH_CONFIG", hostnames = [registry.domain.tld:5005], error = <nil>`},
		{fmt.Sprintf(`Not adding Docker credentials: credentials for "registry.domain.tld:5005" already set from "$DOCKER_AUTH_CONFIG", ignoring credentials from %q`, dockerConf)},
		{fmt.Sprintf(`Loaded Docker credentials, source = %q, hostnames = [registry.domain.tld:5005 registry2.domain.tld:5005], error = <nil>`, dockerConf)},
		{`Loaded Docker credentials, source = "job payload (GitLab Registry)", hostnames = [registry.gitlab.tld:1234], error = <nil>`},
	})
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

	home := setupTestHomeDirectoryConfig(t, testFileAuthConfigs)
	resolver := NewResolver()
	resolver.HomeDirGetter = func() string { return home }
	logger := &fakeLogger{}

	result, err := resolver.AllConfigs("", "", registryCredentials, logger)
	assert.NoError(t, err)

	expectedResult := RegistryInfos{
		{
			RegistryPath: "registry.domain.tld:5005",
			Source:       filepath.Join(home, ".dockercfg"),
			AuthConfig: types.AuthConfig{
				Username:      "test_user_1",
				Password:      "test_password_1",
				ServerAddress: "https://registry.domain.tld:5005/v1/",
			},
		},
		{
			RegistryPath: "registry2.domain.tld:5005",
			Source:       filepath.Join(home, ".dockercfg"),
			AuthConfig: types.AuthConfig{
				Username:      "test_user_2",
				Password:      "test_password_2",
				ServerAddress: "registry2.domain.tld:5005",
			},
		},
	}

	assert.Equal(t, expectedResult, result)

	dockerConf := filepath.Join(home, ".dockercfg")
	logger.ExpectLogs(t, [][]any{
		{fmt.Sprintf(`Loaded Docker credentials, source = %q, hostnames = [registry.domain.tld:5005 registry2.domain.tld:5005], error = <nil>`, dockerConf)},
		{fmt.Sprintf(`Not adding Docker credentials: credentials for "registry.domain.tld:5005" already set from %q, ignoring credentials from "job payload (GitLab Registry)"`, dockerConf)},
		{`Loaded Docker credentials, source = "job payload (GitLab Registry)", hostnames = [registry.domain.tld:5005], error = <nil>`},
	})
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

func setupTestHomeDirectoryConfig(t *testing.T, configFileContents string) string {
	fakeHome := t.TempDir()

	if configFileContents != "" {
		dockerConfigFile := path.Join(fakeHome, ".dockercfg")
		err := os.WriteFile(dockerConfigFile, []byte(configFileContents), 0o600)
		require.NoError(t, err)
	}

	return fakeHome
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
		expectedAuthConfigs []types.AuthConfig
		expectedError       error
	}{
		"Home dir value is blank": {
			expectedError: errNoHomeDir,
		},
		"No configs": {
			homeDirProvided:     true,
			configLocation:      configLocation{},
			expectedAuthConfigs: []types.AuthConfig{},
		},
		"Config: $HOME/.dockercfg": {
			homeDirProvided: true,
			configContent:   config,
			configLocation: configLocation{
				subfolder: "",
				filename:  ".dockercfg",
			},
			expectedAuthConfigs: []types.AuthConfig{
				{
					Username:      expectedUsername,
					Password:      expectedPassword,
					ServerAddress: expectedServerAddr,
				},
			},
		},
		"Config: $HOME/.docker/config.json": {
			homeDirProvided: true,
			configContent:   config,
			configLocation: configLocation{
				subfolder: ".docker",
				filename:  "config.json",
			},
			expectedAuthConfigs: []types.AuthConfig{
				{
					Username:      expectedUsername,
					Password:      expectedPassword,
					ServerAddress: expectedServerAddr,
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			homeDirGetter := func() string { return "" }
			expectedConfigFile := ""

			if test.homeDirProvided {
				fakeHome := t.TempDir()

				homeDirGetter = func() string { return fakeHome }
				configDir := fakeHome

				if test.configLocation.subfolder != "" {
					configDir = filepath.Join(fakeHome, test.configLocation.subfolder)
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

			resolver := NewResolver()
			resolver.HomeDirGetter = homeDirGetter

			configFile, authConfigs, err := resolver.readDockerConfigsFromHomeDir("")

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

type fakeLogger [][]any

func (l *fakeLogger) Debugln(args ...any) {
	*l = append(*l, args)
}

func (l fakeLogger) ExpectLogs(t *testing.T, expectedLogs [][]any) {
	t.Helper()

	le := len(expectedLogs)
	la := len(l)

	assert.Len(t, l, le, "expected %d logs, got %d", le, la)

	for i := 0; i < min(le, la); i++ {
		assert.Equal(t, expectedLogs[i], l[i], "log line %d", i)
	}
}

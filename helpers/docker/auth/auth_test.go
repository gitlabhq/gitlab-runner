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

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
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
	gitlabRegistryCredentials = []spec.Credentials{
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
	pinnedHelperName, pinnedHelperPath := getPinnedCredentialHelper(t)

	tests := map[string]struct {
		configFileContents  string
		dockerAuthValue     string
		jobCredentials      []spec.Credentials
		credHelperAllowlist map[string][]string
		image               string
		checks              func(*testing.T, *RegistryInfo, error, string, *fakeLogger)
	}{
		"registry1 from file only": {
			configFileContents: testFileAuthConfigs,
			image:              imageRegistryDomain1,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				expectedResult := &RegistryInfo{
					Path:       "registry.domain.tld:5005",
					Source:     filepath.Join(homeDir, ".dockercfg"),
					AuthConfig: registryDomain1Config,
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
					Path:       "registry2.domain.tld:5005",
					Source:     filepath.Join(homeDir, ".dockercfg"),
					AuthConfig: registryDomain2Config,
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
					Path:       "registry.gitlab.tld:1234",
					Source:     configSourceNameJobPayload,
					AuthConfig: registryGitlabConfig,
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
					Path:       "registry.gitlab.tld:1234",
					Source:     configSourceNameJobPayload,
					AuthConfig: registryGitlabConfig,
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
					Path:       "registry.domain.tld:5005",
					Source:     configSourceNameUserVariable,
					AuthConfig: registryDomain1Config,
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
					Path:       "registry.domain.tld:5005",
					Source:     configSourceNameUserVariable,
					AuthConfig: registryDomain1Config,
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
			dockerAuthValue:     fmt.Sprintf(`{"credsStore" : "%s"}`, getValidCredentialHelperSuffix(t)),
			credHelperAllowlist: map[string][]string{getValidCredentialHelperSuffix(t): {"*"}},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				authConfig := registryScriptConfig
				authConfig.ServerAddress = "https://registry2.domain.tld:5005/v1/"

				expectedResult := &RegistryInfo{
					Path:       "registry2.domain.tld:5005",
					Source:     configSourceNameUserVariable,
					AuthConfig: authConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
				assert.Empty(t, logger.warningLogs)

				logger.ExpectLogs(t, [][]any{
					{`Loaded Docker credentials, source = "$DOCKER_AUTH_CONFIG", hostnames = [registry2.domain.tld:5005], error = <nil>`},
				})
			},
		},
		"DOCKER_AUTH_CONFIG overrides credential helper path": {
			dockerAuthValue:     getDockerAuthForCredentialHelperPathPath(t, imageRegistryDomain2),
			credHelperAllowlist: map[string][]string{getValidCredentialHelperSuffix(t): {"**"}},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				authConfig := registryScriptConfig
				authConfig.ServerAddress = "registry2.domain.tld:5005/image/name:version"
				expectedResult := &RegistryInfo{
					Path:       "registry2.domain.tld:5005/image/name",
					Source:     configSourceNameUserVariable,
					AuthConfig: authConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
				assert.Empty(t, logger.warningLogs)

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
			// Permit the valid entry so it still executes when iterated first: a
			// path traversal sibling must stay fatal even when another entry is
			// permitted and would have resolved.
			credHelperAllowlist: map[string][]string{getValidCredentialHelperSuffix(t): {"registry2.domain.tld:5005"}},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// path traversal element will cause an error to be returned
				assert.Nil(t, result)
				assert.ErrorIs(t, err, errPathTraversal)

				logger.ExpectLogs(t, nil)
			},
		},
		"allowed credsStore with a missing binary logs warning and continues": {
			dockerAuthValue:     `{"credsStore": "nonexistent-helper"}`,
			credHelperAllowlist: map[string][]string{"nonexistent-helper": {"*"}},
			image:               imageRegistryDomain1,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// Helper is permitted, so it is executed; a missing binary must
				// warn and continue rather than fail the whole resolution.
				assert.Nil(t, result)
				assert.NoError(t, err)
				logger.ExpectLogs(t, nil)
				require.Len(t, logger.warningLogs, 1)
				warnMsg := fmt.Sprint(logger.warningLogs[0]...)
				assert.Contains(t, warnMsg, configSourceNameUserVariable)
				assert.Contains(t, warnMsg, "nonexistent-helper")
			},
		},
		"allowed credHelpers with a missing binary logs warning and continues": {
			dockerAuthValue:     fmt.Sprintf(`{"credHelpers": {%q: "nonexistent-helper"}}`, registryDomain1Config.ServerAddress),
			credHelperAllowlist: map[string][]string{"nonexistent-helper": {"*"}},
			image:               imageRegistryDomain1,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				assert.Nil(t, result)
				assert.NoError(t, err)
				logger.ExpectLogs(t, nil)
				require.Len(t, logger.warningLogs, 1)
				warnMsg := fmt.Sprint(logger.warningLogs[0]...)
				assert.Contains(t, warnMsg, configSourceNameUserVariable)
				assert.Contains(t, warnMsg, "nonexistent-helper")
			},
		},
		"job-supplied credHelpers with an uninstalled binary is skipped quietly": {
			dockerAuthValue: fmt.Sprintf(`{"credHelpers": {%q: "nonexistent-helper"}}`, registryDomain1Config.ServerAddress),
			image:           imageRegistryDomain1,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// The helper is not permitted, but its binary is not installed
				// either, so it never worked; skip without a warning to avoid
				// implying the runner newly broke a working setup.
				assert.NoError(t, err)
				assert.Nil(t, result)
				assert.Empty(t, logger.warningLogs)
				logger.ExpectLogs(t, [][]any{
					{`Ignoring credential helper "nonexistent-helper" from DOCKER_AUTH_CONFIG: it is not permitted by allowed_docker_credential_helpers and its binary "docker-credential-nonexistent-helper" is not installed`},
				})
			},
		},
		"job-supplied credsStore denied by default is skipped with a warning": {
			dockerAuthValue: fmt.Sprintf(`{"credsStore": "%s"}`, getValidCredentialHelperSuffix(t)),
			image:           imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// The binary is installed, so we are actually blocking something
				// that would have run: warn, but do not fail the job.
				assert.NoError(t, err)
				assert.Nil(t, result)
				require.Len(t, logger.warningLogs, 1)
				warnMsg := fmt.Sprint(logger.warningLogs[0]...)
				assert.Contains(t, warnMsg, "does not permit")
				assert.Contains(t, warnMsg, "for all registries")
				assert.Contains(t, warnMsg, fmt.Sprintf(`%q = ["<registry pattern>"]`, getValidCredentialHelperSuffix(t)))
			},
		},
		"credHelpers entry permitted by allowlist glob pattern": {
			// The first pattern must not match, and the second's hostname is
			// lowercased before matching, so this pins both the per-pattern loop
			// and the pattern normalization.
			dockerAuthValue:     getDockerAuthForCredentialHelperPathPath(t, "registry2.domain.tld:5005"),
			credHelperAllowlist: map[string][]string{getValidCredentialHelperSuffix(t): {"registry.other.tld", "*.DOMAIN.tld:5005"}},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				authConfig := registryScriptConfig
				authConfig.ServerAddress = "registry2.domain.tld:5005"
				expectedResult := &RegistryInfo{
					Path:       "registry2.domain.tld:5005",
					Source:     configSourceNameUserVariable,
					AuthConfig: authConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
				assert.Empty(t, logger.warningLogs)

				logger.ExpectLogs(t, [][]any{
					{`Loaded Docker credentials, source = "$DOCKER_AUTH_CONFIG", hostnames = [registry2.domain.tld:5005], error = <nil>`},
				})
			},
		},
		"credHelpers helper not in allowlist is skipped with an actionable warning": {
			dockerAuthValue:     getDockerAuthForCredentialHelperPathPath(t, "https://registry2.domain.tld:5005/v1/"),
			credHelperAllowlist: map[string][]string{"ecr-login": {"*"}},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				assert.NoError(t, err)
				assert.Nil(t, result)

				// The warning tells an operator the exact config.toml line to
				// add, and a non-operator the alternatives.
				require.Len(t, logger.warningLogs, 1)
				warnMsg := fmt.Sprint(logger.warningLogs[0]...)
				assert.Contains(t, warnMsg, "does not permit")
				assert.Contains(t, warnMsg, fmt.Sprintf(`%q = ["registry2.domain.tld:5005"]`, getValidCredentialHelperSuffix(t)))
				assert.Contains(t, warnMsg, "config.toml")
				assert.Contains(t, warnMsg, "ask your runner administrator")
			},
		},
		"credHelpers registry not permitted by allowlist is skipped with a warning": {
			dockerAuthValue:     getDockerAuthForCredentialHelperPathPath(t, "https://registry2.domain.tld:5005/v1/"),
			credHelperAllowlist: map[string][]string{getValidCredentialHelperSuffix(t): {"registry.other.tld"}},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				assert.NoError(t, err)
				assert.Nil(t, result)

				// Helper is listed, so the remedy is to widen the pattern, not
				// add a new helper entry.
				require.Len(t, logger.warningLogs, 1)
				warnMsg := fmt.Sprint(logger.warningLogs[0]...)
				assert.Contains(t, warnMsg, "permitted, but not for that registry")
				assert.Contains(t, warnMsg, fmt.Sprintf("add a pattern matching %q to %q", "registry2.domain.tld:5005", getValidCredentialHelperSuffix(t)))
			},
		},
		"disallowed credHelpers entry does not discard static auths in the same config": {
			dockerAuthValue: fmt.Sprintf(
				`{"auths": {"registry.domain.tld:5005": {"auth": %q}}, "credHelpers": {"registry2.domain.tld:5005": "%s"}}`,
				base64.StdEncoding.EncodeToString([]byte("test_user_1:test_password_1")),
				getValidCredentialHelperSuffix(t),
			),
			credHelperAllowlist: map[string][]string{"ecr-login": {"*"}},
			image:               imageRegistryDomain1,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// The disallowed helper is skipped, but the static auth for the
				// image being pulled must still resolve.
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, "registry.domain.tld:5005", result.Path)
				assert.Equal(t, "test_user_1", result.AuthConfig.Username)
				require.Len(t, logger.warningLogs, 1)
				assert.Contains(t, fmt.Sprint(logger.warningLogs[0]...), "does not permit")
			},
		},
		"home dir credsStore is not gated by the allowlist": {
			configFileContents:  fmt.Sprintf(`{"credsStore" : "%s"}`, getValidCredentialHelperSuffix(t)),
			credHelperAllowlist: map[string][]string{},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				authConfig := registryScriptConfig
				authConfig.ServerAddress = "https://registry2.domain.tld:5005/v1/"
				expectedResult := &RegistryInfo{
					Path:       "registry2.domain.tld:5005",
					Source:     filepath.Join(homeDir, ".dockercfg"),
					AuthConfig: authConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
				assert.Empty(t, logger.warningLogs)
			},
		},
		"home dir credHelpers is not gated by the allowlist": {
			configFileContents:  fmt.Sprintf(`{"credHelpers": {"registry.domain.tld:5005": "%s"}}`, getValidCredentialHelperSuffix(t)),
			credHelperAllowlist: map[string][]string{},
			image:               imageRegistryDomain1,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				authConfig := registryScriptConfig
				authConfig.ServerAddress = "registry.domain.tld:5005"
				expectedResult := &RegistryInfo{
					Path:       "registry.domain.tld:5005",
					Source:     filepath.Join(homeDir, ".dockercfg"),
					AuthConfig: authConfig,
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
				assert.Empty(t, logger.warningLogs)
			},
		},
		"allowlist pins helper to an absolute path outside PATH": {
			dockerAuthValue:     fmt.Sprintf(`{"credHelpers" : {"registry2.domain.tld:5005" : "%s"}}`, pinnedHelperName),
			credHelperAllowlist: map[string][]string{pinnedHelperPath: {"registry2.domain.tld:5005"}},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				expectedResult := &RegistryInfo{
					Path:   "registry2.domain.tld:5005",
					Source: configSourceNameUserVariable,
					AuthConfig: types.AuthConfig{
						Username:      "pinned_user_1",
						Password:      "pinned_password_1",
						ServerAddress: "registry2.domain.tld:5005",
					},
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
				assert.Empty(t, logger.warningLogs)
			},
		},
		"bare-name allowlist entry resolves via PATH, not the pinned binary": {
			dockerAuthValue:     fmt.Sprintf(`{"credHelpers" : {"registry2.domain.tld:5005" : "%s"}}`, pinnedHelperName),
			credHelperAllowlist: map[string][]string{pinnedHelperName: {"registry2.domain.tld:5005"}},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// A bare name is resolved as docker-credential-<name> on PATH.
				// The pinned binary lives outside PATH, so this must fail to
				// find it rather than execute it.
				assert.NoError(t, err)
				assert.Nil(t, result)

				require.Len(t, logger.warningLogs, 1)
				warnMsg := fmt.Sprint(logger.warningLogs[0]...)
				assert.Contains(t, warnMsg, "executable file not found")
			},
		},
		"allowlist key with a relative path is rejected": {
			dockerAuthValue:     testDockerAuthConfigs,
			credHelperAllowlist: map[string][]string{"./docker-credential-evil": {"*"}},
			image:               imageRegistryDomain1,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				assert.Nil(t, result)
				assert.ErrorContains(t, err, "allowed_docker_credential_helpers")
			},
		},
		"credHelpers registry with control characters is skipped with a warning": {
			dockerAuthValue: fmt.Sprintf(
				`{"credHelpers" : {"registry.domain.tld:5005\n.evil" : "%s"}}`, getValidCredentialHelperSuffix(t)),
			credHelperAllowlist: map[string][]string{getValidCredentialHelperSuffix(t): {"**"}},
			image:               imageRegistryDomain1,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// Even though the helper is allowed, the malformed registry name
				// is skipped rather than executed, and does not fail the job.
				assert.NoError(t, err)
				assert.Nil(t, result)
				require.Len(t, logger.warningLogs, 1)
				assert.Contains(t, fmt.Sprint(logger.warningLogs[0]...), "outside the permitted set")
			},
		},
		"credHelpers registry with Unicode whitespace is skipped with a warning": {
			dockerAuthValue: fmt.Sprintf(
				`{"credHelpers" : {"registry.domain.tld:5005\u2028.evil" : "%s"}}`, getValidCredentialHelperSuffix(t)),
			credHelperAllowlist: map[string][]string{getValidCredentialHelperSuffix(t): {"**"}},
			image:               imageRegistryDomain1,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// U+2028 is outside ASCII; the helper must still be skipped.
				assert.NoError(t, err)
				assert.Nil(t, result)
				require.Len(t, logger.warningLogs, 1)
				assert.Contains(t, fmt.Sprint(logger.warningLogs[0]...), "outside the permitted set")
			},
		},
		"credHelpers registry with userinfo is skipped with a warning": {
			dockerAuthValue: fmt.Sprintf(
				`{"credHelpers" : {"registry.domain.tld:5005@registry2.domain.tld:5005" : "%s"}}`, getValidCredentialHelperSuffix(t)),
			credHelperAllowlist: map[string][]string{getValidCredentialHelperSuffix(t): {"registry.domain.tld*"}},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// The glob accepts the raw key, but helpers and kubelet parse the
				// host after "@", so the key must be skipped, not handed to the helper.
				assert.NoError(t, err)
				assert.Nil(t, result)
				require.Len(t, logger.warningLogs, 1)
				assert.Contains(t, fmt.Sprint(logger.warningLogs[0]...), "outside the permitted set")
			},
		},
		"credHelpers entry with an empty registry key is skipped with a warning": {
			dockerAuthValue: fmt.Sprintf(
				`{"credHelpers" : {"" : "%s"}}`, credentialHelperSuffix(t, "listonly")),
			credHelperAllowlist: map[string][]string{credentialHelperSuffix(t, "listonly"): {"registry2.domain.tld:5005"}},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// listonly fails any "get", so a single empty-registry warning proves
				// the listed helper was never executed for the "" key.
				assert.NoError(t, err)
				assert.Nil(t, result)
				require.Len(t, logger.warningLogs, 1)
				assert.Contains(t, fmt.Sprint(logger.warningLogs[0]...), "the registry name is empty")
			},
		},
		"credsStore selective get skips disallowed registries entirely": {
			dockerAuthValue:     fmt.Sprintf(`{"credsStore" : "%s"}`, credentialHelperSuffix(t, "listonly")),
			credHelperAllowlist: map[string][]string{credentialHelperSuffix(t, "listonly"): {"registry.other.tld"}},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// The listonly helper fails any "get" action, so a warning-free
				// nil result proves "get" was never executed for the listed,
				// but disallowed, registry.
				assert.NoError(t, err)
				assert.Nil(t, result)
				assert.Empty(t, logger.warningLogs)
				require.Len(t, logger.debugLogs, 1)
				assert.Contains(t, fmt.Sprint(logger.debugLogs[0]...), "https://registry2.domain.tld:5005/v1/")
			},
		},
		"credsStore selective get retrieves allowed registries": {
			dockerAuthValue:     fmt.Sprintf(`{"credsStore" : "%s"}`, credentialHelperSuffix(t, "listonly")),
			credHelperAllowlist: map[string][]string{credentialHelperSuffix(t, "listonly"): {"registry2.domain.tld:5005"}},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				// The allowed registry is listed, so "get" executes and the
				// listonly helper's failure surfaces as a resolve warning.
				assert.NoError(t, err)
				assert.Nil(t, result)
				require.Len(t, logger.warningLogs, 1)
				warnMsg := fmt.Sprint(logger.warningLogs[0]...)
				assert.Contains(t, warnMsg, "error getting credentials")
			},
		},
		"credential helper identity token is preserved": {
			dockerAuthValue: fmt.Sprintf(
				`{"credHelpers" : {"registry2.domain.tld:5005" : "%s"}}`, credentialHelperSuffix(t, "token")),
			credHelperAllowlist: map[string][]string{credentialHelperSuffix(t, "token"): {"registry2.domain.tld:5005"}},
			image:               imageRegistryDomain2,
			checks: func(t *testing.T, result *RegistryInfo, err error, homeDir string, logger *fakeLogger) {
				expectedResult := &RegistryInfo{
					Path:   "registry2.domain.tld:5005",
					Source: configSourceNameUserVariable,
					AuthConfig: types.AuthConfig{
						IdentityToken: "script_token_1",
						ServerAddress: "registry2.domain.tld:5005",
					},
				}
				assert.NoError(t, err)
				assert.Equal(t, expectedResult, result)
				assert.Empty(t, logger.warningLogs)
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
			resolver := Resolver{
				homeDir:                func() string { return home },
				jobCredHelperAllowlist: tt.credHelperAllowlist,
			}
			logger := &fakeLogger{}

			regInfo, err := resolver.ConfigForImage(tt.image, tt.dockerAuthValue, "", tt.jobCredentials, logger)
			tt.checks(t, regInfo, err, home, logger)
		})
	}
}

func TestParseAllowlistKey(t *testing.T) {
	absPrefixed := func(name string) string {
		p, err := filepath.Abs(name)
		require.NoError(t, err)
		return p
	}

	tests := map[string]struct {
		key            string
		expectHelper   string
		expectProgram  string
		expectErrorSub string
	}{
		"bare name": {
			key:           "ecr-login",
			expectHelper:  "ecr-login",
			expectProgram: "docker-credential-ecr-login",
		},
		"absolute path pins the binary": {
			key:           absPrefixed("docker-credential-ecr-login"),
			expectHelper:  "ecr-login",
			expectProgram: absPrefixed("docker-credential-ecr-login"),
		},
		"absolute path with .exe derives the extensionless name": {
			key:           absPrefixed("docker-credential-ecr-login.exe"),
			expectHelper:  "ecr-login",
			expectProgram: absPrefixed("docker-credential-ecr-login.exe"),
		},
		"absolute path without the docker-credential prefix": {
			key:            absPrefixed("ecr-login"),
			expectErrorSub: "must be a binary named docker-credential-",
		},
		"relative path": {
			key:            "./docker-credential-ecr-login",
			expectErrorSub: "bare helper name or an absolute path",
		},
		"empty": {
			key:            "",
			expectErrorSub: "must not be empty",
		},
		"absolute path with uppercase .EXE derives the extensionless name": {
			key:           absPrefixed("docker-credential-ecr-login.EXE"),
			expectHelper:  "ecr-login",
			expectProgram: absPrefixed("docker-credential-ecr-login.EXE"),
		},
		"pinned name empty after .exe trim": {
			key:            absPrefixed("docker-credential-.exe"),
			expectErrorSub: "must be a binary named docker-credential-",
		},
		"pinned name is exactly the prefix": {
			key:            absPrefixed("docker-credential-"),
			expectErrorSub: "must be a binary named docker-credential-",
		},
		"pinned path with trailing separator": {
			key:            absPrefixed("docker-credential-ecr-login") + string(filepath.Separator),
			expectErrorSub: "must be a binary named docker-credential-",
		},
		"bare name carrying the binary prefix": {
			key:            "docker-credential-ecr-login",
			expectErrorSub: "not the binary name",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			helper, program, err := parseAllowlistKey(tt.key)
			if tt.expectErrorSub != "" {
				assert.ErrorContains(t, err, tt.expectErrorSub)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectHelper, helper)
			assert.Equal(t, tt.expectProgram, program)
		})
	}
}

func TestValidateAllowedHelpers(t *testing.T) {
	pinned, err := filepath.Abs("docker-credential-ecr-login")
	require.NoError(t, err)

	tests := map[string]struct {
		allowlist      map[string][]string
		expectErrorSub string
	}{
		"nil":                 {allowlist: nil},
		"empty map":           {allowlist: map[string][]string{}},
		"valid multi-pattern": {allowlist: map[string][]string{"ecr-login": {"*.dkr.ecr.*.amazonaws.com", "**"}}},
		"invalid glob":        {allowlist: map[string][]string{"ecr-login": {"registry[.example.com"}}, expectErrorSub: "invalid registry pattern"},
		"empty pattern":       {allowlist: map[string][]string{"ecr-login": {""}}, expectErrorSub: "invalid registry pattern"},
		"empty pattern list":  {allowlist: map[string][]string{"ecr-login": {}}, expectErrorSub: "at least one registry pattern"},
		"escaped IPv6 literal pattern": {
			allowlist: map[string][]string{"ipv6-helper": {`\[2001:db8::1\]:5000`}},
		},
		"same helper as bare name and pinned path": {
			allowlist: map[string][]string{"ecr-login": {"**"}, pinned: {"**"}}, expectErrorSub: "configured more than once",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateAllowedHelpers(tt.allowlist)
			if tt.expectErrorSub != "" {
				assert.ErrorContains(t, err, tt.expectErrorSub)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestCredHelperGateRegistryAllowedFailsClosed(t *testing.T) {
	// An operator/home-dir source uses ungated, which allows every registry.
	assert.True(t, ungated{}.registryAllowed("helper", "registry.example.com"))

	// A configured gate with no allowlist denies every registry, rather than
	// failing open, even though permit already blocks reaching this path.
	unconfigured, err := newAllowlistGate(nil, &fakeLogger{})
	require.NoError(t, err)
	assert.False(t, unconfigured.registryAllowed("helper", "registry.example.com"))
}

func TestMatchesAllowedRegistryIPv6Literal(t *testing.T) {
	const registry = "[2001:db8::1]:5000"

	// The glob syntax reads "[" and "]" as a character class, so an operator
	// must escape them to name an IPv6 registry.
	assert.True(t, matchesAllowedRegistry([]string{`\[2001:db8::1\]:5000`}, registry))
	assert.True(t, matchesAllowedRegistry([]string{`\[2001:db8::1\]:5000`}, "https://[2001:db8::1]:5000/v1/"))
	assert.True(t, matchesAllowedRegistry([]string{`\[2001:db8::1\]:5000/team/**`}, registry+"/team/img"))

	// The escaped pattern is exact: it admits no other name of the same shape.
	assert.False(t, matchesAllowedRegistry([]string{`\[2001:db8::1\]:5000`}, "d:5000"))
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
		jobCreds         []spec.Credentials
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
		"registry.local/ns/some/image@sha256:" + strings.Repeat("a", 64): {
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
			jobCreds: []spec.Credentials{{
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
			jobCreds: []spec.Credentials{{
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
			resolver := Resolver{
				homeDir: func() string { return "" },
			}

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

func TestResolver_AllConfigs(t *testing.T) {
	home := setupTestHomeDirectoryConfig(t, testFileAuthConfigs)
	resolver := Resolver{
		homeDir: func() string { return home },
	}
	logger := &fakeLogger{}

	result, err := resolver.AllConfigs(testDockerAuthConfigs, "", gitlabRegistryCredentials, logger)
	assert.NoError(t, err)

	assert.Equal(t, RegistryInfos{
		{
			Path:   "registry.domain.tld:5005",
			Source: configSourceNameUserVariable,
			AuthConfig: types.AuthConfig{
				Username:      "test_user_1",
				Password:      "test_password_1",
				ServerAddress: "https://registry.domain.tld:5005/v1/",
			},
		},
		{
			Path:   "registry2.domain.tld:5005",
			Source: filepath.Join(home, ".dockercfg"),
			AuthConfig: types.AuthConfig{
				Username:      "test_user_2",
				Password:      "test_password_2",
				ServerAddress: "registry2.domain.tld:5005",
			},
		},
		{
			Path:   "registry.gitlab.tld:1234",
			Source: configSourceNameJobPayload,
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
	registryCredentials := []spec.Credentials{
		{
			Type:     "registry",
			URL:      "registry.domain.tld:5005",
			Username: "test_user_1",
			Password: "test_password_1",
		},
	}

	home := setupTestHomeDirectoryConfig(t, testFileAuthConfigs)
	resolver := Resolver{
		homeDir: func() string { return home },
	}
	logger := &fakeLogger{}

	result, err := resolver.AllConfigs("", "", registryCredentials, logger)
	assert.NoError(t, err)

	expectedResult := RegistryInfos{
		{
			Path:   "registry.domain.tld:5005",
			Source: filepath.Join(home, ".dockercfg"),
			AuthConfig: types.AuthConfig{
				Username:      "test_user_1",
				Password:      "test_password_1",
				ServerAddress: "https://registry.domain.tld:5005/v1/",
			},
		},
		{
			Path:   "registry2.domain.tld:5005",
			Source: filepath.Join(home, ".dockercfg"),
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

func TestDockerImagePathNormalizationWithDigest(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	tests := map[string]string{
		"registry.local/ns/image@" + digest:                       "registry.local/ns/image",
		"registry.local/ns/image:latest@" + digest:                "registry.local/ns/image",
		"REGISTRY.local:5000/With/Case@" + digest:                 "registry.local:5000/With/Case",
		"localhost:5000/image@" + digest:                          "localhost:5000/image",
		"ubuntu@" + digest:                                        "docker.io/ubuntu",
		"user/ubuntu@" + digest:                                   "docker.io/user/ubuntu",
		"index.DOCKER.io/user/ubuntu@" + digest:                   "docker.io/user/ubuntu",
		"registry.local/image@sha512:" + strings.Repeat("a", 128): "registry.local/image",
	}

	for imageRef, expected := range tests {
		t.Run(imageRef, func(t *testing.T) {
			assert.Equal(t, expected, normalizeImageRef(imageRef))
		})
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
	if runtime.GOOS == "windows" {
		return credentialHelperSuffix(t, "windows")
	}
	return credentialHelperSuffix(t, "bin")
}

// credentialHelperSuffix returns the OS-specific helper name for one of the
// testdata docker-credential-<name>.{sh,cmd} scripts.
func credentialHelperSuffix(t *testing.T, name string) string {
	switch runtime.GOOS {
	case "windows":
		return name + ".cmd"
	case "linux", "darwin":
		return name + ".sh"
	default:
		assert.FailNow(t, "credentials helper and credentials store tests are not supported on "+runtime.GOOS)
	}
	return ""
}

// getPinnedCredentialHelper returns the helper name a job would reference and
// the absolute path of a helper binary that is not on PATH, so it is only
// executable when pinned through the allowlist.
func getPinnedCredentialHelper(t *testing.T) (helperName, absPath string) {
	dir, err := os.Getwd()
	require.NoError(t, err)

	helperName = credentialHelperSuffix(t, "pinned")
	return helperName, filepath.Join(dir, "testdata", "pinned", "docker-credential-"+helperName)
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

func TestReadDockerAuthConfigsFromHomeDir_NoUsername(t *testing.T) {
	expectedUsername := "test_username"
	expectedPassword := "test_password"
	expectedServerAddr := "https://index.docker.io/v1/"

	auth := base64.StdEncoding.EncodeToString([]byte(expectedUsername + ":" + expectedPassword))

	config := []byte(`{"auths": {"` + expectedServerAddr + `": {"auth": "` + auth + `"}}}`)

	tests := map[string]struct {
		homeDirProvided     bool
		configContent       []byte
		configLocation      []string
		expectedAuthConfigs []types.AuthConfig
		expectedError       error
	}{
		"Home dir value is blank": {
			expectedError: errNoHomeDir,
		},
		"No configs": {
			homeDirProvided:     true,
			expectedAuthConfigs: []types.AuthConfig{},
		},
		"Config: $HOME/.dockercfg": {
			homeDirProvided: true,
			configContent:   config,
			configLocation:  []string{".dockercfg"},
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
			configLocation:  []string{".docker", "config.json"},
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
			fakeHome := ""
			expectedConfigFile := ""

			if test.homeDirProvided {
				fakeHome = t.TempDir()

				if len(test.configLocation) > 0 {
					file := filepath.Join(slices.Concat([]string{fakeHome}, test.configLocation)...)
					require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o777), "creating config directory")
					require.NoError(t, os.WriteFile(file, test.configContent, 0o666), "writing config file")
					expectedConfigFile = file
				}
			}

			resolver := Resolver{
				homeDir: func() string { return fakeHome },
			}

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
		"auths": {},
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

type fakeLogger struct {
	debugLogs   [][]any
	warningLogs [][]any
}

func (l *fakeLogger) Debugln(args ...any) {
	l.debugLogs = append(l.debugLogs, args)
}

func (l *fakeLogger) Warningln(args ...any) {
	l.warningLogs = append(l.warningLogs, args)
}

func (l *fakeLogger) ExpectLogs(t *testing.T, expectedLogs [][]any) {
	t.Helper()

	le := len(expectedLogs)
	la := len(l.debugLogs)

	assert.Len(t, l.debugLogs, le, "expected %d debug logs, got %d", le, la)

	for i := 0; i < min(le, la); i++ {
		assert.Equal(t, expectedLogs[i], l.debugLogs[i], "debug log line %d", i)
	}
}

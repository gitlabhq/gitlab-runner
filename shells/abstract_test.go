//go:build !integration
// +build !integration

package shells

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	_ "gitlab.com/gitlab-org/gitlab-runner/cache/test"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls"
)

func TestWriteGitSSLConfig(t *testing.T) {
	expectedURL := "https://example.com:3443"

	shell := AbstractShell{}
	build := &common.Build{
		Runner: &common.RunnerConfig{},
		JobResponse: common.JobResponse{
			GitInfo: common.GitInfo{
				RepoURL: "https://gitlab-ci-token:xxx@example.com:3443/project/repo.git",
			},
			TLSAuthCert: "TLS_CERT",
			TLSAuthKey:  "TLS_KEY",
			TLSCAChain:  "CA_CHAIN",
		},
	}

	mockWriter := new(MockShellWriter)
	mockWriter.On("EnvVariableKey", tls.VariableCAFile).Return("VariableCAFile").Once()
	mockWriter.On("EnvVariableKey", tls.VariableCertFile).Return("VariableCertFile").Once()
	mockWriter.On("EnvVariableKey", tls.VariableKeyFile).Return("VariableKeyFile").Once()

	mockWriter.On(
		"Command",
		"git",
		"config",
		fmt.Sprintf("http.%s.%s", expectedURL, "sslCAInfo"),
		"VariableCAFile",
	).Once()
	mockWriter.On(
		"Command",
		"git",
		"config",
		fmt.Sprintf("http.%s.%s", expectedURL, "sslCert"),
		"VariableCertFile",
	).Once()
	mockWriter.On(
		"Command",
		"git",
		"config",
		fmt.Sprintf("http.%s.%s", expectedURL, "sslKey"),
		"VariableKeyFile",
	).Once()

	shell.writeGitSSLConfig(mockWriter, build, nil)

	mockWriter.AssertExpectations(t)
}

func getJobResponseWithMultipleArtifacts() common.JobResponse {
	return common.JobResponse{
		ID:    1000,
		Token: "token",
		Artifacts: common.Artifacts{
			common.Artifact{
				Paths: []string{"default"},
			},
			common.Artifact{
				Paths: []string{"on-success"},
				When:  common.ArtifactWhenOnSuccess,
			},
			common.Artifact{
				Paths: []string{"on-failure"},
				When:  common.ArtifactWhenOnFailure,
			},
			common.Artifact{
				Paths: []string{"always"},
				When:  common.ArtifactWhenAlways,
			},
			common.Artifact{
				Paths:  []string{"zip-archive"},
				When:   common.ArtifactWhenAlways,
				Format: common.ArtifactFormatZip,
				Type:   "archive",
			},
			common.Artifact{
				Paths:  []string{"gzip-junit"},
				When:   common.ArtifactWhenAlways,
				Format: common.ArtifactFormatGzip,
				Type:   "junit",
			},
		},
	}
}

func TestWriteWritingArtifactsOnSuccess(t *testing.T) {
	gitlabURL := "https://example.com:3443"

	shell := AbstractShell{}
	build := &common.Build{
		JobResponse: getJobResponseWithMultipleArtifacts(),
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				URL: gitlabURL,
			},
		},
	}
	info := common.ShellScriptInfo{
		RunnerCommand: "gitlab-runner-helper",
		Build:         build,
	}

	mockWriter := new(MockShellWriter)
	defer mockWriter.AssertExpectations(t)
	mockWriter.On("Variable", mock.Anything)
	mockWriter.On("Cd", mock.Anything)
	mockWriter.On("IfCmd", "gitlab-runner-helper", "--version")
	mockWriter.On("Noticef", mock.Anything)
	mockWriter.On(
		"Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "default",
	).Once()
	mockWriter.On(
		"Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "on-success",
	).Once()
	mockWriter.On(
		"Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "always",
	).Once()
	mockWriter.On(
		"Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "zip-archive",
		"--artifact-format", "zip",
		"--artifact-type", "archive",
	).Once()
	mockWriter.On(
		"Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "gzip-junit",
		"--artifact-format", "gzip",
		"--artifact-type", "junit",
	).Once()
	mockWriter.On("Else")
	mockWriter.On("Warningf", mock.Anything, mock.Anything, mock.Anything)
	mockWriter.On("EndIf")

	err := shell.writeScript(mockWriter, common.BuildStageUploadOnSuccessArtifacts, info)
	require.NoError(t, err)
}

func TestWriteWritingArtifactsOnFailure(t *testing.T) {
	gitlabURL := "https://example.com:3443"

	shell := AbstractShell{}
	build := &common.Build{
		JobResponse: getJobResponseWithMultipleArtifacts(),
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				URL: gitlabURL,
			},
		},
	}
	info := common.ShellScriptInfo{
		RunnerCommand: "gitlab-runner-helper",
		Build:         build,
	}

	mockWriter := new(MockShellWriter)
	defer mockWriter.AssertExpectations(t)
	mockWriter.On("Variable", mock.Anything)
	mockWriter.On("Cd", mock.Anything)
	mockWriter.On("IfCmd", "gitlab-runner-helper", "--version")
	mockWriter.On("Noticef", mock.Anything)
	mockWriter.On(
		"Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "on-failure",
	).Once()
	mockWriter.On(
		"Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "always",
	).Once()
	mockWriter.On(
		"Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "zip-archive",
		"--artifact-format", "zip",
		"--artifact-type", "archive",
	).Once()
	mockWriter.On(
		"Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "gzip-junit",
		"--artifact-format", "gzip",
		"--artifact-type", "junit",
	).Once()
	mockWriter.On("Else")
	mockWriter.On("Warningf", mock.Anything, mock.Anything, mock.Anything)
	mockWriter.On("EndIf")

	err := shell.writeScript(mockWriter, common.BuildStageUploadOnFailureArtifacts, info)
	require.NoError(t, err)
}

func TestWriteWritingArtifactsWithExcludedPaths(t *testing.T) {
	shell := AbstractShell{}

	build := &common.Build{
		JobResponse: common.JobResponse{
			ID:    1001,
			Token: "token",
			Artifacts: common.Artifacts{
				common.Artifact{
					Paths:   []string{"include/**"},
					Exclude: []string{"include/exclude/*"},
					When:    common.ArtifactWhenAlways,
					Format:  common.ArtifactFormatZip,
					Type:    "archive",
				},
			},
		},
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				URL: "https://gitlab.example.com",
			},
		},
	}

	info := common.ShellScriptInfo{
		RunnerCommand: "gitlab-runner-helper",
		Build:         build,
	}

	mockWriter := new(MockShellWriter)
	defer mockWriter.AssertExpectations(t)
	mockWriter.On("Variable", mock.Anything)
	mockWriter.On("Cd", mock.Anything).Once()
	mockWriter.On("IfCmd", "gitlab-runner-helper", "--version").Once()
	mockWriter.On("Noticef", mock.Anything).Once()
	mockWriter.On(
		"Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", "https://gitlab.example.com",
		"--token", "token",
		"--id", "1001",
		"--path", "include/**",
		"--exclude", "include/exclude/*",
		"--artifact-format", "zip",
		"--artifact-type", "archive",
	).Once()
	mockWriter.On("Else").Once()
	mockWriter.On("Warningf", mock.Anything, mock.Anything, mock.Anything).Once()
	mockWriter.On("EndIf").Once()

	err := shell.writeScript(mockWriter, common.BuildStageUploadOnSuccessArtifacts, info)
	require.NoError(t, err)
}

func getJobResponseWithCachePaths() common.JobResponse {
	return common.JobResponse{
		ID:    1000,
		Token: "token",
		Cache: common.Caches{
			common.Cache{
				Key:       "cache-key1",
				Untracked: true,
				Policy:    common.CachePolicyPush,
				Paths:     []string{"vendor/"},
				When:      common.CacheWhenOnSuccess,
			},
			common.Cache{
				Key:    "cache-key1",
				Policy: common.CachePolicyPush,
				Paths:  []string{"some/path1", "other/path2"},
				When:   common.CacheWhenOnSuccess,
			},
			common.Cache{
				Key:       "cache-key1",
				Untracked: true,
				Policy:    common.CachePolicyPush,
				Paths:     []string{"when-on-failure"},
				When:      common.CacheWhenOnFailure,
			},
			common.Cache{
				Key:    "cache-key1",
				Policy: common.CachePolicyPush,
				Paths:  []string{"when-always"},
				When:   common.CacheWhenAlways,
			},
		},
	}
}

func TestWriteWritingArchiveCacheOnSuccess(t *testing.T) {
	gitlabURL := "https://example.com:3443"

	shell := AbstractShell{}

	tests := map[string]struct {
		cacheType string
	}{
		"pre-signed URL cache": {
			cacheType: "test",
		},

		"GoCloud cache": {
			cacheType: "goCloudTest",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			runnerConfig := &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Cache: &common.CacheConfig{
						Type:   tt.cacheType,
						Shared: true,
					},
				},
				RunnerCredentials: common.RunnerCredentials{
					URL: gitlabURL,
				},
			}

			build := &common.Build{
				CacheDir:    "cache_dir",
				JobResponse: getJobResponseWithCachePaths(),
				Runner:      runnerConfig,
			}
			info := common.ShellScriptInfo{
				RunnerCommand: "gitlab-runner-helper",
				Build:         build,
			}

			mockWriter := new(MockShellWriter)
			defer mockWriter.AssertExpectations(t)
			mockWriter.On("Cd", mock.Anything)
			mockWriter.On("IfCmd", "gitlab-runner-helper", "--version")
			mockWriter.On("Noticef", "Creating cache %s...", mock.Anything).Times(3)

			if tt.cacheType == "test" {
				mockWriter.On(
					"IfCmdWithOutput", "gitlab-runner-helper", "cache-archiver",
					"--file", mock.Anything,
					"--timeout", mock.Anything,
					"--path", "vendor/",
					"--untracked",
					"--url", mock.Anything,
					"--header", "Header-1: a value",
				).Once()
				mockWriter.On(
					"IfCmdWithOutput", "gitlab-runner-helper", "cache-archiver",
					"--file", mock.Anything,
					"--timeout", mock.Anything,
					"--path", "some/path1",
					"--path", "other/path2",
					"--url", mock.Anything,
					"--header", "Header-1: a value",
				).Once()
				mockWriter.On(
					"IfCmdWithOutput", "gitlab-runner-helper", "cache-archiver",
					"--file", mock.Anything,
					"--timeout", mock.Anything,
					"--path", "when-always",
					"--url", mock.Anything,
					"--header", "Header-1: a value",
				).Once()
			} else {
				mockWriter.On("Variable", mock.MatchedBy(func(v common.JobVariable) bool {
					return v.Key == "FIRST_VAR" && v.Value == "123"
				})).Once()
				mockWriter.On("Variable", mock.MatchedBy(func(v common.JobVariable) bool {
					return v.Key == "SECOND_VAR" && v.Value == "456"
				})).Once()
				mockWriter.On(
					"IfCmdWithOutput", "gitlab-runner-helper", "cache-archiver",
					"--file", mock.Anything,
					"--timeout", mock.Anything,
					"--path", "vendor/",
					"--untracked",
					"--gocloud-url", mock.Anything,
				).Once()
				mockWriter.On(
					"IfCmdWithOutput", "gitlab-runner-helper", "cache-archiver",
					"--file", mock.Anything,
					"--timeout", mock.Anything,
					"--path", "some/path1",
					"--path", "other/path2",
					"--gocloud-url", mock.Anything,
				).Once()
				mockWriter.On(
					"IfCmdWithOutput", "gitlab-runner-helper", "cache-archiver",
					"--file", mock.Anything,
					"--timeout", mock.Anything,
					"--path", "when-always",
					"--gocloud-url", mock.Anything,
				).Once()
			}
			mockWriter.On("Noticef", "Created cache").Times(3)
			mockWriter.On("Else").Times(3)
			mockWriter.On("Warningf", "Failed to create cache").Times(3)
			mockWriter.On("EndIf").Times(3)
			mockWriter.On("Else").Times(3)
			mockWriter.On("Warningf", mock.Anything, mock.Anything, mock.Anything).Times(3)
			mockWriter.On("EndIf").Times(3)
			mockWriter.On("Variable", mock.Anything)

			err := shell.writeScript(mockWriter, common.BuildStageArchiveOnSuccessCache, info)
			require.NoError(t, err)
		})
	}
}

func TestWriteWritingArchiveCacheOnFailure(t *testing.T) {
	gitlabURL := "https://example.com:3443"

	shell := AbstractShell{}

	tests := map[string]struct {
		cacheType string
	}{
		"pre-signed URL cache": {
			cacheType: "test",
		},

		"GoCloud cache": {
			cacheType: "goCloudTest",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			runnerConfig := &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Cache: &common.CacheConfig{
						Type:   tt.cacheType,
						Shared: true,
					},
				},
				RunnerCredentials: common.RunnerCredentials{
					URL: gitlabURL,
				},
			}

			build := &common.Build{
				CacheDir:    "cache_dir",
				JobResponse: getJobResponseWithCachePaths(),
				Runner:      runnerConfig,
			}
			info := common.ShellScriptInfo{
				RunnerCommand: "gitlab-runner-helper",
				Build:         build,
			}

			mockWriter := new(MockShellWriter)
			defer mockWriter.AssertExpectations(t)
			mockWriter.On("Cd", mock.Anything)
			mockWriter.On("IfCmd", "gitlab-runner-helper", "--version")
			mockWriter.On("Noticef", "Creating cache %s...", mock.Anything).Times(2)

			if tt.cacheType == "test" {
				mockWriter.On(
					"IfCmdWithOutput", "gitlab-runner-helper", "cache-archiver",
					"--file", mock.Anything,
					"--timeout", mock.Anything,
					"--path", "when-on-failure",
					"--untracked",
					"--url", mock.Anything,
					"--header", "Header-1: a value",
				).Once()
				mockWriter.On(
					"IfCmdWithOutput", "gitlab-runner-helper", "cache-archiver",
					"--file", mock.Anything,
					"--timeout", mock.Anything,
					"--path", "when-always",
					"--url", mock.Anything,
					"--header", "Header-1: a value",
				).Once()
			} else {
				mockWriter.On("Variable", mock.MatchedBy(func(v common.JobVariable) bool {
					return v.Key == "FIRST_VAR" && v.Value == "123"
				})).Once()
				mockWriter.On("Variable", mock.MatchedBy(func(v common.JobVariable) bool {
					return v.Key == "SECOND_VAR" && v.Value == "456"
				})).Once()
				mockWriter.On(
					"IfCmdWithOutput", "gitlab-runner-helper", "cache-archiver",
					"--file", mock.Anything,
					"--timeout", mock.Anything,
					"--path", "when-on-failure",
					"--untracked",
					"--gocloud-url", mock.Anything,
				).Once()
				mockWriter.On(
					"IfCmdWithOutput", "gitlab-runner-helper", "cache-archiver",
					"--file", mock.Anything,
					"--timeout", mock.Anything,
					"--path", "when-always",
					"--gocloud-url", mock.Anything,
				).Once()
			}
			mockWriter.On("Noticef", "Created cache").Times(2)
			mockWriter.On("Else").Times(2)
			mockWriter.On("Warningf", "Failed to create cache").Times(2)
			mockWriter.On("EndIf").Times(2)
			mockWriter.On("Else").Times(2)
			mockWriter.On("Warningf", mock.Anything, mock.Anything, mock.Anything).Times(2)
			mockWriter.On("EndIf").Times(2)
			mockWriter.On("Variable", mock.Anything)

			err := shell.writeScript(mockWriter, common.BuildStageArchiveOnFailureCache, info)
			require.NoError(t, err)
		})
	}
}

func TestGitCleanFlags(t *testing.T) {
	tests := map[string]struct {
		value string

		expectedGitClean      bool
		expectedGitCleanFlags []interface{}
	}{
		"empty clean flags": {
			value:                 "",
			expectedGitClean:      true,
			expectedGitCleanFlags: []interface{}{"-ffdx"},
		},
		"use custom flags": {
			value:                 "custom-flags",
			expectedGitClean:      true,
			expectedGitCleanFlags: []interface{}{"custom-flags"},
		},
		"use custom flags with multiple arguments": {
			value:                 "-ffdx -e cache/",
			expectedGitClean:      true,
			expectedGitCleanFlags: []interface{}{"-ffdx", "-e", "cache/"},
		},
		"disabled": {
			value:            "none",
			expectedGitClean: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			shell := AbstractShell{}

			const dummySha = "01234567abcdef"
			const dummyRef = "main"

			build := &common.Build{
				Runner: &common.RunnerConfig{},
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{Sha: dummySha, Ref: dummyRef},
					Variables: common.JobVariables{
						{Key: "GIT_CLEAN_FLAGS", Value: test.value},
					},
				},
			}

			mockWriter := new(MockShellWriter)
			defer mockWriter.AssertExpectations(t)

			mockWriter.On("Noticef", "Checking out %s as %s...", dummySha[0:8], dummyRef).Once()
			mockWriter.On("Command", "git", "checkout", "-f", "-q", dummySha).Once()

			if test.expectedGitClean {
				command := []interface{}{"git", "clean"}
				command = append(command, test.expectedGitCleanFlags...)
				mockWriter.On("Command", command...).Once()
			}

			shell.writeCheckoutCmd(mockWriter, build)
		})
	}
}

func TestGitFetchFlags(t *testing.T) {
	tests := map[string]struct {
		value string

		expectedGitFetchFlags []interface{}
	}{
		"empty fetch flags": {
			value:                 "",
			expectedGitFetchFlags: []interface{}{"--prune", "--quiet"},
		},
		"use custom flags": {
			value:                 "--prune",
			expectedGitFetchFlags: []interface{}{"--prune"},
		},
		"disabled": {
			value: "none",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			shell := AbstractShell{}

			const dummySha = "01234567abcdef"
			const dummyRef = "main"
			const dummyProjectDir = "./"

			build := &common.Build{
				Runner: &common.RunnerConfig{},
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{Sha: dummySha, Ref: dummyRef, Depth: 0},
					Variables: common.JobVariables{
						{Key: "GIT_FETCH_EXTRA_FLAGS", Value: test.value},
					},
				},
			}

			mockWriter := new(MockShellWriter)
			defer mockWriter.AssertExpectations(t)

			mockWriter.On("Noticef", "Fetching changes...").Once()
			mockWriter.On("MkTmpDir", mock.Anything).Return(mock.Anything).Once()
			mockWriter.On("Command", "git", "config", "-f", mock.Anything, "fetch.recurseSubmodules", "false").Once()
			mockWriter.On("Command", "git", "init", dummyProjectDir, "--template", mock.Anything).Once()
			mockWriter.On("Cd", mock.Anything)
			mockWriter.On("Join", mock.Anything, mock.Anything).Return(mock.Anything).Once()
			mockWriter.On("IfCmd", "git", "remote", "add", "origin", mock.Anything)
			mockWriter.On("RmFile", mock.Anything)
			mockWriter.On("Noticef", "Created fresh repository.").Once()
			mockWriter.On("Else")
			mockWriter.On("Command", "git", "remote", "set-url", "origin", mock.Anything)
			mockWriter.On("EndIf")

			v := common.AppVersion
			userAgent := fmt.Sprintf("http.userAgent=%s %s %s/%s", v.Name, v.Version, v.OS, v.Architecture)
			command := []interface{}{"git", "-c", userAgent, "fetch", "origin"}

			command = append(command, test.expectedGitFetchFlags...)
			mockWriter.On("Command", command...)

			shell.writeRefspecFetchCmd(mockWriter, build, dummyProjectDir)
		})
	}
}

func TestAbstractShell_writeSubmoduleUpdateCmd(t *testing.T) {
	tests := map[string]struct {
		Recursive               bool
		Depth                   int
		ExpectedNoticeArgs      []interface{}
		ExpectedGitUpdateFlags  []interface{}
		ExpectedGitForEachFlags []interface{}
	}{
		"no recursion, no depth limit": {
			Recursive:          false,
			Depth:              0,
			ExpectedNoticeArgs: []interface{}{"Updating/initializing submodules..."},
		},
		"no recursion, depth limit 10": {
			Recursive:               false,
			Depth:                   10,
			ExpectedNoticeArgs:      []interface{}{"Updating/initializing submodules with git depth set to %d...", 10},
			ExpectedGitUpdateFlags:  []interface{}{"--depth", "10"},
			ExpectedGitForEachFlags: []interface{}{},
		},
		"with recursion, no depth limit": {
			Recursive:               true,
			Depth:                   0,
			ExpectedNoticeArgs:      []interface{}{"Updating/initializing submodules recursively..."},
			ExpectedGitUpdateFlags:  []interface{}{"--recursive"},
			ExpectedGitForEachFlags: []interface{}{"--recursive"},
		},
		"with recursion, depth limit 1": {
			Recursive: true,
			Depth:     1,
			ExpectedNoticeArgs: []interface{}{
				"Updating/initializing submodules recursively with git depth set to %d...",
				1,
			},
			ExpectedGitUpdateFlags:  []interface{}{"--recursive", "--depth", "1"},
			ExpectedGitForEachFlags: []interface{}{"--recursive"},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			shell := AbstractShell{}
			mockWriter := new(MockShellWriter)
			defer mockWriter.AssertExpectations(t)
			expectedGitForEachArgsFn := func() []interface{} {
				return append(
					[]interface{}{"git", "submodule", "foreach"},
					tc.ExpectedGitForEachFlags...,
				)
			}
			mockWriter.On("Noticef", tc.ExpectedNoticeArgs...).Once()
			mockWriter.
				On(
					"Command",
					append([]interface{}{"git", "submodule", "sync"}, tc.ExpectedGitForEachFlags...)...,
				).Once()
			mockWriter.
				On(
					"Command",
					append([]interface{}{"git", "submodule", "update", "--init"}, tc.ExpectedGitUpdateFlags...)...,
				).Once()
			mockWriter.
				On("Command", append(expectedGitForEachArgsFn(), "git clean -ffxd")...).
				Once()
			mockWriter.On("Command", append(expectedGitForEachArgsFn(), "git reset --hard")...).Once()
			mockWriter.On("IfCmd", "git", "lfs", "version").Once()
			mockWriter.On("Command", append(expectedGitForEachArgsFn(), "git lfs pull")...).Once()
			mockWriter.On("EndIf").Once()

			shell.writeSubmoduleUpdateCmd(
				mockWriter,
				&common.Build{
					JobResponse: common.JobResponse{
						GitInfo: common.GitInfo{Depth: tc.Depth},
					},
				},
				tc.Recursive,
			)
		})
	}
}

func TestAbstractShell_extractCacheWithFallbackKey(t *testing.T) {
	testCacheCommon(t, false)
}

func TestAbstractShell_extractCacheWithFallbackKeyRequired(t *testing.T) {
	testCacheCommon(t, true)
}

func testCacheCommon(t *testing.T, cacheRequired bool) {
	testCacheKey := "test-cache-key"
	testFallbackCacheKey := "test-fallback-cache-key"

	shell := AbstractShell{}
	runnerConfig := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Cache: &common.CacheConfig{
				Type:   "test",
				Shared: true,
			},
		},
	}
	build := &common.Build{
		BuildDir: "/builds",
		CacheDir: "/cache",
		Runner:   runnerConfig,
		JobResponse: common.JobResponse{
			ID: 1000,
			JobInfo: common.JobInfo{
				ProjectID: 1000,
			},
			Cache: common.Caches{
				{
					Key:      testCacheKey,
					Policy:   common.CachePolicyPullPush,
					Paths:    []string{"path1", "path2"},
					Required: cacheRequired,
				},
			},
			Variables: common.JobVariables{
				{
					Key:   "CACHE_FALLBACK_KEY",
					Value: testFallbackCacheKey,
				},
			},
		},
	}
	info := common.ShellScriptInfo{
		RunnerCommand: "runner-command",
		Build:         build,
	}

	mockWriter := new(MockShellWriter)
	defer mockWriter.AssertExpectations(t)

	mockWriter.On("IfCmd", "runner-command", "--version").Once()
	mockWriter.On("Noticef", "Checking cache for %s...", testCacheKey).Once()
	mockWriter.On(
		"IfCmdWithOutput",
		"runner-command",
		"cache-extractor",
		"--file",
		filepath.Join("..", build.CacheDir, testCacheKey, "cache.zip"),
		"--timeout",
		"10",
		"--url",
		fmt.Sprintf("test://download/project/1000/%s", testCacheKey),
	).Once()
	mockWriter.On("Noticef", "Successfully extracted cache").Once()
	mockWriter.On("Else").Once()
	mockWriter.On("Warningf", "Failed to extract cache").Once()
	mockWriter.On("Noticef", "Checking cache for %s...", testFallbackCacheKey).Once()
	mockWriter.On(
		"IfCmdWithOutput",
		"runner-command",
		"cache-extractor",
		"--file",
		filepath.Join("..", build.CacheDir, testCacheKey, "cache.zip"),
		"--timeout",
		"10",
		"--url",
		fmt.Sprintf("test://download/project/1000/%s", testFallbackCacheKey),
	).Once()
	mockWriter.On("Noticef", "Successfully extracted cache").Once()
	mockWriter.On("Else").Once()
	mockWriter.On("Warningf", "Failed to extract cache").Once()
	mockWriter.On("EndIf").Once()
	mockWriter.On("EndIf").Once()
	mockWriter.On("Else").Once()
	mockWriter.On("Warningf", "Missing %s. %s is disabled.", "runner-command", "Extracting cache").Once()

	if cacheRequired {
		mockWriter.On("Exit", 1).Once()
	}
	mockWriter.On("EndIf").Once()

	err := shell.cacheExtractor(mockWriter, info)
	assert.NoError(t, err)
}

func TestAbstractShell_writeSubmoduleUpdateCmdPath(t *testing.T) {
	tests := map[string]struct {
		paths string
	}{
		"single path": {
			paths: "submoduleA",
		},
		"multiple paths": {
			paths: "submoduleA submoduleB submoduleC",
		},
		"exclude paths": {
			paths: ":(exclude)submoduleA :(exclude)submoduleB",
		},
		"paths with dash": {
			paths: "-submoduleA :(exclude)-submoduleB",
		},
		"invalid paths": {
			paths: "submoduleA : (exclude)submoduleB submoduleC :::1(exclude) submoduleD",
		},
		"extra spaces": {
			paths: "submoduleA :   (exclude)submoduleB    submoduleC :::1(exclude)   submoduleD",
		},
		"empty paths": {
			paths: "",
		},
		"spaces": {
			paths: "        ",
		},
	}

	submoduleCommand := func(paths string, args ...string) []interface{} {
		command := []interface{}{
			"git",
			"submodule",
		}

		for _, a := range args {
			command = append(command, a)
		}

		paths = strings.TrimSpace(paths)
		if paths != "" {
			command = append(command, "--", paths)
		}

		return command
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			shell := AbstractShell{}
			mockWriter := new(MockShellWriter)
			defer mockWriter.AssertExpectations(t)

			mockWriter.On("Noticef", mock.Anything).Once()
			mockWriter.On("Command", submoduleCommand(test.paths, "sync")...).Once()
			mockWriter.On("Command", submoduleCommand(test.paths, "update", "--init")...).Once()
			mockWriter.On("Command", "git", "submodule", "foreach", "git clean -ffxd").Once()
			mockWriter.On("Command", "git", "submodule", "foreach", "git reset --hard").Once()
			mockWriter.On("IfCmd", "git", "lfs", "version").Once()
			mockWriter.On("Command", "git", "submodule", "foreach", "git lfs pull").Once()
			mockWriter.On("EndIf").Once()

			build := &common.Build{}
			build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_SUBMODULE_PATHS", Value: test.paths})
			shell.writeSubmoduleUpdateCmd(mockWriter, build, false)
		})
	}
}

func TestWriteUserScript(t *testing.T) {
	tests := map[string]struct {
		inputSteps        common.Steps
		prebuildScript    string
		postBuildScript   string
		buildStage        common.BuildStage
		setupExpectations func(*MockShellWriter)
		expectedErr       error
	}{
		"no build steps, after script": {
			inputSteps:        common.Steps{},
			prebuildScript:    "",
			postBuildScript:   "",
			buildStage:        common.BuildStageAfterScript,
			setupExpectations: func(*MockShellWriter) {},
			expectedErr:       common.ErrSkipBuildStage,
		},
		"single script step": {
			inputSteps: common.Steps{
				common.Step{
					Name:   common.StepNameScript,
					Script: common.StepScript{"echo hello"},
				},
			},
			prebuildScript:  "",
			postBuildScript: "",
			buildStage:      "step_script",
			setupExpectations: func(m *MockShellWriter) {
				m.On("Variable", mock.Anything)
				m.On("Cd", mock.AnythingOfType("string"))
				m.On("Noticef", "$ %s", "echo hello").Once()
				m.On("Line", "echo hello").Once()
				m.On("CheckForErrors").Once()
			},
			expectedErr: nil,
		},
		"prebuild, multiple steps postBuild": {
			inputSteps: common.Steps{
				common.Step{
					Name:   common.StepNameScript,
					Script: common.StepScript{"echo script"},
				},
				common.Step{
					Name:   "release",
					Script: common.StepScript{"echo release"},
				},
				common.Step{
					Name:   "a11y",
					Script: common.StepScript{"echo a11y"},
				},
			},
			prebuildScript:  "echo prebuild",
			postBuildScript: "echo postbuild",
			buildStage:      common.BuildStage("step_release"),
			setupExpectations: func(m *MockShellWriter) {
				m.On("Variable", mock.Anything)
				m.On("Cd", mock.AnythingOfType("string"))
				m.On("Noticef", "$ %s", "echo prebuild").Once()
				m.On("Noticef", "$ %s", "echo release").Once()
				m.On("Noticef", "$ %s", "echo postbuild").Once()
				m.On("Line", "echo prebuild").Once()
				m.On("Line", "echo release").Once()
				m.On("Line", "echo postbuild").Once()
				m.On("CheckForErrors").Times(3)
			},
			expectedErr: nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			info := common.ShellScriptInfo{
				PreBuildScript: tt.prebuildScript,
				Build: &common.Build{
					JobResponse: common.JobResponse{
						Steps: tt.inputSteps,
					},
				},
				PostBuildScript: tt.postBuildScript,
			}
			mockShellWriter := &MockShellWriter{}
			defer mockShellWriter.AssertExpectations(t)

			tt.setupExpectations(mockShellWriter)
			shell := AbstractShell{}

			err := shell.writeUserScript(mockShellWriter, info, tt.buildStage)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestSkipBuildStage(t *testing.T) {
	stageTests := map[common.BuildStage]map[string]struct {
		JobResponse common.JobResponse
		Runner      common.RunnerConfig
	}{
		common.BuildStageRestoreCache: {
			"don't skip if cache has paths": {
				common.JobResponse{
					Cache: common.Caches{
						common.Cache{
							Paths: []string{"default"},
						},
					},
				},
				common.RunnerConfig{},
			},
			"don't skip if cache uses untracked files": {
				common.JobResponse{
					Cache: common.Caches{
						common.Cache{
							Untracked: true,
						},
					},
				},
				common.RunnerConfig{},
			},
		},

		common.BuildStageDownloadArtifacts: {
			"don't skip if job has any dependencies": {
				common.JobResponse{
					Dependencies: common.Dependencies{
						common.Dependency{
							ID:            1,
							ArtifactsFile: common.DependencyArtifactsFile{Filename: "dependency.txt"},
						},
					},
				},
				common.RunnerConfig{},
			},
		},

		"step_script": {
			"don't skip if user script is defined": {
				common.JobResponse{
					Steps: common.Steps{
						common.Step{
							Name: common.StepNameScript,
						},
					},
				},
				common.RunnerConfig{},
			},
		},

		common.BuildStageAfterScript: {
			"don't skip if an after script is defined and has content": {
				common.JobResponse{
					Steps: common.Steps{
						common.Step{
							Name:   common.StepNameAfterScript,
							Script: common.StepScript{"echo 'hello world'"},
						},
					},
				},
				common.RunnerConfig{},
			},
		},

		common.BuildStageArchiveOnSuccessCache: {
			"don't skip if cache has paths": {
				common.JobResponse{
					Cache: common.Caches{
						common.Cache{
							Paths: []string{"default"},
							When:  common.CacheWhenOnSuccess,
						},
					},
				},
				common.RunnerConfig{},
			},
			"don't skip if cache uses untracked files": {
				common.JobResponse{
					Cache: common.Caches{
						common.Cache{
							Untracked: true,
							When:      common.CacheWhenOnSuccess,
						},
					},
				},
				common.RunnerConfig{},
			},
		},
		common.BuildStageArchiveOnFailureCache: {
			"don't skip if cache has paths": {
				common.JobResponse{
					Cache: common.Caches{
						common.Cache{
							Paths: []string{"default"},
							When:  common.CacheWhenOnFailure,
						},
					},
				},
				common.RunnerConfig{},
			},
			"don't skip if cache uses untracked files": {
				common.JobResponse{
					Cache: common.Caches{
						common.Cache{
							Untracked: true,
							When:      common.CacheWhenOnFailure,
						},
					},
				},
				common.RunnerConfig{},
			},
		},

		common.BuildStageUploadOnSuccessArtifacts: {
			"don't skip if artifact has paths and URL defined": {
				common.JobResponse{
					Artifacts: common.Artifacts{
						common.Artifact{
							When:  common.ArtifactWhenOnSuccess,
							Paths: []string{"default"},
						},
					},
				},
				common.RunnerConfig{
					RunnerCredentials: common.RunnerCredentials{
						URL: "https://example.com",
					},
				},
			},
			"don't skip if artifact uses untracked files and URL defined": {
				common.JobResponse{
					Artifacts: common.Artifacts{
						common.Artifact{
							When:      common.ArtifactWhenOnSuccess,
							Untracked: true,
						},
					},
				},
				common.RunnerConfig{
					RunnerCredentials: common.RunnerCredentials{
						URL: "https://example.com",
					},
				},
			},
		},

		common.BuildStageUploadOnFailureArtifacts: {
			"don't skip if artifact has paths and URL defined": {
				common.JobResponse{
					Artifacts: common.Artifacts{
						common.Artifact{
							When:  common.ArtifactWhenOnFailure,
							Paths: []string{"default"},
						},
					},
				},
				common.RunnerConfig{
					RunnerCredentials: common.RunnerCredentials{
						URL: "https://example.com",
					},
				},
			},
			"don't skip if artifact uses untracked files and URL defined": {
				common.JobResponse{
					Artifacts: common.Artifacts{
						common.Artifact{
							When:      common.ArtifactWhenOnFailure,
							Untracked: true,
						},
					},
				},
				common.RunnerConfig{
					RunnerCredentials: common.RunnerCredentials{
						URL: "https://example.com",
					},
				},
			},
		},
		common.BuildStageCleanupFileVariables: {
			"don't skip if file artifact defined": {
				common.JobResponse{
					Variables: common.JobVariables{
						{
							Key:      "FILE_VARIABLE",
							Value:    "CONTENTS",
							Public:   true,
							Internal: true,
							File:     true,
						},
					},
				},
				common.RunnerConfig{
					RunnerCredentials: common.RunnerCredentials{
						URL: "https://example.com",
					},
				},
			},
		},
	}

	shell := AbstractShell{}
	for stage, tests := range stageTests {
		t.Run(string(stage), func(t *testing.T) {
			for tn, tc := range tests {
				t.Run(tn, func(t *testing.T) {
					build := &common.Build{
						JobResponse: common.JobResponse{},
						Runner:      &common.RunnerConfig{},
					}
					info := common.ShellScriptInfo{
						RunnerCommand: "gitlab-runner-helper",
						Build:         build,
					}

					// empty stages should always be skipped
					err := shell.writeScript(&BashWriter{}, stage, info)
					assert.ErrorIs(t, err, common.ErrSkipBuildStage)

					// stages with bare minimum requirements should not be skipped.
					build = &common.Build{
						JobResponse: tc.JobResponse,
						Runner:      &tc.Runner,
					}
					info = common.ShellScriptInfo{
						RunnerCommand: "gitlab-runner-helper",
						Build:         build,
					}
					err = shell.writeScript(&BashWriter{}, stage, info)
					assert.NoError(t, err, "stage %v should not have been skipped", stage)
				})
			}
		})
	}
}

func TestAbstractShell_writeCleanupFileVariablesScript(t *testing.T) {
	testVar1 := "VAR_1"
	testVar2 := "VAR_2"
	testVar3 := "VAR_3"
	testVar4 := "VAR_4"

	testPath1 := "path/VAR_1_file"
	testPath3 := "path/VAR_3_file"

	info := common.ShellScriptInfo{
		Build: &common.Build{
			JobResponse: common.JobResponse{
				Variables: common.JobVariables{
					{Key: testVar1, Value: "test", File: true},
					{Key: testVar2, Value: "test", File: false},
					{Key: testVar3, Value: "test", File: true},
					{Key: testVar4, Value: "test", File: false},
				},
			},
		},
	}

	mockShellWriter := &MockShellWriter{}
	defer mockShellWriter.AssertExpectations(t)

	mockShellWriter.On("TmpFile", testVar1).Return(testPath1).Once()
	mockShellWriter.On("RmFile", testPath1).Once()
	mockShellWriter.On("TmpFile", testVar3).Return(testPath3).Once()
	mockShellWriter.On("RmFile", testPath3).Once()

	shell := new(AbstractShell)

	err := shell.writeCleanupFileVariablesScript(mockShellWriter, info)
	assert.NoError(t, err)
}

func BenchmarkScriptStage(b *testing.B) {
	stages := []common.BuildStage{
		common.BuildStagePrepare,
		common.BuildStageGetSources,
		common.BuildStageRestoreCache,
		common.BuildStageDownloadArtifacts,
		common.BuildStageAfterScript,
		common.BuildStageArchiveOnSuccessCache,
		common.BuildStageArchiveOnFailureCache,
		common.BuildStageUploadOnSuccessArtifacts,
		common.BuildStageUploadOnFailureArtifacts,
		common.BuildStageCleanupFileVariables,
		common.BuildStage("step_release"),
	}

	shells := []common.Shell{
		&BashShell{Shell: "sh"},
		&BashShell{Shell: "bash"},
		&PowerShell{Shell: SNPwsh, EOL: "\n"},
		&PowerShell{Shell: SNPowershell, EOL: "\r\n"},
		&CmdShell{},
	}

	for _, shell := range shells {
		for _, stage := range stages {
			b.Run(fmt.Sprintf("%s-%s", shell.GetName(), stage), func(b *testing.B) {
				benchmarkScriptStage(b, shell, stage)
			})
		}
	}
}

func benchmarkScriptStage(b *testing.B, shell common.Shell, stage common.BuildStage) {
	info := common.ShellScriptInfo{
		RunnerCommand:  "runner-helper",
		PreBuildScript: "echo prebuild",
		Build: &common.Build{
			CacheDir: "cache",
			Runner: &common.RunnerConfig{
				RunnerCredentials: common.RunnerCredentials{
					URL: "https://example.com",
				},
				RunnerSettings: common.RunnerSettings{
					BuildsDir: "build",
					CacheDir:  "cache",
					Cache: &common.CacheConfig{
						Type: "test",
					},
				},
			},
			JobResponse: common.JobResponse{
				GitInfo: common.GitInfo{
					Sha: "deadbeef",
				},
				Dependencies: []common.Dependency{{
					ID: 1,
					ArtifactsFile: common.DependencyArtifactsFile{
						Filename: "artifact.zip",
					},
				}},
				Artifacts: []common.Artifact{
					{
						Name:  "artifact",
						Paths: []string{"*"},
						When:  common.ArtifactWhenOnSuccess,
					},
					{
						Name:  "artifact",
						Paths: []string{"*"},
						When:  common.ArtifactWhenOnFailure,
					},
				},
				Cache: []common.Cache{
					{
						Key:    "cache",
						Paths:  []string{"*"},
						Policy: common.CachePolicyPullPush,
						When:   common.CacheWhenOnSuccess,
					},
					{
						Key:    "cache",
						Paths:  []string{"*"},
						Policy: common.CachePolicyPullPush,
						When:   common.CacheWhenOnFailure,
					},
				},
				Steps: common.Steps{
					common.Step{
						Name:   common.StepNameScript,
						Script: common.StepScript{"echo script"},
					},
					common.Step{
						Name:   common.StepNameAfterScript,
						Script: common.StepScript{"echo after_script"},
					},
					common.Step{
						Name:   "release",
						Script: common.StepScript{"echo release"},
					},
					common.Step{
						Name:   "a11y",
						Script: common.StepScript{"echo a11y"},
					},
				},
				Variables: []common.JobVariable{
					{
						Key:   "GIT_STRATEGY",
						Value: "fetch",
					},
					{
						Key:   "GIT_SUBMODULE_STRATEGY",
						Value: "normal",
					},
					{
						Key:   "file",
						Value: "value",
						File:  true,
					},
				},
			},
		},
		PostBuildScript: "echo postbuild",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		script, err := shell.GenerateScript(stage, info)
		b.SetBytes(int64(len(script)))
		assert.NoError(b, err, stage)
	}
}

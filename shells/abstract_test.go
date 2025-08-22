//go:build !integration

package shells

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	_ "gitlab.com/gitlab-org/gitlab-runner/cache/test"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls"
)

func TestAbstractShell_guardGetSourcesScriptHooks(t *testing.T) {
	testCases := []struct {
		name     string
		strategy common.GitStrategy
		script   []string
		setup    func(t *testing.T) ShellWriter
	}{
		{
			name:   "no scripts",
			script: []string{},
			setup: func(t *testing.T) ShellWriter {
				return NewMockShellWriter(t)
			},
		},
		{
			name:     "git strategy none",
			strategy: common.GitNone,
			script:   []string{"test"},
			setup: func(t *testing.T) ShellWriter {
				return NewMockShellWriter(t)
			},
		},
		{
			name:     "git strategy empty",
			strategy: common.GitEmpty,
			script:   []string{"test"},
			setup: func(t *testing.T) ShellWriter {
				return NewMockShellWriter(t)
			},
		},
		{
			name:   "writes command",
			script: []string{"test"},
			setup: func(t *testing.T) ShellWriter {
				msw := NewMockShellWriter(t)
				msw.On("Noticef", "$ %s", "test").Once().Return()
				msw.On("Line", "test").Once().Return()
				msw.On("CheckForErrors").Once().Return()
				return msw
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockShellWriter := tc.setup(t)
			shell := AbstractShell{}

			shell.guardGetSourcesScriptHooks(
				mockShellWriter,
				common.ShellScriptInfo{
					Build: &common.Build{
						JobResponse: common.JobResponse{
							Variables: common.JobVariables{
								{Key: "GIT_STRATEGY", Value: string(tc.strategy)},
								{Key: "GIT_CHECKOUT", Value: "false"},
							},
						},
					},
				},
				"",
				func() []string { return tc.script },
			)
		})
	}
}

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

	mockWriter := NewMockShellWriter(t)
	mockWriter.On("EnvVariableKey", tls.VariableCAFile).Return("VariableCAFile").Once()
	mockWriter.On("EnvVariableKey", tls.VariableCertFile).Return("VariableCertFile").Once()
	mockWriter.On("EnvVariableKey", tls.VariableKeyFile).Return("VariableKeyFile").Once()

	mockWriter.On(
		"CommandArgExpand",
		"git",
		"config",
		fmt.Sprintf("http.%s.%s", expectedURL, "sslCAInfo"),
		"VariableCAFile",
	).Once()
	mockWriter.On(
		"CommandArgExpand",
		"git",
		"config",
		fmt.Sprintf("http.%s.%s", expectedURL, "sslCert"),
		"VariableCertFile",
	).Once()
	mockWriter.On(
		"CommandArgExpand",
		"git",
		"config",
		fmt.Sprintf("http.%s.%s", expectedURL, "sslKey"),
		"VariableKeyFile",
	).Once()

	shell.writeGitSSLConfig(mockWriter, build, nil)
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

	mockWriter := NewMockShellWriter(t)
	mockWriter.On("Variable", mock.Anything)
	mockWriter.On("TmpFile", "gitlab_runner_env").Return("path/to/env/file").Once()
	mockWriter.On("SourceEnv", "path/to/env/file").Once()
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

	err := shell.writeScript(context.Background(), mockWriter, common.BuildStageUploadOnSuccessArtifacts, info)
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

	mockWriter := NewMockShellWriter(t)
	mockWriter.On("Variable", mock.Anything)
	mockWriter.On("TmpFile", "gitlab_runner_env").Return("path/to/env/file").Once()
	mockWriter.On("SourceEnv", "path/to/env/file").Once()
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

	err := shell.writeScript(context.Background(), mockWriter, common.BuildStageUploadOnFailureArtifacts, info)
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

	mockWriter := NewMockShellWriter(t)
	mockWriter.On("Variable", mock.Anything)
	mockWriter.On("TmpFile", "gitlab_runner_env").Return("path/to/env/file").Once()
	mockWriter.On("SourceEnv", "path/to/env/file").Once()
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

	err := shell.writeScript(context.Background(), mockWriter, common.BuildStageUploadOnSuccessArtifacts, info)
	require.NoError(t, err)
}

func getJobResponseWithCachePaths() common.JobResponse {
	return common.JobResponse{
		ID:    1000,
		Token: "token",
		JobInfo: common.JobInfo{
			Name: "some-job-name",
		},
		GitInfo: common.GitInfo{
			Ref: "some-git-ref",
		},
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
			common.Cache{
				Key:   "", // this forces the default cache key, comprised of the job name & the git ref
				Paths: []string{"unset-cache-key"},
				When:  common.CacheWhenAlways,
			},
		},
	}
}

var headerMatcher = mock.MatchedBy(func(arg string) bool {
	return regexp.
		MustCompile(`^(Header-1: a value|X-Fakecloud-Meta-Cachekey: (cache-key1|some-job-name\/some-git-ref))$`).
		MatchString(arg)
})

func localCacheFileMatcher(t *testing.T, expectedCacheDir string) any {
	expectedCacheDir = regexp.QuoteMeta(expectedCacheDir)
	sep := regexp.QuoteMeta(string(filepath.Separator))
	cacheKey := ".+" // can be different for hashed & unhashed cache keys
	re := regexp.MustCompile("^" + expectedCacheDir + sep + cacheKey + sep + "cache.zip" + "$")
	return mock.MatchedBy(func(actualCacheFile string) bool {
		return assert.Regexp(t, re, actualCacheFile, "local cache file path")
	})
}

func TestWriteWritingArchiveCache(t *testing.T) {
	const (
		gitlabURL    = "https://example.com:3443"
		cacheEnvFile = "/some/path/to/runner-cache-env"
	)

	shell := AbstractShell{}

	// for caches on the build see: getJobResponseWithCachePaths
	buildStages := map[common.BuildStage][][]any{
		common.BuildStageArchiveOnSuccessCache: [][]any{
			{"--path", "vendor/", "--untracked"},
			{"--path", "some/path1", "--path", "other/path2"},
			{"--path", "when-always"},
			{"--path", "unset-cache-key"},
		},
		common.BuildStageArchiveOnFailureCache: [][]any{
			{"--path", "when-on-failure", "--untracked"},
			{"--path", "when-always"},
			{"--path", "unset-cache-key"},
		},
	}

	tests := map[string]struct {
		cacheType                    string
		uploadArgs                   []any
		additionalExpectedAssertions func(shellWriter *MockShellWriter, nrOfCaches int)
	}{
		"no cache upload": {},
		"pre-signed URL cache": {
			cacheType: "test",
			uploadArgs: []any{
				"--url", mock.Anything,
				"--header", headerMatcher,
				"--header", headerMatcher,
			},
		},
		"GoCloud cache": {
			cacheType: "goCloudTest",
			uploadArgs: []any{
				"--gocloud-url", mock.Anything,
			},
		},
	}

	for buildStage, expectedArgsPerCache := range buildStages {
		t.Run(string(buildStage), func(t *testing.T) {
			for tn, tt := range tests {
				t.Run(tn, func(t *testing.T) {
					info := common.ShellScriptInfo{
						RunnerCommand: "gitlab-runner-helper",
						Build: &common.Build{
							CacheDir:    "cache_dir",
							JobResponse: getJobResponseWithCachePaths(),
							Runner: &common.RunnerConfig{
								RunnerSettings: common.RunnerSettings{
									Cache: &common.CacheConfig{
										Type:   tt.cacheType,
										Shared: true,
									},
								},
								RunnerCredentials: common.RunnerCredentials{
									URL: gitlabURL,
								},
							},
						},
					}

					mockWriter := NewMockShellWriter(t)
					mockWriter.On("Variable", mock.MatchedBy(func(v common.JobVariable) bool {
						return v.Key == "GITLAB_ENV"
					})).Once()
					mockWriter.On("TmpFile", "gitlab_runner_env").Return("path/to/env/file").Once()
					mockWriter.On("SourceEnv", "path/to/env/file").Once()
					mockWriter.On("Cd", mock.Anything).Once()

					for _, perCacheCommandArgs := range expectedArgsPerCache {
						allArgs := slices.Concat(
							// we expect this cmd & args, even if we don't upload
							[]any{
								"gitlab-runner-helper",
								"cache-archiver",
								"--file", localCacheFileMatcher(t, info.Build.CacheDir),
								"--timeout", mock.Anything,
							},
							// args per cache, e.g. paths of to-be-cached files
							perCacheCommandArgs,
							// args for the upload, e.g. URL, headers, env file
							tt.uploadArgs,
							// lastly, we expect the env file arg
							[]any{
								"--env-file", cacheEnvFile,
							},
						)

						mockWriter.On("IfCmd", "gitlab-runner-helper", "--version").Once()
						mockWriter.On("Noticef", "Creating cache %s...", mock.Anything).Once()

						mockWriter.On("IfCmdWithOutput", allArgs...).Once()

						mockWriter.On("Noticef", "Created cache").Once()
						mockWriter.On("Else").Once()
						mockWriter.On("Warningf", "Failed to create cache").Once()
						mockWriter.On("EndIf").Once()
						mockWriter.On("Else").Once()
						mockWriter.On("Warningf", mock.Anything, mock.Anything, mock.Anything).Once()
						mockWriter.On("EndIf").Once()

						mockWriter.On("DotEnvVariables", "gitlab_runner_cache_env", mock.Anything).Return(cacheEnvFile).Once()
						mockWriter.On("RmFile", cacheEnvFile).Once()
					}

					varCount := len(info.Build.GetAllVariables())
					mockWriter.On("Variable", mock.Anything).Times(varCount)

					err := shell.writeScript(context.Background(), mockWriter, buildStage, info)
					require.NoError(t, err)
				})
			}
		})
	}
}

func TestAbstractShell_handleGetSourcesStrategy(t *testing.T) {
	const (
		// The jobResponse is always a http(s) URL. If it were a `git@example.com/...` URL, go's url.Parse() would fail. To
		// make the runner pull via SSH, `clone_url` would need to be used, to change from a http(s) URL to an SSH URL.
		repoURI        = "https://user:pass@example.com/project/repo.git"
		cleanedRepoURI = "https://example.com/project/repo.git"
		repoHostURI    = "https://example.com"
	)

	v := common.AppVersion
	userAgent := fmt.Sprintf("http.userAgent=%s %s %s/%s", v.Name, v.Version, v.OS, v.Architecture)

	withoutNative := func(m *MockShellWriter, repoURI, templateDir, credHelperPath string) {
		m.EXPECT().Noticef("Fetching changes...").Once()

		m.EXPECT().Command("git", "init", "build/dir", "--template", templateDir).Once()
		m.EXPECT().Cd("build/dir").Once()

		if credHelperPath != "" {
			m.EXPECT().Command("git", "config", "include.path", credHelperPath).Once()
		}

		m.EXPECT().IfCmd("git", "remote", "add", "origin", repoURI).Once()
		m.EXPECT().Noticef("Created fresh repository.").Once()
		m.EXPECT().Else().Once()
		m.EXPECT().Command("git", "remote", "set-url", "origin", repoURI).Once()
		m.EXPECT().EndIf().Once()

		m.EXPECT().IfFile(".git/shallow").Once()
		m.EXPECT().Command("git", "-c", userAgent, "fetch", "origin", "--no-recurse-submodules", "--prune", "--quiet", "--unshallow").Once()
		m.EXPECT().Else().Once()
		m.EXPECT().Command("git", "-c", userAgent, "fetch", "origin", "--no-recurse-submodules", "--prune", "--quiet").Once()
		m.EXPECT().EndIf().Once()
	}

	testCases := []struct {
		name              string
		buildDir          string
		depth             int
		ref               string
		gitStrategy       string
		nativeClone       bool
		setupExpectations func(*MockShellWriter, string, string, string)
	}{
		{
			name:              "clone strategy without native",
			buildDir:          "build/dir",
			gitStrategy:       "clone",
			setupExpectations: withoutNative,
		},
		{
			name:        "clone strategy with native",
			buildDir:    "build/dir",
			gitStrategy: "clone",
			nativeClone: true,
			setupExpectations: func(m *MockShellWriter, repoURI, templateDir, credHelperPath string) {
				m.EXPECT().IfGitVersionIsAtLeast("2.49").Once()
				m.EXPECT().Noticef("Cloning repository...").Once()

				gitArgs := []any{"-c", userAgent, "clone", "--no-checkout", repoURI, "build/dir", "--template", templateDir}
				if credHelperPath != "" {
					gitArgs = append(gitArgs, "-c", "include.path="+credHelperPath)
				}

				m.EXPECT().Command("git", gitArgs...).Once()
				m.EXPECT().Cd("build/dir").Once()

				if credHelperPath != "" {
					m.EXPECT().Command("git", "config", "include.path", credHelperPath).Once()
				}

				m.EXPECT().Else().Once()
				withoutNative(m, repoURI, templateDir, credHelperPath)
				m.EXPECT().EndIf().Once()
			},
		},
		{
			name:        "clone strategy with native and branch ref",
			buildDir:    "build/dir",
			gitStrategy: "clone",
			nativeClone: true,
			ref:         "feature",
			setupExpectations: func(m *MockShellWriter, repoURI, templateDir, credHelperPath string) {
				m.EXPECT().IfGitVersionIsAtLeast("2.49").Once()
				m.EXPECT().Noticef("Cloning repository for %s...", "feature").Once()

				gitArgs := []any{"-c", userAgent, "clone", "--no-checkout", repoURI, "build/dir", "--template", templateDir, "--branch", "feature"}
				if credHelperPath != "" {
					gitArgs = append(gitArgs, "-c", "include.path="+credHelperPath)
				}

				m.EXPECT().Command("git", gitArgs...).Once()
				m.EXPECT().Cd("build/dir").Once()

				if credHelperPath != "" {
					m.EXPECT().Command("git", "config", "include.path", credHelperPath).Once()
				}

				m.EXPECT().Else().Once()
				withoutNative(m, repoURI, templateDir, credHelperPath)
				m.EXPECT().EndIf().Once()
			},
		},
		{
			name:        "clone strategy with native and ref",
			buildDir:    "build/dir",
			gitStrategy: "clone",
			nativeClone: true,
			ref:         "refs/some/thing",
			setupExpectations: func(m *MockShellWriter, repoURI, templateDir, credHelperPath string) {
				m.EXPECT().IfGitVersionIsAtLeast("2.49").Once()
				m.EXPECT().Noticef("Cloning repository for %s...", "refs/some/thing").Once()

				gitArgs := []any{"-c", userAgent, "clone", "--no-checkout", repoURI, "build/dir", "--template", templateDir, "--revision", "refs/some/thing"}
				if credHelperPath != "" {
					gitArgs = append(gitArgs, "-c", "include.path="+credHelperPath)
				}

				m.EXPECT().Command("git", gitArgs...).Once()
				m.EXPECT().Cd("build/dir").Once()

				if credHelperPath != "" {
					m.EXPECT().Command("git", "config", "include.path", credHelperPath).Once()
				}

				m.EXPECT().Else().Once()
				withoutNative(m, repoURI, templateDir, credHelperPath)
				m.EXPECT().EndIf().Once()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, jobTokenFromEnv := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s:%t", featureflags.GitURLsWithoutTokens, jobTokenFromEnv), func(t *testing.T) {
					build := &common.Build{
						Runner: &common.RunnerConfig{
							RunnerSettings: common.RunnerSettings{
								FeatureFlags: map[string]bool{
									featureflags.UseGitNativeClone:    tc.nativeClone,
									featureflags.GitURLsWithoutTokens: jobTokenFromEnv,
								},
							},
						},
						JobResponse: common.JobResponse{
							GitInfo: common.GitInfo{
								Depth:   tc.depth,
								Ref:     tc.ref,
								RepoURL: repoURI,
							},
							Variables: common.JobVariables{
								{Key: "GIT_STRATEGY", Value: tc.gitStrategy},
							},
							JobRequestCorrelationID: "foobar",
						},
						BuildDir: tc.buildDir,
					}

					info := common.ShellScriptInfo{
						RunnerCommand: "gitlab-runner-helper",
						Build:         build,
					}

					mockShellWriter := NewMockShellWriter(t)

					// common expectations across all test cases
					mockShellWriter.EXPECT().RmDir("build/dir").Once()
					templateDir, _ := expectSetupTemplate(mockShellWriter, "build/dir")
					expectFileCleanup(mockShellWriter, "build/dir/.git", false)
					expectGitConfigCleanup(mockShellWriter, "build/dir", false)
					expectedRepoURI := repoURI
					credHelperPath := ""
					if build.IsFeatureFlagOn(featureflags.GitURLsWithoutTokens) {
						expectedRepoURI = cleanedRepoURI
						credHelperPath = expectGitCredHelperSetup(mockShellWriter, repoHostURI)
					}

					// additional specific expectations per test case
					tc.setupExpectations(mockShellWriter, expectedRepoURI, templateDir, credHelperPath)

					shell := AbstractShell{}

					assert.NoError(t, shell.handleGetSourcesStrategy(mockShellWriter, info))
				})
			}
		})
	}
}

func TestAbstractShell_writeGetSourcesScript(t *testing.T) {
	t.Parallel()
	const (
		bash       = "bash"
		pwsh       = "pwsh"
		powershell = "powershell"
	)

	for _, shell := range []string{bash, pwsh, powershell} {
		for _, useJobTokenFromEnv := range []bool{true, false} {
			testCases := []struct {
				name     string
				strategy common.GitStrategy
				setup    func(t *testing.T) ShellWriter
			}{
				{
					name:     "strategy none",
					strategy: common.GitNone,
					setup: func(t *testing.T) ShellWriter {
						msw := NewMockShellWriter(t)
						msw.EXPECT().Variable(mock.Anything)
						msw.EXPECT().TmpFile("gitlab_runner_env").Return("path/to/env/file").Once()
						msw.EXPECT().SourceEnv("path/to/env/file").Once()
						msw.EXPECT().Noticef("Skipping Git repository setup").Once()
						msw.EXPECT().MkDir("build-dir").Once()
						msw.EXPECT().Noticef("Skipping Git checkout").Once()
						msw.EXPECT().Noticef("Skipping Git submodules setup").Once()
						if shell == bash {
							msw.EXPECT().IfFile("/.gitlab-build-uid-gid").Once()
							msw.EXPECT().EndIf().Once()
						}
						return msw
					},
				},
				{
					name:     "strategy empty",
					strategy: common.GitEmpty,
					setup: func(t *testing.T) ShellWriter {
						msw := NewMockShellWriter(t)
						msw.EXPECT().Variable(mock.Anything)
						msw.EXPECT().TmpFile("gitlab_runner_env").Return("path/to/env/file").Once()
						msw.EXPECT().SourceEnv("path/to/env/file").Once()
						msw.EXPECT().Noticef("Skipping Git repository setup and creating an empty build directory").Once()
						msw.EXPECT().RmDir("build-dir").Once()
						msw.EXPECT().MkDir("build-dir").Once()
						msw.EXPECT().Noticef("Skipping Git checkout").Once()
						msw.EXPECT().Noticef("Skipping Git submodules setup").Once()
						if shell == bash {
							msw.EXPECT().IfFile("/.gitlab-build-uid-gid").Once()
							msw.EXPECT().EndIf().Once()
						}
						return msw
					},
				},
				{
					name:     "strategy clone",
					strategy: common.GitClone,
					setup: func(t *testing.T) ShellWriter {
						msw := NewMockShellWriter(t)
						msw.EXPECT().Variable(mock.Anything)
						msw.EXPECT().TmpFile("gitlab_runner_env").Return("path/to/env/file").Once()
						msw.EXPECT().SourceEnv("path/to/env/file").Once()
						msw.EXPECT().Noticef("$ %s", "config pre_get_sources").Once()
						msw.EXPECT().Line("config pre_get_sources").Once()
						msw.EXPECT().Noticef("$ %s", "job payload").Once()
						msw.EXPECT().Line("job payload").Once()
						msw.EXPECT().Noticef("$ %s", "pre_get_sources").Once()
						msw.EXPECT().Line("pre_get_sources").Once()
						msw.EXPECT().CheckForErrors()
						msw.EXPECT().Noticef("Fetching changes...").Once()
						msw.EXPECT().RmDir("build-dir").Once()

						templateDir, templateSetupCommands := expectSetupTemplate(msw, "build-dir")
						expectFileCleanup(msw, "build-dir/.git", false)
						gitCleanupCommands := expectGitConfigCleanup(msw, "build-dir", false)
						// Ensure, cleanup happens before template dir setup
						mock.InOrder(slices.Concat(gitCleanupCommands, templateSetupCommands)...)

						msw.EXPECT().Command("git", "init", "build-dir", "--template", templateDir).Once()
						msw.EXPECT().Cd("build-dir").Once()

						msw.EXPECT().IfCmd("git", "remote", "add", "origin", "https://repo-url/some/repo").Once()
						msw.EXPECT().Noticef("Created fresh repository.").Once()
						msw.EXPECT().Else().Once()
						msw.EXPECT().Command("git", "remote", "set-url", "origin", "https://repo-url/some/repo").Once()
						msw.EXPECT().EndIf().Once()

						msw.EXPECT().IfFile(".git/shallow").Once()
						msw.EXPECT().Command("git", "-c", mock.Anything, "fetch", "origin", "--no-recurse-submodules", "--prune", "--quiet", "--unshallow").Once()
						msw.EXPECT().Else().Once()
						msw.EXPECT().Command("git", "-c", mock.Anything, "fetch", "origin", "--no-recurse-submodules", "--prune", "--quiet").Once()
						msw.EXPECT().EndIf().Once()

						msw.EXPECT().Noticef("Skipping Git checkout").Once()
						msw.EXPECT().Noticef("Skipping Git submodules setup").Once()

						msw.EXPECT().Noticef("$ %s", "job payload").Once()
						msw.EXPECT().Line("job payload").Once()
						msw.EXPECT().Noticef("$ %s", "post_get_sources").Once()
						msw.EXPECT().Line("post_get_sources").Once()
						msw.EXPECT().Noticef("$ %s", "config post_get_sources").Once()
						msw.EXPECT().Line("config post_get_sources").Once()

						if shell == bash {
							msw.EXPECT().IfFile("/.gitlab-build-uid-gid").Once()
							msw.EXPECT().EndIf().Once()
						}
						if useJobTokenFromEnv {
							helperPath := expectGitCredHelperSetup(msw, "https://repo-url")
							msw.EXPECT().Command("git", "config", "include.path", helperPath)
						}
						return msw
					},
				},
				{
					name:     "strategy fetch",
					strategy: common.GitFetch,
					setup: func(t *testing.T) ShellWriter {
						msw := NewMockShellWriter(t)
						msw.EXPECT().Variable(mock.Anything)
						msw.EXPECT().TmpFile("gitlab_runner_env").Return("path/to/env/file").Once()
						msw.EXPECT().SourceEnv("path/to/env/file").Once()
						msw.EXPECT().Noticef("$ %s", "config pre_get_sources").Once()
						msw.EXPECT().Line("config pre_get_sources").Once()
						msw.EXPECT().Noticef("$ %s", "job payload").Once()
						msw.EXPECT().Line("job payload").Once()
						msw.EXPECT().Noticef("$ %s", "pre_get_sources").Once()
						msw.EXPECT().Line("pre_get_sources").Once()
						msw.EXPECT().CheckForErrors()
						msw.EXPECT().Noticef("Fetching changes...").Once()
						templateDir, templateSetupCommands := expectSetupTemplate(msw, "build-dir")
						expectFileCleanup(msw, "build-dir/.git", false)
						gitCleanupCommands := expectGitConfigCleanup(msw, "build-dir", false)
						// Ensure, cleanup happens before template dir setup
						mock.InOrder(slices.Concat(gitCleanupCommands, templateSetupCommands)...)

						msw.EXPECT().Command("git", "init", "build-dir", "--template", templateDir).Once()
						msw.EXPECT().Cd("build-dir").Once()
						msw.EXPECT().IfCmd("git", "remote", "add", "origin", "https://repo-url/some/repo").Once()
						msw.EXPECT().Noticef("Created fresh repository.").Once()
						msw.EXPECT().Else().Once()
						msw.EXPECT().Command("git", "remote", "set-url", "origin", "https://repo-url/some/repo").Once()
						msw.EXPECT().EndIf().Once()
						msw.EXPECT().IfFile(".git/shallow").Once()
						msw.EXPECT().Command("git", "-c", mock.Anything, "fetch", "origin", "--no-recurse-submodules", "--prune", "--quiet", "--unshallow").Once()
						msw.EXPECT().Else().Once()
						msw.EXPECT().Command("git", "-c", mock.Anything, "fetch", "origin", "--no-recurse-submodules", "--prune", "--quiet").Once()
						msw.EXPECT().EndIf().Once()
						msw.EXPECT().Noticef("Skipping Git checkout").Once()
						msw.EXPECT().Noticef("Skipping Git submodules setup").Once()

						msw.EXPECT().Noticef("$ %s", "job payload").Once()
						msw.EXPECT().Line("job payload").Once()
						msw.EXPECT().Noticef("$ %s", "post_get_sources").Once()
						msw.EXPECT().Line("post_get_sources").Once()

						msw.EXPECT().Noticef("$ %s", "config post_get_sources").Once()
						msw.EXPECT().Line("config post_get_sources").Once()
						if shell == bash {
							msw.EXPECT().IfFile("/.gitlab-build-uid-gid").Once()
							msw.EXPECT().EndIf().Once()
						}
						if useJobTokenFromEnv {
							helperPath := expectGitCredHelperSetup(msw, "https://repo-url")
							msw.EXPECT().Command("git", "config", "include.path", helperPath)
						}
						return msw
					},
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name+" with shell "+shell+" use job token from env "+fmt.Sprintf("%t", useJobTokenFromEnv), func(t *testing.T) {
					// Arrange
					info := common.ShellScriptInfo{
						Shell: shell,
						Build: &common.Build{
							JobResponse: common.JobResponse{
								Token: "some-token",
								Variables: common.JobVariables{
									{Key: "GIT_STRATEGY", Value: string(tc.strategy)},
									{Key: "GIT_CHECKOUT", Value: "false"},
								},
								GitInfo: common.GitInfo{
									RepoURL: "https://repo-url/some/repo",
								},
								Hooks: common.Hooks{
									{
										Name:   common.HookPreGetSourcesScript,
										Script: common.StepScript{"job payload", "pre_get_sources"},
									},
									{
										Name:   common.HookPostGetSourcesScript,
										Script: common.StepScript{"job payload", "post_get_sources"},
									},
								},
								JobRequestCorrelationID: "foobar",
							},
							Runner: &common.RunnerConfig{
								RunnerSettings: common.RunnerSettings{
									FeatureFlags: map[string]bool{
										featureflags.GitURLsWithoutTokens: useJobTokenFromEnv,
									},
								},
							},
							BuildDir: "build-dir",
						},
						PreGetSourcesScript:  "config pre_get_sources",
						PostGetSourcesScript: "config post_get_sources",
					}
					msw := tc.setup(t)
					shell := new(AbstractShell)

					// Act
					err := shell.writeGetSourcesScript(context.Background(), msw, info)
					assert.NoError(t, err)

					// Assert: assertion on shell wrtier performed in the setup of each test case.
				})
			}
		}
	}
}

func TestAbstractShell_writeCleanupBuildDirectoryScript(t *testing.T) {
	testCases := []struct {
		name                 string
		buildDir             string
		gitStrategy          string
		gitCleanFlags        string
		gitSubmoduleStrategy string
		setupExpectations    func(*MockShellWriter)
	}{
		{
			name:        "cloned repository",
			buildDir:    "build/dir",
			gitStrategy: "clone",
			setupExpectations: func(m *MockShellWriter) {
				m.On("RmDir", "build/dir")
			},
		},
		{
			name:        "empty git strategy",
			buildDir:    "build/dir",
			gitStrategy: "empty",
			setupExpectations: func(m *MockShellWriter) {
				m.On("RmDir", "build/dir")
			},
		},
		{
			name:        "no git strategy",
			buildDir:    "build/dir",
			gitStrategy: "none",
			setupExpectations: func(m *MockShellWriter) {
				m.On("Noticef", "Skipping build directory cleanup step")
			},
		},
		{
			name:        "git fetch strategy",
			buildDir:    "some/dir/to/the/repo",
			gitStrategy: "fetch",
			setupExpectations: func(m *MockShellWriter) {
				m.On("Cd", "some/dir/to/the/repo")
				m.On("Command", "git", "clean", "-ffdx")
				m.On("Command", "git", "reset", "--hard")
			},
		},
		{
			name:          "git fetch with git clean flags",
			buildDir:      "/build/dir/for/project",
			gitStrategy:   "fetch",
			gitCleanFlags: "-x -d -f",
			setupExpectations: func(m *MockShellWriter) {
				m.On("Cd", "/build/dir/for/project")
				m.On("Command", "git", "clean", "-x", "-d", "-f")
				m.On("Command", "git", "reset", "--hard")
			},
		},
		{
			name:                 "git fetch with recursive submodule strategy",
			buildDir:             "/dir/for/project",
			gitStrategy:          "fetch",
			gitCleanFlags:        "-n -q",
			gitSubmoduleStrategy: "recursive",
			setupExpectations: func(m *MockShellWriter) {
				m.On("Cd", "/dir/for/project")
				m.On("Command", "git", "clean", "-n", "-q")
				m.On("Command", "git", "reset", "--hard")
				m.On("Command", "git", "submodule", "foreach", "--recursive", "git", "clean", "-n", "-q")
				m.On("Command", "git", "submodule", "foreach", "--recursive", "git", "reset", "--hard")
			},
		},
		{
			name:                 "git fetch with normal submodule strategy",
			buildDir:             "/dir/for/project",
			gitStrategy:          "fetch",
			gitCleanFlags:        "-x",
			gitSubmoduleStrategy: "normal",
			setupExpectations: func(m *MockShellWriter) {
				m.On("Cd", "/dir/for/project")
				m.On("Command", "git", "clean", "-x")
				m.On("Command", "git", "reset", "--hard")
				m.On("Command", "git", "submodule", "foreach", "git", "clean", "-x")
				m.On("Command", "git", "submodule", "foreach", "git", "reset", "--hard")
			},
		},
		{
			name:        "invalid git strategy",
			buildDir:    "/dir/for/project",
			gitStrategy: "use-svn",
			setupExpectations: func(m *MockShellWriter) {
				m.On("RmDir", "/dir/for/project")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			info := common.ShellScriptInfo{
				Build: &common.Build{
					JobResponse: common.JobResponse{
						Variables: common.JobVariables{
							common.JobVariable{
								Key:   "GIT_STRATEGY",
								Value: tc.gitStrategy,
							},
							common.JobVariable{
								Key:   "GIT_CLEAN_FLAGS",
								Value: tc.gitCleanFlags,
							},
							common.JobVariable{
								Key:   "GIT_SUBMODULE_STRATEGY",
								Value: tc.gitSubmoduleStrategy,
							},
						},
					},
					BuildDir: tc.buildDir,
				},
			}
			mockShellWriter := NewMockShellWriter(t)
			tc.setupExpectations(mockShellWriter)
			shell := AbstractShell{}

			assert.NoError(t, shell.writeCleanupBuildDirectoryScript(mockShellWriter, info))
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

			mockWriter := NewMockShellWriter(t)
			mockWriter.On("Noticef", "Checking out %s as detached HEAD (ref is %s)...", dummySha[0:8], dummyRef).Once()
			mockWriter.On("Command", "git", "-c", "submodule.recurse=false", "checkout", "-f", "-q", dummySha).Once()

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
	const (
		dummySha        = "01234567abcdef"
		dummyRef        = "main"
		dummyProjectDir = "./"
	)

	tests := map[string]struct {
		value        string
		depth        int
		objectFormat string

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
		"depth non zero": {
			depth:                 1,
			value:                 "--quiet",
			expectedGitFetchFlags: []interface{}{"--depth", "1", "--quiet"},
		},
		"object format SHA256": {
			value:                 "",
			objectFormat:          "sha256",
			expectedGitFetchFlags: []interface{}{"--prune", "--quiet"},
		},
		"disabled": {
			value: "none",
		},
	}

	for _, credConfigFile := range []string{"some/cred-helper.conf", ""} {
		t.Run("credConfigFile:"+credConfigFile, func(t *testing.T) {
			for name, test := range tests {
				t.Run(name, func(t *testing.T) {
					shell := AbstractShell{}

					build := &common.Build{
						Runner: &common.RunnerConfig{},
						JobResponse: common.JobResponse{
							GitInfo: common.GitInfo{Sha: dummySha, Ref: dummyRef, Depth: test.depth, RepoObjectFormat: test.objectFormat},
							Variables: common.JobVariables{
								{Key: "GIT_FETCH_EXTRA_FLAGS", Value: test.value},
							},
						},
						BuildDir: dummyProjectDir,
					}
					build.SafeDirectoryCheckout = true

					mockWriter := NewMockShellWriter(t)
					shellScriptInfo := common.ShellScriptInfo{
						Build: build,
					}

					if test.depth == 0 {
						mockWriter.EXPECT().Noticef("Fetching changes...").Once()
					} else {
						mockWriter.EXPECT().Noticef("Fetching changes with git depth set to %d...", test.depth).Once()
					}

					var expectedObjectFormat = "sha1"
					if test.objectFormat != "" {
						expectedObjectFormat = test.objectFormat
					}

					templateDir := "/some/template/dir"
					if expectedObjectFormat != "sha1" {
						mockWriter.EXPECT().Command("git", "init", dummyProjectDir, "--template", templateDir, "--object-format", expectedObjectFormat).Once()
					} else {
						mockWriter.EXPECT().Command("git", "init", dummyProjectDir, "--template", templateDir).Once()
					}

					mockWriter.EXPECT().Cd(mock.Anything).Once()
					mockWriter.EXPECT().IfCmd("git", "remote", "add", "origin", mock.Anything).Once()
					mockWriter.EXPECT().Noticef("Created fresh repository.").Once()
					mockWriter.EXPECT().Else().Once()
					mockWriter.EXPECT().Command("git", "remote", "set-url", "origin", mock.Anything).Once()
					mockWriter.EXPECT().EndIf().Once()

					v := common.AppVersion
					userAgent := fmt.Sprintf("http.userAgent=%s %s %s/%s", v.Name, v.Version, v.OS, v.Architecture)
					command := []interface{}{"-c", userAgent, "fetch", "origin", "--no-recurse-submodules"}
					command = append(command, test.expectedGitFetchFlags...)

					if test.depth == 0 {
						unshallowArgs := append(command, "--unshallow") //nolint:gocritic
						mockWriter.EXPECT().IfFile(".git/shallow").Once()
						mockWriter.EXPECT().Command("git", unshallowArgs...).Once()
						mockWriter.EXPECT().Else().Once()
						mockWriter.EXPECT().Command("git", command...).Once()
						mockWriter.EXPECT().EndIf().Once()
					} else {
						mockWriter.EXPECT().Command("git", command...).Once()
					}

					if credConfigFile != "" {
						mockWriter.EXPECT().Command("git", "config", "include.path", credConfigFile)
					}

					err := shell.writeRefspecFetchCmd(mockWriter, shellScriptInfo, templateDir, credConfigFile)
					assert.NoError(t, err, "calling shell.writeRefspecFetchCmd")
				})
			}
		})
	}
}

func expectGitConfigCleanup(sw *MockShellWriter, buildDir string, withSubmodules bool) []*mock.Call {
	calls := []*mock.Call{
		sw.EXPECT().TmpFile("git-template").Return("someGitTemplateDir").Once(),
		sw.EXPECT().Join(buildDir, ".git").Return("someGitDir").Once(),

		sw.EXPECT().Join("someGitTemplateDir", "config").Return("someGitTemplateDir/config").Once(),
		sw.EXPECT().RmFile("someGitTemplateDir/config").Once(),
		sw.EXPECT().Join("someGitTemplateDir", "hooks").Return("someGitTemplateDir/hooks").Once(),
		sw.EXPECT().RmDir("someGitTemplateDir/hooks").Once(),
		sw.EXPECT().Join("someGitDir", "config").Return("someGitDir/config").Once(),
		sw.EXPECT().RmFile("someGitDir/config").Once(),
		sw.EXPECT().Join("someGitDir", "hooks").Return("someGitDir/hooks").Once(),
		sw.EXPECT().RmDir("someGitDir/hooks").Once(),
	}

	if withSubmodules {
		calls = append(calls,
			sw.EXPECT().Join(buildDir, ".git", "modules").Return("someModulesDir").Once(),
			sw.EXPECT().RmFilesRecursive("someModulesDir", "config").Once(),
			sw.EXPECT().RmDirsRecursive("someModulesDir", "hooks").Once(),
		)
	}

	mock.InOrder(calls...)

	return calls
}

func TestAbstractShell_writeSubmoduleUpdateCmd(t *testing.T) {
	const (
		exampleBaseURL  = "http://test.remote"
		exampleJobToken = "job-token"
		insteadOf       = "url.http://gitlab-ci-token:job-token@test.remote.insteadOf=http://test.remote"
	)

	tests := map[string]struct {
		Recursive               bool
		Depth                   int
		GitCleanFlags           string
		GitSubmoduleUpdateFlags string
		ExpectedNoticeArgs      []any
		ExpectedGitUpdateFlags  []any
		ExpectedGitForEachFlags []any
		ExpectedGitCleanFlags   []string
	}{
		"no recursion, no depth limit": {
			Recursive:             false,
			Depth:                 0,
			ExpectedNoticeArgs:    []any{"Updating/initializing submodules..."},
			ExpectedGitCleanFlags: []string{"-ffdx"},
		},
		"no recursion, depth limit 10": {
			Recursive:               false,
			Depth:                   10,
			ExpectedNoticeArgs:      []any{"Updating/initializing submodules with git depth set to %d...", 10},
			ExpectedGitUpdateFlags:  []any{"--depth", "10"},
			ExpectedGitForEachFlags: []any{},
			ExpectedGitCleanFlags:   []string{"-ffdx"},
		},
		"with recursion, no depth limit": {
			Recursive:               true,
			Depth:                   0,
			ExpectedNoticeArgs:      []any{"Updating/initializing submodules recursively..."},
			ExpectedGitUpdateFlags:  []any{"--recursive"},
			ExpectedGitForEachFlags: []any{"--recursive"},
			ExpectedGitCleanFlags:   []string{"-ffdx"},
		},
		"with recursion, depth limit 1": {
			Recursive:               true,
			Depth:                   1,
			ExpectedNoticeArgs:      []any{"Updating/initializing submodules recursively with git depth set to %d...", 1},
			ExpectedGitUpdateFlags:  []any{"--recursive", "--depth", "1"},
			ExpectedGitForEachFlags: []any{"--recursive"},
			ExpectedGitCleanFlags:   []string{"-ffdx"},
		},
		"with custom git clean flags": {
			Recursive:             false,
			Depth:                 0,
			ExpectedNoticeArgs:    []any{"Updating/initializing submodules..."},
			GitCleanFlags:         "custom-flags",
			ExpectedGitCleanFlags: []string{"custom-flags"},
		},
		"with recursion, no depth limit, and update flags": {
			Recursive:               true,
			Depth:                   0,
			GitSubmoduleUpdateFlags: " --remote  --progress  ",
			ExpectedNoticeArgs:      []any{"Updating/initializing submodules recursively..."},
			ExpectedGitUpdateFlags:  []any{"--recursive", "--remote", "--progress"},
			ExpectedGitForEachFlags: []any{"--recursive"},
			ExpectedGitCleanFlags:   []string{"-ffdx"},
		},
	}

	for _, useJobTokenFromEnv := range []bool{true, false} {
		name := fmt.Sprintf("%s:%t", featureflags.GitURLsWithoutTokens, useJobTokenFromEnv)
		t.Run(name, func(t *testing.T) {
			for tn, tc := range tests {
				t.Run(tn, func(t *testing.T) {
					shell := AbstractShell{}
					mockWriter := NewMockShellWriter(t)

					someCredHelperPath := "/some/helper/path"

					expectedGitForEachArgsFn := func() []any {
						return append(
							[]any{"submodule", "foreach"},
							tc.ExpectedGitForEachFlags...,
						)
					}
					gitSubArgs := []any{"-c", insteadOf}
					if useJobTokenFromEnv {
						gitSubArgs = []any{"-c", "include.path=" + someCredHelperPath}
					}

					expectSubmoduleCleanCommand := func() {
						mockWriter.EXPECT().Command("git", append(expectedGitForEachArgsFn(), "git clean "+strings.Join(tc.ExpectedGitCleanFlags, " "))...).Once()
					}
					expectSubmoduleSyncCommand := func() {
						mockWriter.EXPECT().Command("git", append([]any{"submodule", "sync"}, tc.ExpectedGitForEachFlags...)...).Once()
					}

					mockWriter.EXPECT().Noticef(tc.ExpectedNoticeArgs[0], tc.ExpectedNoticeArgs[1:]...).Once()
					mockWriter.EXPECT().Command("git", "submodule", "init").Once()
					expectSubmoduleSyncCommand()
					if useJobTokenFromEnv {
						mockWriter.EXPECT().TmpFile(mock.Anything).Return(someCredHelperPath).Once()
					}

					expectSubmoduleCleanCommand()
					mockWriter.EXPECT().Command("git", append(expectedGitForEachArgsFn(), "git reset --hard")...).Once()

					mockWriter.EXPECT().IfCmdWithOutput("git", slices.Concat(gitSubArgs, []any{"submodule", "update", "--init"}, tc.ExpectedGitUpdateFlags)...).Once()
					{ //nolint:gocritic
						// if branch ...
						mockWriter.EXPECT().Noticef("Updated submodules").Once()
						expectSubmoduleSyncCommand()
					}
					mockWriter.EXPECT().Else().Once()
					{ //nolint:gocritic
						// else branch ...
						mockWriter.EXPECT().Warningf("Updating submodules failed. Retrying...").Once()
						if strings.Contains(tc.GitSubmoduleUpdateFlags, "--remote") {
							mockWriter.EXPECT().Command("git", slices.Concat(gitSubArgs, []any{"submodule", "foreach"}, tc.ExpectedGitForEachFlags, []any{"git fetch origin +refs/heads/*:refs/remotes/origin/*"})...).Once()
						}
						expectSubmoduleSyncCommand()
						mockWriter.EXPECT().Command("git", slices.Concat(gitSubArgs, []any{"submodule", "update", "--init"}, tc.ExpectedGitUpdateFlags)...).Once()
						mockWriter.EXPECT().Command("git", append(expectedGitForEachArgsFn(), "git reset --hard")...).Once()
					}
					mockWriter.EXPECT().EndIf().Once()

					expectSubmoduleCleanCommand()

					mockWriter.EXPECT().IfCmd("git", "lfs", "version").Once()
					mockWriter.EXPECT().Command("git", slices.Concat(gitSubArgs, expectedGitForEachArgsFn(), []any{"git lfs pull"})...).Once()
					mockWriter.EXPECT().EndIf().Once()

					err := shell.writeSubmoduleUpdateCmd(
						mockWriter,
						&common.Build{
							JobResponse: common.JobResponse{
								GitInfo: common.GitInfo{Depth: tc.Depth},
								Token:   exampleJobToken,
								Variables: common.JobVariables{
									{Key: "GIT_SUBMODULE_UPDATE_FLAGS", Value: tc.GitSubmoduleUpdateFlags},
									{Key: "GIT_CLEAN_FLAGS", Value: tc.GitCleanFlags},
								},
							},
							Runner: &common.RunnerConfig{
								RunnerCredentials: common.RunnerCredentials{URL: exampleBaseURL},
								RunnerSettings: common.RunnerSettings{
									FeatureFlags: map[string]bool{
										featureflags.GitURLsWithoutTokens: useJobTokenFromEnv,
									},
								},
							},
						},
						tc.Recursive,
					)
					assert.NoError(t, err)
				})
			}
		})
	}
}

func getCacheKeyHasher(hash bool) func(string) string {
	if !hash {
		return func(s string) string { return s }
	}
	return func(s string) string {
		return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
	}
}

func TestAbstractShell_extractCacheWithDefaultFallbackKey(t *testing.T) {
	const cacheEnvFile = "/some/path/to/runner-cache-env"

	type expectations struct {
		cacheKeys   []string
		usesEnvFile []bool
		warning     []any
		notices     [][]any
	}
	type hashMode uint8
	const (
		withOrWithoutHashing hashMode = iota
		withoutHashing
		withHashing
	)

	tests := map[string]struct {
		cacheType                string
		cacheKey                 string
		cacheFallbackKeyVarValue string
		expectations             map[hashMode]expectations
	}{
		"using allowed key value": {
			cacheType:                "test",
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: "test-fallback-cache-key",
			expectations: map[hashMode]expectations{
				withOrWithoutHashing: {
					cacheKeys: []string{"test-cache-key", "test-fallback-cache-key"},
				},
			},
		},
		"using sanitized fallback key": {
			cacheType:                "test",
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: `hello.%2e..there  `,
			expectations: map[hashMode]expectations{
				withoutHashing: {
					cacheKeys: []string{"test-cache-key", "hello....there"},
					warning:   []any{`cache key "hello.%2e..there  " sanitized to "hello....there"`},
				},
				withHashing: {
					cacheKeys: []string{"test-cache-key", "hello.%2e..there  "},
				},
			},
		},
		"using something that looks like a windows path": {
			cacheType:                "test",
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: `looks\like\a\win\path`,
			expectations: map[hashMode]expectations{
				withoutHashing: {
					cacheKeys: []string{"test-cache-key", "looks/like/a/win/path"},
					warning:   []any{`cache key "looks\\like\\a\\win\\path" sanitized to "looks/like/a/win/path"`},
				},
				withHashing: {
					cacheKeys: []string{"test-cache-key", "looks\\like\\a\\win\\path"},
				},
			},
		},
		"using path-like fallback cache key": {
			cacheType:                "test",
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: `foo/bar/baz`,
			expectations: map[hashMode]expectations{
				withOrWithoutHashing: {
					cacheKeys: []string{"foo/bar/baz", "test-cache-key"},
				},
			},
		},
		"using invalid fallback cache key": {
			cacheType:                "test",
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: `..`,
			expectations: map[hashMode]expectations{
				withoutHashing: {
					cacheKeys: []string{"test-cache-key"},
					warning:   []any{`cache key ".." could not be sanitized`},
					notices:   [][]any{{`Skipping cache extraction due to %v`, fmt.Errorf("empty cache key")}},
				},
				withHashing: {
					cacheKeys:   []string{"test-cache-key", ".."},
					usesEnvFile: []bool{true, false},
				},
			},
		},
		"using reserved suffix": {
			cacheType:                "test",
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: "main-protected",
			expectations: map[hashMode]expectations{
				withoutHashing: {
					cacheKeys: []string{"test-cache-key"},
					warning:   []any{"CACHE_FALLBACK_KEY %q not allowed to end in %q", "main-protected", "-protected"},
				},
				withHashing: {
					cacheKeys:   []string{"test-cache-key"},
					warning:     []any{"CACHE_FALLBACK_KEY %q not allowed to end in %q", "main-protected", "-protected"},
					usesEnvFile: []bool{true},
				},
			},
		},
		"empty cache key": {
			cacheType: "test",
			expectations: map[hashMode]expectations{
				withOrWithoutHashing: {
					cacheKeys: []string{"some-job-name/some-ref-name"},
				},
			},
		},
		"empty cache key, with fallback": {
			cacheType:                "test",
			cacheFallbackKeyVarValue: "some-fallback",
			expectations: map[hashMode]expectations{
				withOrWithoutHashing: {
					cacheKeys: []string{"some-job-name/some-ref-name", "some-fallback"},
				},
			},
		},
		"empty cache key, with invalid fallback": {
			cacheType:                "test",
			cacheFallbackKeyVarValue: ".",
			expectations: map[hashMode]expectations{
				withoutHashing: {
					cacheKeys: []string{"some-job-name/some-ref-name"},
					warning:   []any{`cache key "." could not be sanitized`},
					notices:   [][]any{{`Skipping cache extraction due to %v`, fmt.Errorf("empty cache key")}},
				},
				withHashing: {
					cacheKeys:   []string{"some-job-name/some-ref-name", "."},
					usesEnvFile: []bool{true, false},
				},
			},
		},
		"GoCloud cache with allowed key value": {
			cacheType:                "goCloudTest",
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: "test-fallback-cache-key",
			expectations: map[hashMode]expectations{
				withOrWithoutHashing: {
					cacheKeys: []string{"test-cache-key", "test-fallback-cache-key"},
				},
			},
		},
		"GoCloud cache with reserved suffix": {
			cacheType:                "goCloudTest",
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: "main-protected",
			expectations: map[hashMode]expectations{
				withOrWithoutHashing: {
					cacheKeys: []string{"test-cache-key"},
					warning:   []any{"CACHE_FALLBACK_KEY %q not allowed to end in %q", "main-protected", "-protected"},
				},
			},
		},
		"GoCloud empty cache key": {
			cacheType: "goCloudTest",
			expectations: map[hashMode]expectations{
				withOrWithoutHashing: {
					cacheKeys: []string{"some-job-name/some-ref-name"},
				},
			},
		},
		"GoCloud empty cache key, with fallback": {
			cacheType:                "goCloudTest",
			cacheFallbackKeyVarValue: "some-fallback",
			expectations: map[hashMode]expectations{
				withOrWithoutHashing: {
					cacheKeys: []string{"some-job-name/some-ref-name", "some-fallback"},
				},
			},
		},
		"GoCloud empty cache key, with invalid fallback": {
			cacheType:                "goCloudTest",
			cacheFallbackKeyVarValue: " /  ",
			expectations: map[hashMode]expectations{
				withoutHashing: {
					cacheKeys: []string{"some-job-name/some-ref-name"},
					warning:   []any{`cache key " /  " could not be sanitized`},
					notices:   [][]any{{`Skipping cache extraction due to %v`, fmt.Errorf("empty cache key")}},
				},
				withHashing: {
					cacheKeys: []string{"some-job-name/some-ref-name", " /  "},
				},
			},
		},
		"cache:key:files": {
			cacheType: "test",
			cacheKey:  "0_project/dependencies-7ab1ff8ddd4179468d07100f16b6f19f91b645a8-non_protected",
			expectations: map[hashMode]expectations{
				withOrWithoutHashing: {
					cacheKeys: []string{`0_project/dependencies-7ab1ff8ddd4179468d07100f16b6f19f91b645a8-non_protected`},
				},
			},
		},
		"cache:key:files sanitized": {
			cacheType: "test",
			cacheKey:  "0_project/foo/../dependencies-7ab1ff8ddd4179468d07100f16b6f19f91b645a8-non_protected",
			expectations: map[hashMode]expectations{
				withoutHashing: {
					cacheKeys: []string{`0_project/dependencies-7ab1ff8ddd4179468d07100f16b6f19f91b645a8-non_protected`},
					warning:   []any{`cache key "0_project/foo/../dependencies-7ab1ff8ddd4179468d07100f16b6f19f91b645a8-non_protected" sanitized to "0_project/dependencies-7ab1ff8ddd4179468d07100f16b6f19f91b645a8-non_protected"`},
				},
				withHashing: {
					cacheKeys: []string{`0_project/foo/../dependencies-7ab1ff8ddd4179468d07100f16b6f19f91b645a8-non_protected`},
				},
			},
		},
	}

	for _, hashCacheKeys := range []bool{false, true} {
		hashed := getCacheKeyHasher(hashCacheKeys)

		t.Run(fmt.Sprintf("%s:%t", featureflags.HashCacheKeys, hashCacheKeys), func(t *testing.T) {
			for tn, tc := range tests {
				t.Run(tn, func(t *testing.T) {
					runnerConfig := &common.RunnerConfig{
						RunnerSettings: common.RunnerSettings{
							Cache: &common.CacheConfig{
								Type:   tc.cacheType,
								Shared: true,
							},
							FeatureFlags: map[string]bool{
								featureflags.HashCacheKeys: hashCacheKeys,
							},
						},
					}
					shell := AbstractShell{}

					build := &common.Build{
						BuildDir: "/builds",
						CacheDir: "/cache",
						Runner:   runnerConfig,
						JobResponse: common.JobResponse{
							ID: 1000,
							JobInfo: common.JobInfo{
								ProjectID: 1000,
								Name:      "some-job-name",
							},
							GitInfo: common.GitInfo{
								Ref: "some-ref-name",
							},
							Cache: common.Caches{
								{
									Key:    tc.cacheKey,
									Policy: common.CachePolicyPullPush,
									Paths:  []string{"path1", "path2"},
								},
							},
							Variables: common.JobVariables{
								{
									Key:   "CACHE_FALLBACK_KEY",
									Value: tc.cacheFallbackKeyVarValue,
								},
							},
						},
					}
					info := common.ShellScriptInfo{
						RunnerCommand: "runner-command",
						Build:         build,
					}

					mockWriter := NewMockShellWriter(t)
					mockWriter.On("IfCmd", "runner-command", "--version").Once()

					var expectations expectations
					if e, ok := tc.expectations[withOrWithoutHashing]; ok {
						expectations = e
					}
					if e, ok := tc.expectations[withHashing]; ok && hashCacheKeys {
						expectations = e
					}
					if e, ok := tc.expectations[withoutHashing]; ok && !hashCacheKeys {
						expectations = e
					}

					for _, expectedCacheKey := range expectations.cacheKeys {
						expectedHashedCacheKey := hashed(expectedCacheKey)

						mockWriter.On("Noticef", "Checking cache for %s...", expectedCacheKey).Once()

						if tc.cacheType == "test" {
							mockWriter.On("IfCmdWithOutput",
								"runner-command",
								"cache-extractor",
								"--file",
								filepath.Join("..", build.CacheDir, expectedHashedCacheKey, "cache.zip"),
								"--timeout",
								"10",
								"--url",
								fmt.Sprintf("test://download/project/1000/%s", expectedHashedCacheKey),
							).Once()
						} else {
							mockWriter.On("DotEnvVariables", "gitlab_runner_cache_env", mock.Anything).Return(cacheEnvFile).Once()
							mockWriter.On("IfCmdWithOutput",
								"runner-command",
								"cache-extractor",
								"--file",
								filepath.Join("..", build.CacheDir, expectedHashedCacheKey, "cache.zip"),
								"--timeout",
								"10",
								"--gocloud-url",
								fmt.Sprintf("gocloud://test/project/1000/%s", expectedHashedCacheKey),
								"--env-file", cacheEnvFile,
							).Once()
						}

						mockWriter.On("Noticef", "Successfully extracted cache").Once()
						mockWriter.On("Else").Once()
						mockWriter.On("Warningf", "Failed to extract cache").Once()
						mockWriter.On("EndIf").Once()

						if tc.cacheType != "test" {
							mockWriter.On("RmFile", cacheEnvFile).Once()
						}
					}

					if w := expectations.warning; len(w) > 0 {
						mockWriter.On("Warningf", w...).Once()
					}
					for _, e := range expectations.notices {
						mockWriter.On("Noticef", e...)
					}

					mockWriter.On("Else").Once()
					mockWriter.On("Warningf", "Missing %s. %s is disabled.", "runner-command", "Extracting cache").Once()
					mockWriter.On("EndIf").Once()

					mockWriter.On("IfFile", "/.gitlab-build-uid-gid").Return(true).Once()
					mockWriter.On("IfDirectory", "/cache").Return(true).Once()
					mockWriter.On("Line", "chown -R \"$(stat -c '%u:%g' '/.gitlab-build-uid-gid')\" '/cache'").
						Return("chown -R \"$(stat -c '%u:%g' '/.gitlab-build-uid-gid')\" '/cache'").
						Once()
					mockWriter.On("EndIf").Once()
					mockWriter.On("EndIf").Once()

					err := shell.cacheExtractor(context.Background(), mockWriter, info)
					assert.NoError(t, err)
				})
			}
		})
	}
}

func TestAbstractShell_extractCacheWithMultipleFallbackKeys(t *testing.T) {
	tests := map[string]struct {
		cacheKey                  string
		cacheFallbackKeyVarValue  string
		cacheFallbackKeysValues   []string
		allowedCacheKeys          []string
		variables                 common.JobVariables
		expectedAdditionalWarning []any
	}{
		"multiple fallback keys": {
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: "test-var-fallback-cache-key",
			cacheFallbackKeysValues: []string{
				"test-fallback-cache-key-1",
				"test-fallback-cache-key-2",
			},
			allowedCacheKeys: []string{
				"test-cache-key",
				"test-fallback-cache-key-1",
				"test-fallback-cache-key-2",
				"test-var-fallback-cache-key",
			},
		},
		"fallback keys with variables": {
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: "test-var-fallback-cache-key",
			cacheFallbackKeysValues: []string{
				"test-fallback-cache-$CACHE_FALLBACK_1",
				"test-fallback-cache-$CACHE_FALLBACK_2",
			},
			allowedCacheKeys: []string{
				"test-cache-key",
				"test-fallback-cache-key-1",
				"test-fallback-cache-key-2",
				"test-var-fallback-cache-key",
			},
			variables: common.JobVariables{
				{
					Key:   "CACHE_FALLBACK_1",
					Value: "key-1",
				},
				{
					Key:   "CACHE_FALLBACK_2",
					Value: "key-2",
				},
			},
		},
		"protected fallback keys": {
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: "test-var-fallback-cache-key",
			cacheFallbackKeysValues: []string{
				"test-fallback-protected-1",
				"test-fallback-protected-2",
			},
			allowedCacheKeys: []string{
				"test-cache-key",
				"test-fallback-protected-1",
				"test-fallback-protected-2",
				"test-var-fallback-cache-key",
			},
		},
		"invalid global protected fallback key": {
			cacheKey:                  "test-cache-key",
			cacheFallbackKeyVarValue:  "test-var-fallback-key-protected",
			expectedAdditionalWarning: []any{"CACHE_FALLBACK_KEY %q not allowed to end in %q", "test-var-fallback-key-protected", "-protected"},
			cacheFallbackKeysValues: []string{
				"test-fallback-cache-key-1",
				"test-fallback-cache-key-2",
			},
			allowedCacheKeys: []string{
				"test-cache-key",
				"test-fallback-cache-key-1",
				"test-fallback-cache-key-2",
			},
		},
		"empty cache key": {
			allowedCacheKeys: []string{
				"some-job-name/some-ref-name",
			},
		},
	}

	for _, hashedCacheKey := range []bool{false, true} {
		hashed := getCacheKeyHasher(hashedCacheKey)

		t.Run(fmt.Sprintf("%s:%t", featureflags.HashCacheKeys, hashedCacheKey), func(t *testing.T) {
			for tn, tc := range tests {
				t.Run(tn, func(t *testing.T) {
					runnerConfig := &common.RunnerConfig{
						RunnerSettings: common.RunnerSettings{
							Cache: &common.CacheConfig{
								Type:   "test",
								Shared: true,
							},
							FeatureFlags: map[string]bool{
								featureflags.HashCacheKeys: hashedCacheKey,
							},
						},
					}
					shell := AbstractShell{}

					variables := common.JobVariables{
						{
							Key:   "CACHE_FALLBACK_KEY",
							Value: tc.cacheFallbackKeyVarValue,
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
								Name:      "some-job-name",
							},
							GitInfo: common.GitInfo{
								Ref: "some-ref-name",
							},
							Cache: common.Caches{
								{
									Key:          tc.cacheKey,
									Policy:       common.CachePolicyPullPush,
									Paths:        []string{"path1", "path2"},
									FallbackKeys: tc.cacheFallbackKeysValues,
								},
							},
							Variables: append(variables, tc.variables...),
						},
					}
					info := common.ShellScriptInfo{
						RunnerCommand: "runner-command",
						Build:         build,
					}

					mockWriter := NewMockShellWriter(t)
					mockWriter.On("IfCmd", "runner-command", "--version").Once()

					for _, cacheKey := range tc.allowedCacheKeys {
						hashedCacheKey := hashed(cacheKey)

						mockWriter.On("Noticef", "Checking cache for %s...", cacheKey).Once()
						mockWriter.On(
							"IfCmdWithOutput",
							"runner-command",
							"cache-extractor",
							"--file",
							filepath.Join("..", build.CacheDir, hashedCacheKey, "cache.zip"),
							"--timeout",
							"10",
							"--url",
							fmt.Sprintf("test://download/project/1000/%s", hashedCacheKey),
						).Once()
						mockWriter.On("Noticef", "Successfully extracted cache").Once()
						mockWriter.On("Else").Once()
						mockWriter.On("Warningf", "Failed to extract cache").Once()
						mockWriter.On("EndIf").Once()
					}

					mockWriter.On("Else").Once()
					mockWriter.On("Warningf", "Missing %s. %s is disabled.", "runner-command", "Extracting cache").Once()
					mockWriter.On("EndIf").Once()

					mockWriter.On("IfFile", "/.gitlab-build-uid-gid").Return(true)
					mockWriter.On("IfDirectory", "/cache").Return(true)
					mockWriter.On("Line", "chown -R \"$(stat -c '%u:%g' '/.gitlab-build-uid-gid')\" '/cache'").
						Return("chown -R \"$(stat -c '%u:%g' '/.gitlab-build-uid-gid')\" '/cache'")
					mockWriter.On("EndIf").Once()
					mockWriter.On("EndIf").Once()

					if w := tc.expectedAdditionalWarning; len(w) > 0 {
						mockWriter.On("Warningf", w...).Once()
					}

					err := shell.cacheExtractor(context.Background(), mockWriter, info)
					assert.NoError(t, err)
				})
			}
		})
	}
}

// As above, but enabling FF_CLEAN_UP_FAILED_CACHE_EXTRACT.
func TestAbstractShell_extractCacheWithMultipleFallbackKeysWithCleanup(t *testing.T) {
	tests := map[string]struct {
		cacheKey                  string
		cacheFallbackKeyVarValue  string
		cacheFallbackKeysValues   []string
		allowedCacheKeys          []string
		variables                 common.JobVariables
		expectedAdditionalWarning []any
	}{
		"multiple fallback keys": {
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: "test-var-fallback-cache-key",
			cacheFallbackKeysValues: []string{
				"test-fallback-cache-key-1",
				"test-fallback-cache-key-2",
			},
			allowedCacheKeys: []string{
				"test-cache-key",
				"test-fallback-cache-key-1",
				"test-fallback-cache-key-2",
				"test-var-fallback-cache-key",
			},
		},
		"fallback keys with variables": {
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: "test-var-fallback-cache-key",
			cacheFallbackKeysValues: []string{
				"test-fallback-cache-$CACHE_FALLBACK_1",
				"test-fallback-cache-$CACHE_FALLBACK_2",
			},
			allowedCacheKeys: []string{
				"test-cache-key",
				"test-fallback-cache-key-1",
				"test-fallback-cache-key-2",
				"test-var-fallback-cache-key",
			},
			variables: common.JobVariables{
				{
					Key:   "CACHE_FALLBACK_1",
					Value: "key-1",
				},
				{
					Key:   "CACHE_FALLBACK_2",
					Value: "key-2",
				},
			},
		},
		"protected fallback keys": {
			cacheKey:                 "test-cache-key",
			cacheFallbackKeyVarValue: "test-var-fallback-cache-key",
			cacheFallbackKeysValues: []string{
				"test-fallback-protected-1",
				"test-fallback-protected-2",
			},
			allowedCacheKeys: []string{
				"test-cache-key",
				"test-fallback-protected-1",
				"test-fallback-protected-2",
				"test-var-fallback-cache-key",
			},
		},
		"invalid global protected fallback key": {
			cacheKey:                  "test-cache-key",
			cacheFallbackKeyVarValue:  "test-var-fallback-key-protected",
			expectedAdditionalWarning: []any{"CACHE_FALLBACK_KEY %q not allowed to end in %q", "test-var-fallback-key-protected", "-protected"},
			cacheFallbackKeysValues: []string{
				"test-fallback-cache-key-1",
				"test-fallback-cache-key-2",
			},
			allowedCacheKeys: []string{
				"test-cache-key",
				"test-fallback-cache-key-1",
				"test-fallback-cache-key-2",
			},
		},
		"empty cache key": {
			allowedCacheKeys: []string{
				"some-job-name/some-ref-name",
			},
		},
	}

	for _, hashedCacheKey := range []bool{false, true} {
		hashed := getCacheKeyHasher(hashedCacheKey)

		t.Run(fmt.Sprintf("%s:%t", featureflags.HashCacheKeys, hashedCacheKey), func(t *testing.T) {
			for tn, tc := range tests {
				t.Run(tn, func(t *testing.T) {
					runnerConfig := &common.RunnerConfig{
						RunnerSettings: common.RunnerSettings{
							Cache: &common.CacheConfig{
								Type:   "test",
								Shared: true,
							},
							FeatureFlags: map[string]bool{
								featureflags.HashCacheKeys: hashedCacheKey,
							},
						},
					}
					shell := AbstractShell{}

					variables := common.JobVariables{
						{
							Key:   "CACHE_FALLBACK_KEY",
							Value: tc.cacheFallbackKeyVarValue,
						},
						{
							Key:   "FF_CLEAN_UP_FAILED_CACHE_EXTRACT",
							Value: "true",
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
								Name:      "some-job-name",
							},
							GitInfo: common.GitInfo{
								Ref: "some-ref-name",
							},
							Cache: common.Caches{
								{
									Key:          tc.cacheKey,
									Policy:       common.CachePolicyPullPush,
									Paths:        []string{"path1", "path2"},
									FallbackKeys: tc.cacheFallbackKeysValues,
								},
							},
							Variables: append(variables, tc.variables...),
						},
					}
					info := common.ShellScriptInfo{
						RunnerCommand: "runner-command",
						Build:         build,
					}

					mockWriter := NewMockShellWriter(t)
					mockWriter.On("IfCmd", "runner-command", "--version").Once()

					for _, cacheKey := range tc.allowedCacheKeys {
						hashedCacheKey := hashed(cacheKey)

						mockWriter.On("Noticef", "Checking cache for %s...", cacheKey).Once()
						mockWriter.On(
							"IfCmdWithOutput",
							"runner-command",
							"cache-extractor",
							"--file",
							filepath.Join("..", build.CacheDir, hashedCacheKey, "cache.zip"),
							"--timeout",
							"10",
							"--url",
							fmt.Sprintf("test://download/project/1000/%s", hashedCacheKey),
						).Once()
						mockWriter.On("Noticef", "Successfully extracted cache").Once()
						mockWriter.On("Else").Once()
						mockWriter.On("Warningf", "Failed to extract cache").Once()
						mockWriter.On("Printf", "Removing %s", "path1").Once()
						mockWriter.On("RmDir", "path1").Once()
						mockWriter.On("Printf", "Removing %s", "path2").Once()
						mockWriter.On("RmDir", "path2").Once()
					}

					for range tc.allowedCacheKeys {
						mockWriter.On("EndIf").Once()
					}

					if w := tc.expectedAdditionalWarning; len(w) > 0 {
						mockWriter.On("Warningf", w...).Once()
					}

					mockWriter.On("Else").Once()
					mockWriter.On("Warningf", "Missing %s. %s is disabled.", "runner-command", "Extracting cache").Once()
					mockWriter.On("EndIf").Once()

					mockWriter.On("IfFile", "/.gitlab-build-uid-gid").Return(true)
					mockWriter.On("IfDirectory", "/cache").Return(true)
					mockWriter.On("Line", "chown -R \"$(stat -c '%u:%g' '/.gitlab-build-uid-gid')\" '/cache'").
						Return("chown -R \"$(stat -c '%u:%g' '/.gitlab-build-uid-gid')\" '/cache'")
					mockWriter.On("EndIf").Once()
					mockWriter.On("EndIf").Once()

					err := shell.cacheExtractor(context.Background(), mockWriter, info)
					assert.NoError(t, err)
				})
			}
		})
	}
}

func TestAbstractShell_cachePolicy(t *testing.T) {
	tests := map[string]struct {
		cacheKey  string
		policy    string
		variables common.JobVariables
		errMsg    string
	}{
		"using pull policy": {
			cacheKey:  "test-cache-key",
			policy:    "pull",
			variables: common.JobVariables{},
		},
		"using push policy": {
			cacheKey:  "test-cache-key",
			policy:    "push",
			variables: common.JobVariables{},
		},
		"using pull-push policy": {
			cacheKey:  "test-cache-key",
			policy:    "pull-push",
			variables: common.JobVariables{},
		},
		"using variable with pull-push value": {
			cacheKey: "test-cache-key",
			policy:   "$CACHE_POLICY",
			variables: common.JobVariables{
				{
					Key:   "CACHE_POLICY",
					Value: "pull-push",
				},
			},
		},
		"using variable with invalid value": {
			cacheKey: "test-cache-key",
			policy:   "$CACHE_POLICY",
			variables: common.JobVariables{
				{
					Key:   "CACHE_POLICY",
					Value: "blah",
				},
			},
			errMsg: "unknown cache policy blah for test-cache-key",
		},
		"using hardcoded value matching variable name": {
			cacheKey: "test-cache-key",
			policy:   "CACHE_POLICY",
			variables: common.JobVariables{
				{
					Key:   "CACHE_POLICY",
					Value: "pull",
				},
			},
			errMsg: "unknown cache policy CACHE_POLICY for test-cache-key",
		},
		"empty key": {
			cacheKey: "",
		},
	}

	type cacheFunc func(AbstractShell, common.ShellScriptInfo) error

	functions := map[string]cacheFunc{
		"cacheExtractor": func(shell AbstractShell, info common.ShellScriptInfo) error {
			return shell.cacheExtractor(context.Background(), &BashWriter{}, info)
		},
		"cacheArchiver": func(shell AbstractShell, info common.ShellScriptInfo) error {
			return shell.cacheArchiver(context.Background(), &BashWriter{}, info, true)
		},
	}

	runnerConfig := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Cache: &common.CacheConfig{
				Type:   "test",
				Shared: true,
			},
		},
	}
	shell := AbstractShell{}

	for ft, fn := range functions {
		for tn, tc := range tests {
			t.Run(fmt.Sprintf("%s-%s", ft, tn), func(t *testing.T) {
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
								Key:    tc.cacheKey,
								Policy: common.CachePolicy(tc.policy),
								Paths:  []string{"path1", "path2"},
							},
						},
						Variables: tc.variables,
					},
				}
				info := common.ShellScriptInfo{
					RunnerCommand: "runner-command",
					Build:         build,
				}

				err := fn(shell, info)

				if tc.errMsg == "" {
					assert.NoError(t, err)
				} else {
					assert.ErrorContains(t, err, tc.errMsg)
				}
			})
		}
	}
}

func TestAbstractShell_archiveCache_keySanitation(t *testing.T) {
	type expectations struct {
		skipUpload        bool
		sanitationWarning string
		cacheKey          string
		notice            []any
	}
	type hashMode uint8
	const (
		withOrWithoutHashing hashMode = iota
		withoutHashing
		withHashing

		cacheEnvFile = "/some/path/to/runner-cache-env"
	)

	tests := map[string]struct {
		rawCacheKey  string
		jobName      string
		gitRef       string
		expectations map[hashMode]expectations
	}{
		"defaulted cache key": {
			jobName: "some-job-name",
			gitRef:  "some-git-ref",
			expectations: map[hashMode]expectations{
				withOrWithoutHashing: {
					cacheKey: "some-job-name/some-git-ref",
				},
			},
		},
		"defaulted cache key sanitized": {
			jobName: `some\job\name`,
			gitRef:  "some/git/ref",
			expectations: map[hashMode]expectations{
				withoutHashing: {
					cacheKey:          "some/job/name/some/git/ref",
					sanitationWarning: `cache key "some\\job\\name/some/git/ref" sanitized to "some/job/name/some/git/ref"`,
				},
				withHashing: {
					cacheKey: "some\\job\\name/some/git/ref",
				},
			},
		},
		"cache key": {
			rawCacheKey: "hola",
			expectations: map[hashMode]expectations{
				withOrWithoutHashing: {
					cacheKey: "hola",
				},
			},
		},
		"cache key sanitized": {
			rawCacheKey: `this/../key/will/be\sanitized\  `,
			expectations: map[hashMode]expectations{
				withoutHashing: {
					cacheKey:          "key/will/be/sanitized",
					sanitationWarning: `cache key "this/../key/will/be\\sanitized\\  " sanitized to "key/will/be/sanitized"`,
				},
				withHashing: {
					cacheKey: `this/../key/will/be\sanitized\  `,
				},
			},
		},
		"cannot be sanitized": {
			rawCacheKey: "/",
			expectations: map[hashMode]expectations{
				withoutHashing: {
					skipUpload:        true,
					sanitationWarning: `cache key "/" could not be sanitized`,
					notice:            []any{`Skipping cache archiving due to %v`, fmt.Errorf("empty cache key")},
				},
				withHashing: {
					cacheKey: `/`,
				},
			},
		},
	}

	for _, hashCacheKeys := range []bool{false, true} {
		hashed := getCacheKeyHasher(hashCacheKeys)

		t.Run(fmt.Sprintf("%s:%t", featureflags.HashCacheKeys, hashCacheKeys), func(t *testing.T) {
			for name, test := range tests {
				t.Run(name, func(t *testing.T) {
					info := common.ShellScriptInfo{
						RunnerCommand: "some-runner-command",
						Build: &common.Build{
							CacheDir: "/some/cacheDir",
							BuildDir: "/some/buildDir",
							Runner: &common.RunnerConfig{
								RunnerSettings: common.RunnerSettings{
									FeatureFlags: map[string]bool{featureflags.HashCacheKeys: hashCacheKeys},
								},
							},
							JobResponse: common.JobResponse{
								JobInfo: common.JobInfo{Name: test.jobName},
								GitInfo: common.GitInfo{Ref: test.gitRef},
								Cache: common.Caches{
									{
										When:  common.CacheWhenAlways,
										Paths: common.ArtifactPaths{"foo/bar", "foo/barz"},
										Key:   test.rawCacheKey,
									},
								},
							},
						},
					}
					shell := AbstractShell{}
					w := NewMockShellWriter(t)

					var expectations expectations
					if e, ok := test.expectations[withOrWithoutHashing]; ok {
						expectations = e
					}
					if e, ok := test.expectations[withHashing]; hashCacheKeys && ok {
						expectations = e
					}
					if e, ok := test.expectations[withoutHashing]; !hashCacheKeys && ok {
						expectations = e
					}

					if warning := expectations.sanitationWarning; warning != "" {
						w.On("Warningf", warning).Once()
					}

					if notice := expectations.notice; len(notice) > 0 {
						w.On("Noticef", notice...).Once()
					}

					if !expectations.skipUpload {
						expectedLocalFile := filepath.Join(
							"../cacheDir", hashed(expectations.cacheKey), "cache.zip",
						)

						w.On("IfCmd", "some-runner-command", "--version").Once()
						w.On("Noticef", "Creating cache %s...", expectations.cacheKey).Once()

						{ // cache metadata passing
							expectedMetadata := map[string]string{
								"cachekey": expectations.cacheKey,
							}
							metadataBlob, err := json.Marshal(expectedMetadata)
							require.NoError(t, err, "marshalling expected cache metadata")
							expectedEnvs := map[string]string{
								"CACHE_METADATA": string(metadataBlob),
							}
							w.On("DotEnvVariables", "gitlab_runner_cache_env", expectedEnvs).Return(cacheEnvFile).Once()
							w.On("RmFile", cacheEnvFile).Once()
						}

						w.On("IfCmdWithOutput",
							"some-runner-command", "cache-archiver",
							"--file", expectedLocalFile,
							"--timeout", "10",
							"--path", "foo/bar",
							"--path", "foo/barz",
							"--env-file", cacheEnvFile,
						).Once()
						w.On("Noticef", "Created cache").Once()
						w.On("Else").Once()
						w.On("Warningf", "Failed to create cache").Once()
						w.On("EndIf").Once()
						w.On("Else").Once()
						w.On("Warningf", "Missing %s. %s is disabled.", "some-runner-command", "Creating cache").Once()
						w.On("EndIf")
					}

					_, err := shell.archiveCache(context.TODO(), w, info, true)

					assert.NoError(t, err, "expected achiveCache to succeed")
				})
			}
		})
	}
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
		var command []interface{}

		for _, a := range args {
			command = append(command, a)
		}

		subpaths := strings.Fields(paths)
		if len(subpaths) != 0 {
			command = append(command, "--")
			for i := 0; i < len(subpaths); i++ {
				command = append(command, subpaths[i])
			}
		}

		return command
	}

	insteadOf := "url.https://gitlab-ci-token:xxx@example.com.insteadOf=https://example.com"

	for _, useJobTokenFromEnv := range []bool{true, false} {
		name := fmt.Sprintf("%s:%t", featureflags.GitURLsWithoutTokens, useJobTokenFromEnv)
		t.Run(name, func(t *testing.T) {
			for name, test := range tests {
				t.Run(name, func(t *testing.T) {
					shell := AbstractShell{}
					mockWriter := NewMockShellWriter(t)

					gitSubArgs := []string{"-c", insteadOf}
					if useJobTokenFromEnv {
						gitSubArgs = []string{"-c", "include.path=/some/file.conf"}
					}

					forEachCmd := func(cmd string) []any {
						args := []any{}
						for _, s := range gitSubArgs {
							args = append(args, s)
						}
						return append(args, "submodule", "foreach", cmd)
					}

					mockWriter.EXPECT().Noticef("Updating/initializing submodules...").Once()
					mockWriter.EXPECT().Command("git", "submodule", "init").Once()
					mockWriter.EXPECT().Command("git", submoduleCommand(test.paths, "submodule", "sync")...).Times(3)

					if useJobTokenFromEnv {
						mockWriter.EXPECT().TmpFile(mock.Anything).Return("/some/file.conf").Once()
					}

					mockWriter.EXPECT().IfCmdWithOutput("git", submoduleCommand(test.paths, append(gitSubArgs, "submodule", "update", "--init")...)...).Once()

					mockWriter.EXPECT().Noticef("Updated submodules").Once()
					mockWriter.EXPECT().Else().Once()
					mockWriter.EXPECT().Warningf("Updating submodules failed. Retrying...").Once()

					// git submodule foreach 'git fetch origin +refs/heads/*:refs/remotes/origin/*' is only called when the
					// `--remote` flag is actually used.
					mockWriter.EXPECT().Command("git", submoduleCommand(test.paths, append(gitSubArgs, "submodule", "update", "--init")...)...).Once()

					mockWriter.EXPECT().EndIf().Once()

					cleanCmd := mockWriter.EXPECT().Command("git", "submodule", "foreach", "git clean -ffdx").Once()
					mockWriter.EXPECT().Command("git", "submodule", "foreach", "git reset --hard").Run(func(command string, arguments ...string) {
						cleanCmd.Once()
					}).Twice()

					mockWriter.EXPECT().IfCmd("git", "lfs", "version").Once()
					mockWriter.EXPECT().Command("git", forEachCmd("git lfs pull")...).Once()
					mockWriter.EXPECT().EndIf().Once()

					build := &common.Build{
						JobResponse: common.JobResponse{Token: "xxx"},
						Runner: &common.RunnerConfig{
							RunnerCredentials: common.RunnerCredentials{URL: "https://example.com"},
							RunnerSettings: common.RunnerSettings{
								FeatureFlags: map[string]bool{
									featureflags.GitURLsWithoutTokens: useJobTokenFromEnv,
								},
							},
						},
					}
					build.Variables = append(build.Variables, common.JobVariable{Key: "GIT_SUBMODULE_PATHS", Value: test.paths})
					err := shell.writeSubmoduleUpdateCmd(mockWriter, build, false)
					assert.NoError(t, err)
				})
			}
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
				m.On("TmpFile", "gitlab_runner_env").Return("path/to/env/file").Once()
				m.On("SourceEnv", "path/to/env/file").Once()
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
				m.On("TmpFile", "gitlab_runner_env").Return("path/to/env/file").Once()
				m.On("SourceEnv", "path/to/env/file").Once()
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
					Runner: &common.RunnerConfig{},
				},
				PostBuildScript: tt.postBuildScript,
			}
			mockShellWriter := NewMockShellWriter(t)
			tt.setupExpectations(mockShellWriter)
			shell := AbstractShell{}

			err := shell.writeUserScript(mockShellWriter, info, tt.buildStage)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestScriptSections(t *testing.T) {
	tests := []struct {
		inputSteps        common.Steps
		setupExpectations func(*MockShellWriter)
		featureFlagOn     bool
		traceSections     bool
	}{
		{
			featureFlagOn: true,
			traceSections: true,
			inputSteps: common.Steps{
				common.Step{
					Name: common.StepNameScript,
					Script: common.StepScript{`Multi line
					script 1`, `Multi line
					script 2`, `Multi line
					script 3`},
				},
			},
			setupExpectations: func(m *MockShellWriter) {
				m.On("Variable", mock.Anything)
				m.On("TmpFile", "gitlab_runner_env").Return("path/to/env/file").Once()
				m.On("SourceEnv", "path/to/env/file").Once()
				m.On("Cd", mock.AnythingOfType("string"))
				m.On("Noticef", "$ %s", "echo prebuild").Once()
				m.On(
					"SectionStart",
					mock.AnythingOfType("string"),
					"$ Multi line\n\t\t\t\t\tscript 1",
					mock.AnythingOfType("[]string"),
				).Once()
				m.On("SectionEnd", mock.AnythingOfType("string")).Once()
				m.On(
					"SectionStart",
					mock.AnythingOfType("string"),
					"$ Multi line\n\t\t\t\t\tscript 2",
					mock.AnythingOfType("[]string"),
				).Once()
				m.On("SectionEnd", mock.AnythingOfType("string")).Once()
				m.On(
					"SectionStart",
					mock.AnythingOfType("string"),
					"$ Multi line\n\t\t\t\t\tscript 3",
					mock.AnythingOfType("[]string"),
				).Once()
				m.On("SectionEnd", mock.AnythingOfType("string")).Once()
				m.On("Noticef", "$ %s", "echo postbuild").Once()
				m.On("Line", "echo prebuild").Once()
				m.On("Line", "Multi line\n\t\t\t\t\tscript 1").Once()
				m.On("Line", "Multi line\n\t\t\t\t\tscript 2").Once()
				m.On("Line", "Multi line\n\t\t\t\t\tscript 3").Once()
				m.On("Line", "echo postbuild").Once()
				m.On("CheckForErrors").Times(5)
			},
		},
		{
			featureFlagOn: true,
			traceSections: true,
			inputSteps: common.Steps{
				common.Step{
					Name:   common.StepNameScript,
					Script: common.StepScript{"script 1", "script 2", "script 3"},
				},
			},
			setupExpectations: func(m *MockShellWriter) {
				m.On("Variable", mock.Anything)
				m.On("TmpFile", "gitlab_runner_env").Return("path/to/env/file").Once()
				m.On("SourceEnv", "path/to/env/file").Once()
				m.On("Cd", mock.AnythingOfType("string"))
				m.On("Noticef", "$ %s", "echo prebuild").Once()
				m.On("Noticef", "$ %s", "script 1").Once()
				m.On("Noticef", "$ %s", "script 2").Once()
				m.On("Noticef", "$ %s", "script 3").Once()
				m.On("Noticef", "$ %s", "echo postbuild").Once()
				m.On("Line", "echo prebuild").Once()
				m.On("Line", "script 1").Once()
				m.On("Line", "script 2").Once()
				m.On("Line", "script 3").Once()
				m.On("Line", "echo postbuild").Once()
				m.On("CheckForErrors").Times(5)
			},
		},
		{
			featureFlagOn: false,
			traceSections: false,
			inputSteps: common.Steps{
				common.Step{
					Name:   common.StepNameScript,
					Script: common.StepScript{"script 1", "script 2", "script 3"},
				},
			},
			setupExpectations: func(m *MockShellWriter) {
				m.On("Variable", mock.Anything)
				m.On("TmpFile", "gitlab_runner_env").Return("path/to/env/file").Once()
				m.On("SourceEnv", "path/to/env/file").Once()
				m.On("Cd", mock.AnythingOfType("string"))
				m.On("Noticef", "$ %s", "echo prebuild").Once()
				m.On("Noticef", "$ %s", "script 1").Once()
				m.On("Noticef", "$ %s", "script 2").Once()
				m.On("Noticef", "$ %s", "script 3").Once()
				m.On("Noticef", "$ %s", "echo postbuild").Once()
				m.On("Line", "echo prebuild").Once()
				m.On("Line", "script 1").Once()
				m.On("Line", "script 2").Once()
				m.On("Line", "script 3").Once()
				m.On("Line", "echo postbuild").Once()
				m.On("CheckForErrors").Times(5)
			},
		},
		{
			featureFlagOn: true,
			traceSections: false,
			inputSteps: common.Steps{
				common.Step{
					Name:   common.StepNameScript,
					Script: common.StepScript{"script 1", "script 2", "script 3"},
				},
			},
			setupExpectations: func(m *MockShellWriter) {
				m.On("Variable", mock.Anything)
				m.On("TmpFile", "gitlab_runner_env").Return("path/to/env/file").Once()
				m.On("SourceEnv", "path/to/env/file").Once()
				m.On("Cd", mock.AnythingOfType("string"))
				m.On("Noticef", "$ %s", "echo prebuild").Once()
				m.On("Noticef", "$ %s", "script 1").Once()
				m.On("Noticef", "$ %s", "script 2").Once()
				m.On("Noticef", "$ %s", "script 3").Once()
				m.On("Noticef", "$ %s", "echo postbuild").Once()
				m.On("Line", "echo prebuild").Once()
				m.On("Line", "script 1").Once()
				m.On("Line", "script 2").Once()
				m.On("Line", "script 3").Once()
				m.On("Line", "echo postbuild").Once()
				m.On("CheckForErrors").Times(5)
			},
		},
		{
			featureFlagOn: false,
			traceSections: true,
			inputSteps: common.Steps{
				common.Step{
					Name:   common.StepNameScript,
					Script: common.StepScript{"script 1", "script 2", "script 3"},
				},
			},
			setupExpectations: func(m *MockShellWriter) {
				m.On("Variable", mock.Anything)
				m.On("TmpFile", "gitlab_runner_env").Return("path/to/env/file").Once()
				m.On("SourceEnv", "path/to/env/file").Once()
				m.On("Cd", mock.AnythingOfType("string"))
				m.On("Noticef", "$ %s", "echo prebuild").Once()
				m.On("Noticef", "$ %s", "script 1").Once()
				m.On("Noticef", "$ %s", "script 2").Once()
				m.On("Noticef", "$ %s", "script 3").Once()
				m.On("Noticef", "$ %s", "echo postbuild").Once()
				m.On("Line", "echo prebuild").Once()
				m.On("Line", "script 1").Once()
				m.On("Line", "script 2").Once()
				m.On("Line", "script 3").Once()
				m.On("Line", "echo postbuild").Once()
				m.On("CheckForErrors").Times(5)
			},
		},
	}

	for _, tt := range tests {
		t.Run(
			fmt.Sprintf("feature flag %t, trace sections %t", tt.featureFlagOn, tt.traceSections),
			func(t *testing.T) {
				info := common.ShellScriptInfo{
					PreBuildScript: "echo prebuild",
					Build: &common.Build{
						JobResponse: common.JobResponse{
							Steps: tt.inputSteps,
							Features: common.GitlabFeatures{
								TraceSections: tt.traceSections,
							},
						},
						Runner: &common.RunnerConfig{RunnerSettings: common.RunnerSettings{
							FeatureFlags: map[string]bool{featureflags.ScriptSections: tt.featureFlagOn},
						}},
					},
					PostBuildScript: "echo postbuild",
				}
				mockShellWriter := NewMockShellWriter(t)

				tt.setupExpectations(mockShellWriter)
				shell := AbstractShell{}

				assert.NoError(t, shell.writeUserScript(mockShellWriter, info, common.BuildStage("step_script")))
			},
		)
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
					err := shell.writeScript(context.Background(), &BashWriter{}, stage, info)
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
					err = shell.writeScript(context.Background(), &BashWriter{}, stage, info)
					assert.NoError(t, err, "stage %v should not have been skipped", stage)
				})
			}
		})
	}
}

func TestAbstractShell_writeCleanupScript(t *testing.T) {
	testVar1 := "VAR_1"
	testVar2 := "VAR_2"
	testVar3 := "VAR_3"
	testVar4 := "VAR_4"

	testPath1 := "path/VAR_1_file"
	testPath3 := "path/VAR_3_file"

	someTrue, someFalse := true, false
	type executorName = string

	tests := map[executorName]map[string]struct {
		cleanGitConfig       *bool
		gitStrategy          string
		shouldCleanGitConfig bool
	}{
		"shell": {
			"no clean-git-config set": {
				shouldCleanGitConfig: false,
			},
			"clean-git-config explicitly enabled": {
				cleanGitConfig:       &someTrue,
				shouldCleanGitConfig: true,
			},
			"clean-git-config explicitly disabled": {
				cleanGitConfig:       &someFalse,
				shouldCleanGitConfig: false,
			},
		},
		"not-shell": {
			"no clean-git-config set": {
				shouldCleanGitConfig: true,
			},
			"no clean-git-config set, but git strategy is none": {
				shouldCleanGitConfig: false,
				gitStrategy:          "none",
			},
			"clean-git-config explicitly enabled": {
				cleanGitConfig:       &someTrue,
				gitStrategy:          "none",
				shouldCleanGitConfig: true,
			},
			"clean-git-config explicitly disabled": {
				cleanGitConfig:       &someFalse,
				shouldCleanGitConfig: false,
			},
		},
	}

	for executorName, testCases := range tests {
		t.Run("executor:"+executorName, func(t *testing.T) {
			for name, test := range testCases {
				t.Run(name, func(t *testing.T) {
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
							Runner: &common.RunnerConfig{
								RunnerSettings: common.RunnerSettings{
									CleanGitConfig: test.cleanGitConfig,
									Executor:       executorName,
									Environment:    []string{"GIT_STRATEGY=" + test.gitStrategy},
								},
							},
						},
					}

					mockShellWriter := NewMockShellWriter(t)
					mockShellWriter.On("TmpFile", "masking.db").Return("masking.db").Once()
					mockShellWriter.On("RmFile", "masking.db").Once()

					mockShellWriter.On("TmpFile", testVar1).Return(testPath1).Once()
					mockShellWriter.On("RmFile", testPath1).Once()
					mockShellWriter.On("TmpFile", testVar3).Return(testPath3).Once()
					mockShellWriter.On("RmFile", testPath3).Once()

					mockShellWriter.On("TmpFile", "gitlab_runner_env").Return("temp_env").Once()
					mockShellWriter.On("RmFile", "temp_env").Once()

					expectFileCleanup(mockShellWriter, ".git", false)

					if test.shouldCleanGitConfig {
						expectGitConfigCleanup(mockShellWriter, "", false)
					}

					mockShellWriter.On("RmFile", "/.gitlab-build-uid-gid").Once()
					mockShellWriter.On("Join", "", ".gitlab-build-uid-gid").Return("/.gitlab-build-uid-gid").Once()

					shell := new(AbstractShell)

					err := shell.writeCleanupScript(context.Background(), mockShellWriter, info)
					assert.NoError(t, err)
				})
			}
		})
	}
}

func testGenerateArtifactsMetadataData() (common.ShellScriptInfo, []interface{}) {
	info := common.ShellScriptInfo{
		Build: &common.Build{
			JobResponse: common.JobResponse{
				Variables: common.JobVariables{
					{Key: "CI_RUNNER_ID", Value: "1000"},
					{Key: "TEST_VARIABLE", Value: ""},
					{Key: "SLSA_PROVENANCE_SCHEMA_VERSION", Value: "v1"},
				},
				GitInfo: common.GitInfo{
					RepoURL: "https://gitlab.com/my/repo.git",
					Sha:     "testsha",
				},
				JobInfo: common.JobInfo{
					Name: "testjob",
				},
			},
			Runner: &common.RunnerConfig{
				Name: "testrunner",
				RunnerSettings: common.RunnerSettings{
					Executor: "testexecutor",
				},
			},
		},
	}

	parseRFC3339Mock := func(t string) bool {
		_, err := time.Parse(time.RFC3339, t)
		return err == nil
	}

	expected := []interface{}{
		"--generate-artifacts-metadata",
		"--runner-id",
		"1000",
		"--repo-url",
		"https://gitlab.com/my/repo",
		"--repo-digest",
		"testsha",
		"--job-name",
		"testjob",
		"--executor-name",
		"testexecutor",
		"--runner-name",
		"testrunner",
		"--started-at",
		mock.MatchedBy(parseRFC3339Mock),
		"--ended-at",
		mock.MatchedBy(parseRFC3339Mock),
		"--schema-version",
		"v1",
		"--metadata-parameter",
		"CI_RUNNER_ID",
		"--metadata-parameter",
		"TEST_VARIABLE",
		"--metadata-parameter",
		"SLSA_PROVENANCE_SCHEMA_VERSION",
		"--metadata-parameter",
		"RUNNER_GENERATE_ARTIFACTS_METADATA",
	}

	return info, expected
}

func TestWriteUploadArtifactIncludesGenerateArtifactsMetadataArgs(t *testing.T) {
	info, expectedMetadataArgs := testGenerateArtifactsMetadataData()

	info.Build.Runner.URL = "testurl"
	info.Build.Token = "testtoken"
	info.Build.ID = 1000
	info.RunnerCommand = "testcommand"
	info.Build.Variables = append(
		info.Build.Variables,
		common.JobVariable{Key: common.GenerateArtifactsMetadataVariable, Value: "true"},
	)

	uploaderArgs := []interface{}{
		"artifacts-uploader",
		"--url",
		"testurl",
		"--token",
		"testtoken",
		"--id",
		"1000",
	}

	for _, f := range []common.ArtifactFormat{
		common.ArtifactFormatZip,
		common.ArtifactFormatGzip,
		common.ArtifactFormatRaw,
		common.ArtifactFormatDefault,
	} {
		t.Run(string(f), func(t *testing.T) {
			args := []interface{}{"testcommand"}
			args = append(args, uploaderArgs...)

			if f == common.ArtifactFormatZip {
				args = append(args, expectedMetadataArgs...)
			}

			args = append(args, "--path", "testpath")

			if f != common.ArtifactFormatDefault {
				args = append(args, "--artifact-format", string(f))
			}

			shellWriter := NewMockShellWriter(t)
			shellWriter.On("IfCmd", mock.Anything, mock.Anything).Once()
			shellWriter.On("Noticef", "Uploading artifacts...").Once()
			shellWriter.On("Command", args...).Once()
			shellWriter.On("Else").Once()
			shellWriter.On("Warningf", mock.Anything, mock.Anything, mock.Anything).Once()
			shellWriter.On("EndIf").Once()

			shell := &AbstractShell{}
			shell.writeUploadArtifact(shellWriter, info, common.Artifact{
				Paths:  []string{"testpath"},
				Format: f,
			})
		})
	}
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
		common.BuildStageCleanup,
		common.BuildStage("step_release"),
	}

	shells := []common.Shell{
		&BashShell{Shell: "sh"},
		&BashShell{Shell: "bash"},
		&PowerShell{Shell: SNPwsh, EOL: "\n"},
		&PowerShell{Shell: SNPowershell, EOL: "\r\n"},
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
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		script, err := shell.GenerateScript(ctx, stage, info)
		b.SetBytes(int64(len(script)))
		assert.NoError(b, err, stage)
	}
}

func expectSetupTemplate(shellWriter *MockShellWriter, dir string) (string, []*mock.Call) {
	templateDir := "git-template-dir"
	config := path.Join(templateDir, "config")

	calls := []*mock.Call{
		shellWriter.EXPECT().MkTmpDir("git-template").Return(templateDir).Once(),
		shellWriter.EXPECT().Command("git", "config", "--global", "--add", "safe.directory", dir).Maybe(),
		shellWriter.EXPECT().Join("git-template-dir", "config").Return(config).Once(),
		shellWriter.EXPECT().Command("git", "config", "-f", config, "init.defaultBranch", "none").Once(),
		shellWriter.EXPECT().Command("git", "config", "-f", config, "fetch.recurseSubmodules", "false").Once(),
		shellWriter.EXPECT().Command("git", "config", "-f", config, "credential.interactive", "never").Once(),
		shellWriter.EXPECT().Command("git", "config", "-f", mock.Anything, "gc.autoDetach", "false").Once(),
		shellWriter.EXPECT().Command("git", "config", "-f", config, "transfer.bundleURI", "true").Maybe(),
		shellWriter.EXPECT().Command("git", "config", "-f", config, "http.extraHeader", "X-Gitaly-Correlation-ID: foobar").Maybe(),
		shellWriter.EXPECT().Noticef("Gitaly correlation ID: %s", "foobar").Maybe(),
	}
	mock.InOrder(calls...)

	return templateDir, calls
}

func expectFileCleanup(shellWriter *MockShellWriter, dir string, withSubmodules bool) {
	files := []string{"index.lock", "shallow.lock", "HEAD.lock", "hooks/post-checkout", "config.lock"}

	for _, f := range files {
		shellWriter.EXPECT().RmFile(dir + "/" + f).Once()
	}

	if withSubmodules {
		for _, f := range files {
			shellWriter.EXPECT().RmFilesRecursive(dir+"/modules", filepath.Base(f)).Once()
		}
	}

	shellWriter.EXPECT().RmFilesRecursive(dir+"/refs", "*.lock").Once()
}

func expectGitCredHelperSetup(shellWriter *MockShellWriter, remoteURL string) string {
	someHelperPath := "/some/path/cred-helper.conf"

	expectedCredSection := "credential." + remoteURL
	expectedUsername := "gitlab-ci-token"

	shellWriter.EXPECT().TmpFile(mock.Anything).Return(someHelperPath).Once()

	shellWriter.EXPECT().RmFile(someHelperPath).Once()
	shellWriter.EXPECT().SetupGitCredHelper(someHelperPath, expectedCredSection, expectedUsername).Once()

	return someHelperPath
}

func TestSanitizeCacheKey(t *testing.T) {
	tests := []struct {
		rawKey          string
		jobNameOverride *string
		gitRefOverride  *string
		expectedKey     string
		errMsg          string
	}{
		{rawKey: "fallback_key", expectedKey: "fallback_key"},
		{rawKey: "fallback_key/", expectedKey: "fallback_key", errMsg: "sanitized to"},
		{rawKey: "fallback_key ", expectedKey: "fallback_key", errMsg: "sanitized to"},
		{rawKey: "fallback_key\\", expectedKey: "fallback_key", errMsg: "sanitized to"},
		{rawKey: "fallback_key/ \\", expectedKey: "fallback_key", errMsg: "sanitized to"},
		{rawKey: "fallback_key/ / \\  \\", expectedKey: "fallback_key", errMsg: "sanitized to"},
		{rawKey: "fallback_key/o", expectedKey: "fallback_key/o"},
		{rawKey: "fallback_key / \\o", expectedKey: "fallback_key / /o", errMsg: "sanitized to"},

		{rawKey: ""},
		{rawKey: "\\", errMsg: "could not be sanitized"},
		{rawKey: "\\.", errMsg: "could not be sanitized"},
		{rawKey: "/", errMsg: "could not be sanitized"},
		{rawKey: " ", errMsg: "could not be sanitized"},
		{rawKey: ".", errMsg: "could not be sanitized"},
		{rawKey: "..", errMsg: "could not be sanitized"},
		{rawKey: "...", expectedKey: "..."},

		// %2F == '/', %2E == '.', %5C == `\`
		{rawKey: "something %2F something", expectedKey: "something / something", errMsg: "sanitized to"},
		{rawKey: "something %2f something", expectedKey: "something / something", errMsg: "sanitized to"},
		{rawKey: "%2E", errMsg: "could not be sanitized"},
		{rawKey: "%2E%2E", errMsg: "could not be sanitized"},
		{rawKey: "%2E%2E%2E", expectedKey: "...", errMsg: "sanitized to"},
		{rawKey: "%2e", errMsg: "could not be sanitized"},
		{rawKey: "%2e%2E", errMsg: "could not be sanitized"},
		{rawKey: ".%2E", errMsg: "could not be sanitized"},
		{rawKey: "%2e.", errMsg: "could not be sanitized"},
		{rawKey: "%2E%2e%2E", expectedKey: "...", errMsg: "sanitized to"},
		// This is allowed, we don't know of any problem with this neither on local storage nor on remote/cloud.
		{rawKey: "%5C", expectedKey: "%5C"},
		{rawKey: "%5c", expectedKey: "%5c"},

		{rawKey: "\t foo bar \t\r", expectedKey: "\t foo bar", errMsg: "sanitized to"},

		{rawKey: "some-job/some-ref", expectedKey: "some-job/some-ref"},
		{rawKey: "some%2f../job/some/ref/.", expectedKey: "job/some/ref", errMsg: "sanitized to"},
		{rawKey: ".../....", expectedKey: ".../...."},
		{rawKey: "../.", errMsg: "could not be sanitized"},
		{rawKey: `job\name/git\ref`, expectedKey: "job/name/git/ref", errMsg: "sanitized to"},
		{rawKey: ` / `, errMsg: "could not be sanitized"},
		{rawKey: ` foo / bar `, expectedKey: " foo / bar", errMsg: "sanitized to"},
		{rawKey: `//`, errMsg: "could not be sanitized"},
		{rawKey: `//\`, errMsg: "could not be sanitized"},

		{rawKey: "foo/./bar", expectedKey: "foo/bar", errMsg: "sanitized to"},
		{rawKey: "foo/blipp/../bar", expectedKey: "foo/bar", errMsg: "sanitized to"},
		{rawKey: "/foo/bar", expectedKey: "foo/bar", errMsg: "sanitized to"},
		{rawKey: "//foo/bar", expectedKey: "foo/bar", errMsg: "sanitized to"},
		{rawKey: "./foo/bar", expectedKey: "foo/bar", errMsg: "sanitized to"},
		{rawKey: "../foo/bar", expectedKey: "foo/bar", errMsg: "sanitized to"},
		{rawKey: ".../foo/bar", expectedKey: ".../foo/bar"},
		{rawKey: "foo/bar/..", expectedKey: "foo", errMsg: "sanitized to"},
		{rawKey: "foo/bar/../..", errMsg: "could not be sanitized"},
		{rawKey: "foo/bar/../../../.././blerp", expectedKey: "blerp", errMsg: "sanitized to"},

		{rawKey: "foo\\.\\bar", expectedKey: "foo/bar", errMsg: "sanitized to"},
		{rawKey: "foo\\blipp\\..\\bar", expectedKey: "foo/bar", errMsg: "sanitized to"},
		{rawKey: "\\foo\\bar", expectedKey: "foo/bar", errMsg: "sanitized to"},
		{rawKey: "\\\\foo\\bar", expectedKey: "foo/bar", errMsg: "sanitized to"},
		{rawKey: ".\\foo\\bar", expectedKey: "foo/bar", errMsg: "sanitized to"},
		{rawKey: "..\\foo\\bar", expectedKey: "foo/bar", errMsg: "sanitized to"},
		{rawKey: "...\\foo\\bar", expectedKey: ".../foo/bar", errMsg: "sanitized to"},
		{rawKey: "foo\\bar\\..", expectedKey: "foo", errMsg: "sanitized to"},
		{rawKey: "foo\\bar\\..\\..", errMsg: "could not be sanitized"},
		{rawKey: "foo\\bar\\..\\..\\..\\..\\.\\blerp", expectedKey: "blerp", errMsg: "sanitized to"},
	}

	for i, test := range tests {
		name := fmt.Sprintf("%d: %s", i, test.rawKey)
		t.Run(name, func(t *testing.T) {
			actual, err := sanitizeCacheKey(test.rawKey)
			if test.errMsg == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.errMsg)
			}
			assert.Equal(t, test.expectedKey, actual)
		})
	}
}

func TestAbstractShell_writeGitCleanup(t *testing.T) {
	v := common.AppVersion
	userAgent := fmt.Sprintf("http.userAgent=%s %s %s/%s", v.Name, v.Version, v.OS, v.Architecture)
	submoduleStrategies := map[common.SubmoduleStrategy]bool{
		common.SubmoduleNone:      false,
		common.SubmoduleNormal:    true,
		common.SubmoduleRecursive: true,
		common.SubmoduleInvalid:   false,
	}
	cleanGitConfigs := map[string]struct {
		configValue                 *bool
		expectGitConfigsToBeCleaned bool
	}{
		"<nil>": {
			expectGitConfigsToBeCleaned: true,
		},
		"enabled": {
			configValue:                 &[]bool{true}[0],
			expectGitConfigsToBeCleaned: true,
		},
		"disabled": {
			configValue:                 &[]bool{false}[0],
			expectGitConfigsToBeCleaned: false,
		},
	}

	for name, cleanGitConfig := range cleanGitConfigs {
		t.Run("cleanGitConfig:"+name, func(t *testing.T) {
			for submoduleStrategy, expectSubmoduleCleanupCalls := range submoduleStrategies {
				t.Run("submoduleStrategy:"+string(submoduleStrategy), func(t *testing.T) {
					for _, gitURLsWithoutTokens := range []bool{true, false} {
						t.Run(fmt.Sprintf("%s:%t", featureflags.GitURLsWithoutTokens, gitURLsWithoutTokens), func(t *testing.T) {
							shell := new(AbstractShell)

							info := common.ShellScriptInfo{
								Build: &common.Build{
									JobResponse: common.JobResponse{
										Variables: common.JobVariables{
											{Key: "GIT_STRATEGY", Value: "fetch"},
											{Key: "GIT_SUBMODULE_STRATEGY", Value: string(submoduleStrategy)},
											{Key: featureflags.GitURLsWithoutTokens, Value: fmt.Sprint(gitURLsWithoutTokens)},
										},
										GitInfo: common.GitInfo{
											RepoURL: "https://repo-url/some/repo",
										},
										JobRequestCorrelationID: "foobar",
									},
									Runner: &common.RunnerConfig{
										RunnerSettings: common.RunnerSettings{
											CleanGitConfig: cleanGitConfig.configValue,
										},
									},
								},
							}

							// ensure the cleanup is called at the beginning, from handleGetSourcesStrategy
							t.Run("handleGetSourcesStrategy", func(t *testing.T) {
								sw := NewMockShellWriter(t)

								// ---- from handleGetSourcesStrategy
								expectFileCleanup(sw, ".git", expectSubmoduleCleanupCalls)

								templateDir, templateSetupCommands := expectSetupTemplate(sw, "")

								if cleanGitConfig.expectGitConfigsToBeCleaned {
									cleanupCommands := expectGitConfigCleanup(sw, "", expectSubmoduleCleanupCalls)
									// Ensure, cleanup happens before template dir setup
									mock.InOrder(slices.Concat(cleanupCommands, templateSetupCommands)...)
								}

								if info.Build.IsFeatureFlagOn(featureflags.GitURLsWithoutTokens) {
									credConfigFile := expectGitCredHelperSetup(sw, "https://repo-url")
									sw.EXPECT().Command("git", "config", "include.path", credConfigFile).Once()
								}

								// ---- from writeRefspecFetchCmd
								sw.EXPECT().Noticef("Fetching changes...").Once()

								sw.EXPECT().Command("git", "init", "", "--template", templateDir).Once()
								sw.EXPECT().Cd("").Once()

								sw.EXPECT().IfCmd("git", "remote", "add", "origin", "https://repo-url/some/repo")
								sw.EXPECT().Noticef("Created fresh repository.").Once()
								sw.EXPECT().Else().Once()
								sw.EXPECT().Command("git", "remote", "set-url", "origin", "https://repo-url/some/repo").Once()
								sw.EXPECT().EndIf().Once()

								sw.EXPECT().IfFile(".git/shallow").Once()
								sw.EXPECT().Command("git", "-c", userAgent, "fetch", "origin", "--no-recurse-submodules", "--prune", "--quiet", "--unshallow").Once()
								sw.EXPECT().Else().Once()
								sw.EXPECT().Command("git", "-c", userAgent, "fetch", "origin", "--no-recurse-submodules", "--prune", "--quiet").Once()
								sw.EXPECT().EndIf().Once()

								err := shell.handleGetSourcesStrategy(sw, info)
								assert.NoError(t, err)
							})

							// ensure the cleanup is also called at the end, from writeCleanupScript
							t.Run("writeCleanupScript", func(t *testing.T) {
								sw := NewMockShellWriter(t)

								sw.EXPECT().TmpFile("masking.db").Return("masking.db").Once()
								sw.EXPECT().RmFile("masking.db").Once()

								sw.EXPECT().TmpFile("gitlab_runner_env").Return("someRunnerEnv").Once()
								sw.EXPECT().RmFile("someRunnerEnv").Once()

								expectFileCleanup(sw, ".git", expectSubmoduleCleanupCalls)
								if cleanGitConfig.expectGitConfigsToBeCleaned {
									expectGitConfigCleanup(sw, "", expectSubmoduleCleanupCalls)
								}

								sw.EXPECT().RmFile("/.gitlab-build-uid-gid").Once()
								sw.EXPECT().Join("", ".gitlab-build-uid-gid").Return("/.gitlab-build-uid-gid").Once()

								err := shell.writeCleanupScript(context.TODO(), sw, info)
								assert.NoError(t, err)
							})
						})
					}
				})
			}
		})
	}
}

func TestNewCacheConfig(t *testing.T) {
	const (
		fallback = "__fallback__"
		windows  = "windows"
	)

	tests := map[string]struct {
		userKey  string
		buildDir string
		cacheDir string
		ffs      map[string]bool
		jobName  string
		gitRef   string
		vars     common.JobVariables

		expectedErrorMsg    string
		expectedCacheConfig map[string]cacheConfig
		expectedWarning     string
	}{
		"no cache dir": {
			expectedErrorMsg: "unset cache directory",
		},
		"empty cache key": {
			cacheDir:         "/some/cache/dir",
			expectedErrorMsg: "empty cache key",
		},
		"not able to resolve cache dir": {
			cacheDir:         "/caches",
			jobName:          "some-job-name",
			expectedErrorMsg: "inability to make the cache file path relative to the build directory",
		},
		"hashed, key from job": {
			cacheDir: "/some/cache/dir",
			buildDir: "/some/build/dir",
			jobName:  "some-job",
			gitRef:   "some-ref",
			ffs: map[string]bool{
				featureflags.HashCacheKeys: true,
			},
			expectedCacheConfig: map[string]cacheConfig{
				fallback: {
					HumanKey:    "some-job/some-ref",
					HashedKey:   "d03a852ba491ba611e907b1ef60ad5c4516a05b8f3aae6abb77f42bc60325aed",
					ArchiveFile: "../../cache/dir/d03a852ba491ba611e907b1ef60ad5c4516a05b8f3aae6abb77f42bc60325aed/cache.zip",
				},
				windows: {
					HumanKey:    "some-job/some-ref",
					HashedKey:   "d03a852ba491ba611e907b1ef60ad5c4516a05b8f3aae6abb77f42bc60325aed",
					ArchiveFile: "..\\..\\cache\\dir\\d03a852ba491ba611e907b1ef60ad5c4516a05b8f3aae6abb77f42bc60325aed\\cache.zip",
				},
			},
		},
		"hashed, key from user": {
			cacheDir: "/some/cache/dir",
			buildDir: "/some/build/dir",
			userKey:  "some/user/key",
			ffs: map[string]bool{
				featureflags.HashCacheKeys: true,
			},
			expectedCacheConfig: map[string]cacheConfig{
				fallback: {
					HumanKey:    "some/user/key",
					HashedKey:   "7f6da050858a8c8767cddbfdf331cbe3a0269abba1fc11fd3fa381b8851b7917",
					ArchiveFile: "../../cache/dir/7f6da050858a8c8767cddbfdf331cbe3a0269abba1fc11fd3fa381b8851b7917/cache.zip",
				},
				windows: {
					HumanKey:    "some/user/key",
					HashedKey:   "7f6da050858a8c8767cddbfdf331cbe3a0269abba1fc11fd3fa381b8851b7917",
					ArchiveFile: "..\\..\\cache\\dir\\7f6da050858a8c8767cddbfdf331cbe3a0269abba1fc11fd3fa381b8851b7917\\cache.zip",
				},
			},
		},
		"hasehd, with powershell path resolver enabled": {
			cacheDir: "/some/cache/dir",
			buildDir: "/some/build/dir",
			userKey:  "some/user/key",
			ffs: map[string]bool{
				featureflags.UsePowershellPathResolver: true,
				featureflags.HashCacheKeys:             true,
			},
			expectedCacheConfig: map[string]cacheConfig{
				fallback: {
					HumanKey:    "some/user/key",
					HashedKey:   "7f6da050858a8c8767cddbfdf331cbe3a0269abba1fc11fd3fa381b8851b7917",
					ArchiveFile: "/some/cache/dir/7f6da050858a8c8767cddbfdf331cbe3a0269abba1fc11fd3fa381b8851b7917/cache.zip",
				},
			},
		},
		"unhashed": {
			cacheDir: "/some/cache/dir",
			buildDir: "/some/build/dir",
			userKey:  "some/user/key",
			ffs: map[string]bool{
				featureflags.HashCacheKeys: false,
			},
			expectedCacheConfig: map[string]cacheConfig{
				fallback: {
					HumanKey:    "some/user/key",
					HashedKey:   "some/user/key",
					ArchiveFile: "../../cache/dir/some/user/key/cache.zip",
				},
				windows: {
					HumanKey:    "some/user/key",
					HashedKey:   "some/user/key",
					ArchiveFile: "..\\..\\cache\\dir\\some\\user\\key\\cache.zip",
				},
			},
		},
		"unhashed, and key sanitized": {
			cacheDir: "/some/cache/dir",
			buildDir: "/some/build/dir",
			userKey:  "some%2fuser%2Fkey",
			ffs: map[string]bool{
				featureflags.HashCacheKeys: false,
			},
			expectedWarning: `cache key "some%2fuser%2Fkey" sanitized to "some/user/key"`,
			expectedCacheConfig: map[string]cacheConfig{
				fallback: {
					HumanKey:    "some/user/key",
					HashedKey:   "some/user/key",
					ArchiveFile: "../../cache/dir/some/user/key/cache.zip",
				},
				windows: {
					HumanKey:    "some/user/key",
					HashedKey:   "some/user/key",
					ArchiveFile: "..\\..\\cache\\dir\\some\\user\\key\\cache.zip",
				},
			},
		},
		"unhashed, and sanitizing failure": {
			cacheDir: "/some/cache/dir",
			buildDir: "/some/build/dir",
			userKey:  "/",
			ffs: map[string]bool{
				featureflags.HashCacheKeys: false,
			},
			expectedWarning:  `cache key "/" could not be sanitized`,
			expectedErrorMsg: "empty cache key",
		},
		"hashed, user key is expanded": {
			cacheDir: "/some/cache/dir",
			buildDir: "/some/build/dir",
			userKey:  "${foo}/${bar}/baz",
			vars: common.JobVariables{
				{Key: "foo", Value: "someFoo"},
				{Key: "bar", Value: "someBar"},
			},
			ffs: map[string]bool{
				featureflags.HashCacheKeys: true,
			},
			expectedCacheConfig: map[string]cacheConfig{
				fallback: {
					HumanKey:    "someFoo/someBar/baz",
					HashedKey:   "78c3e86b9d11a834cb5fe576456a2790c90c6068ef9907415873f1a9bd1b87bb",
					ArchiveFile: "../../cache/dir/78c3e86b9d11a834cb5fe576456a2790c90c6068ef9907415873f1a9bd1b87bb/cache.zip",
				},
				windows: {
					HumanKey:    "someFoo/someBar/baz",
					HashedKey:   "78c3e86b9d11a834cb5fe576456a2790c90c6068ef9907415873f1a9bd1b87bb",
					ArchiveFile: "..\\..\\cache\\dir\\78c3e86b9d11a834cb5fe576456a2790c90c6068ef9907415873f1a9bd1b87bb\\cache.zip",
				},
			},
		},
		"unhashed, user key is expanded": {
			cacheDir: "/some/cache/dir",
			buildDir: "/some/build/dir",
			userKey:  "${foo}/${bar}/baz",
			vars: common.JobVariables{
				{Key: "foo", Value: "someFoo"},
				{Key: "bar", Value: "someBar"},
			},
			ffs: map[string]bool{
				featureflags.HashCacheKeys: false,
			},
			expectedCacheConfig: map[string]cacheConfig{
				fallback: {
					HumanKey:    "someFoo/someBar/baz",
					HashedKey:   "someFoo/someBar/baz",
					ArchiveFile: "../../cache/dir/someFoo/someBar/baz/cache.zip",
				},
				windows: {
					HumanKey:    "someFoo/someBar/baz",
					HashedKey:   "someFoo/someBar/baz",
					ArchiveFile: "..\\..\\cache\\dir\\someFoo\\someBar\\baz\\cache.zip",
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			build := &common.Build{
				JobResponse: common.JobResponse{
					Variables: test.vars,
					JobInfo: common.JobInfo{
						Name: test.jobName,
					},
					GitInfo: common.GitInfo{
						Ref: test.gitRef,
					},
				},
				Runner: &common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						FeatureFlags: test.ffs,
					},
				},
				CacheDir: test.cacheDir,
				BuildDir: test.buildDir,
			}

			actualCacheConfig, warning, err := newCacheConfig(build, test.userKey)

			assert.Equal(t, test.expectedWarning, warning)

			if e := test.expectedErrorMsg; e != "" {
				require.ErrorContains(t, err, e)
				return
			}

			require.NoError(t, err)

			expectedCacheConfig := cmp.Or(
				test.expectedCacheConfig[runtime.GOOS],
				test.expectedCacheConfig[fallback],
			)

			assert.Equal(t, expectedCacheConfig, *actualCacheConfig)
		})
	}
}

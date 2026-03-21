//go:build !integration

package stages

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
)

func TestGetSources_Strategy(t *testing.T) {
	tests := map[string]struct {
		strategy    string
		seedFile    bool
		expectDir   bool
		expectFile  bool
		expectError string
	}{
		"none creates dir": {
			strategy:  "none",
			expectDir: true,
		},
		"empty removes existing content": {
			strategy:   "empty",
			seedFile:   true,
			expectDir:  true,
			expectFile: false,
		},
		"empty creates clean dir from scratch": {
			strategy:  "empty",
			expectDir: true,
		},
		"unknown strategy errors": {
			strategy:    "svn",
			expectError: "unknown GIT_STRATEGY",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			e := newTestEnv(t, "bash")
			projectDir := filepath.Join(e.WorkingDir, "project")
			e.WorkingDir = projectDir

			if tc.seedFile {
				require.NoError(t, os.MkdirAll(projectDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(projectDir, "file"), []byte("data"), 0o644))
			}

			gs := GetSources{
				GitStrategy: tc.strategy,
				MaxAttempts: 1,
			}

			err := gs.Run(t.Context(), e)

			if tc.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
				return
			}

			require.NoError(t, err)

			if tc.expectDir {
				assert.DirExists(t, projectDir)
			}
			if tc.seedFile && !tc.expectFile {
				assert.NoFileExists(t, filepath.Join(projectDir, "file"))
			}

			if tc.strategy == "empty" && !tc.seedFile {
				entries, err := os.ReadDir(projectDir)
				require.NoError(t, err)
				assert.Empty(t, entries)
			}
		})
	}
}

func TestGetSources_HasSubmodules(t *testing.T) {
	tests := map[string]struct {
		strategy string
		expected bool
	}{
		"normal":    {strategy: "normal", expected: true},
		"recursive": {strategy: "recursive", expected: true},
		"none":      {strategy: "none", expected: false},
		"empty":     {strategy: "", expected: false},
		"unknown":   {strategy: "something", expected: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gs := GetSources{SubmoduleStrategy: tc.strategy}
			assert.Equal(t, tc.expected, gs.hasSubmodules())
		})
	}
}

func TestGetSources_CleanupGitState(t *testing.T) {
	tests := map[string]struct {
		submoduleStrategy string
		cleanGitConfig    bool
		setup             func(t *testing.T, e *env.Env)
		assertGone        []string
		assertKept        []string
	}{
		"removes top-level lock files and post-checkout hook": {
			setup: func(t *testing.T, e *env.Env) {
				gitDir := filepath.Join(e.WorkingDir, ".git")
				require.NoError(t, os.MkdirAll(filepath.Join(gitDir, "hooks"), 0o755))
				for _, f := range []string{"index.lock", "shallow.lock", "HEAD.lock", "config.lock"} {
					require.NoError(t, os.WriteFile(filepath.Join(gitDir, f), nil, 0o644))
				}
				require.NoError(t, os.WriteFile(filepath.Join(gitDir, "hooks", "post-checkout"), nil, 0o644))
			},
			assertGone: []string{
				".git/index.lock", ".git/shallow.lock",
				".git/HEAD.lock", ".git/config.lock",
				".git/hooks/post-checkout",
			},
		},
		"removes nested ref locks but keeps non-lock files": {
			setup: func(t *testing.T, e *env.Env) {
				refsDir := filepath.Join(e.WorkingDir, ".git", "refs", "heads")
				require.NoError(t, os.MkdirAll(refsDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(refsDir, "main.lock"), nil, 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(refsDir, "feature.lock"), nil, 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(refsDir, "main"), []byte("ref"), 0o644))
			},
			assertGone: []string{
				".git/refs/heads/main.lock",
				".git/refs/heads/feature.lock",
			},
			assertKept: []string{
				".git/refs/heads/main",
			},
		},
		"removes lock files and post-checkout in submodule dirs": {
			submoduleStrategy: "recursive",
			setup: func(t *testing.T, e *env.Env) {
				modDir := filepath.Join(e.WorkingDir, ".git", "modules", "sub1")
				require.NoError(t, os.MkdirAll(modDir, 0o755))
				for _, f := range []string{"index.lock", "HEAD.lock", "config.lock", "shallow.lock"} {
					require.NoError(t, os.WriteFile(filepath.Join(modDir, f), nil, 0o644))
				}
				require.NoError(t, os.WriteFile(filepath.Join(modDir, "post-checkout"), nil, 0o644))
			},
			assertGone: []string{
				".git/modules/sub1/index.lock",
				".git/modules/sub1/HEAD.lock",
				".git/modules/sub1/config.lock",
				".git/modules/sub1/shallow.lock",
				".git/modules/sub1/post-checkout",
			},
		},
		"skips submodule lock cleanup when no submodule strategy": {
			submoduleStrategy: "none",
			setup: func(t *testing.T, e *env.Env) {
				modDir := filepath.Join(e.WorkingDir, ".git", "modules", "sub1")
				require.NoError(t, os.MkdirAll(modDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(modDir, "index.lock"), nil, 0o644))
			},
			assertKept: []string{
				".git/modules/sub1/index.lock",
			},
		},
		"no .git dir is a no-op": {
			setup: func(t *testing.T, e *env.Env) {},
		},
		"missing hooks dir is a no-op": {
			setup: func(t *testing.T, e *env.Env) {
				require.NoError(t, os.MkdirAll(filepath.Join(e.WorkingDir, ".git"), 0o755))
			},
		},
		"CleanGitConfig removes config and hooks from .git and template dir": {
			cleanGitConfig: true,
			setup: func(t *testing.T, e *env.Env) {
				gitDir := filepath.Join(e.WorkingDir, ".git")
				require.NoError(t, os.MkdirAll(filepath.Join(gitDir, "hooks"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"), []byte("[core]"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(gitDir, "hooks", "pre-commit"), nil, 0o755))

				tmplDir := filepath.Join(e.WorkingDir+".tmp", templateDirName)
				require.NoError(t, os.MkdirAll(filepath.Join(tmplDir, "hooks"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "config"), []byte("[init]"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "hooks", "post-receive"), nil, 0o755))
			},
			assertGone: []string{
				".git/config",
				".git/hooks",
			},
		},
		"CleanGitConfig removes submodule configs and hooks": {
			cleanGitConfig:    true,
			submoduleStrategy: "normal",
			setup: func(t *testing.T, e *env.Env) {
				gitDir := filepath.Join(e.WorkingDir, ".git")
				require.NoError(t, os.MkdirAll(gitDir, 0o755))

				for _, sub := range []string{"sub1", "sub2"} {
					modDir := filepath.Join(gitDir, "modules", sub)
					require.NoError(t, os.MkdirAll(filepath.Join(modDir, "hooks"), 0o755))
					require.NoError(t, os.WriteFile(filepath.Join(modDir, "config"), []byte("[core]"), 0o644))
					require.NoError(t, os.WriteFile(filepath.Join(modDir, "hooks", "pre-commit"), nil, 0o755))
				}
			},
			assertGone: []string{
				".git/modules/sub1/config",
				".git/modules/sub1/hooks",
				".git/modules/sub2/config",
				".git/modules/sub2/hooks",
			},
		},
		"CleanGitConfig=false leaves configs and hooks intact": {
			cleanGitConfig: false,
			setup: func(t *testing.T, e *env.Env) {
				gitDir := filepath.Join(e.WorkingDir, ".git")
				require.NoError(t, os.MkdirAll(filepath.Join(gitDir, "hooks"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"), []byte("[core]"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(gitDir, "hooks", "pre-commit"), nil, 0o755))
			},
			assertKept: []string{
				".git/config",
				".git/hooks/pre-commit",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			e := newTestEnv(t, "bash")
			tc.setup(t, e)

			gs := GetSources{
				SubmoduleStrategy: tc.submoduleStrategy,
				CleanGitConfig:    tc.cleanGitConfig,
			}
			gs.cleanupGitState(e)

			for _, rel := range tc.assertGone {
				p := filepath.Join(e.WorkingDir, rel)
				assert.NoFileExists(t, p, "expected %s to be removed", rel)
				assert.NoDirExists(t, p, "expected %s to be removed", rel)
			}
			for _, rel := range tc.assertKept {
				p := filepath.Join(e.WorkingDir, rel)
				_, err := os.Stat(p)
				assert.NoError(t, err, "expected %s to still exist", rel)
			}
		})
	}
}

func TestGetSources_ClearWorktree(t *testing.T) {
	tests := map[string]struct {
		setup     func(t *testing.T) string
		expectErr bool
	}{
		"non-existent dir is a no-op": {
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nonexistent")
			},
		},
		"path is a file not a dir is a no-op": {
			setup: func(t *testing.T) string {
				f := filepath.Join(t.TempDir(), "afile")
				require.NoError(t, os.WriteFile(f, []byte("data"), 0o644))
				return f
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			e := newTestEnv(t, "bash")
			e.WorkingDir = tc.setup(t)

			gs := GetSources{}
			err := gs.clearWorktree(t.Context(), e)

			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSources_SetupExternalGitConfig(t *testing.T) {
	tests := map[string]struct {
		gs               GetSources
		expectInsteadOf  bool
		expectBundleURI  bool
		expectGitaly     bool
		expectCredHelper bool
	}{
		"repo with creds": {
			gs:              GetSources{RepoURL: "https://gitlab-ci-token:mytoken@example.com/project/repo.git"},
			expectInsteadOf: true,
		},
		"repo without creds": {
			gs:              GetSources{RepoURL: "https://example.com/project/repo.git"},
			expectInsteadOf: false,
		},
		"with bundle URIs and creds": {
			gs: GetSources{
				RepoURL:       "https://gitlab-ci-token:mytoken@example.com/project/repo.git",
				UseBundleURIs: true,
			},
			expectInsteadOf: true,
			expectBundleURI: true,
		},
		"with bundle URIs but no creds": {
			gs: GetSources{
				RepoURL:       "https://example.com/project/repo.git",
				UseBundleURIs: true,
			},
			expectBundleURI: true,
		},
		"with port": {
			gs:              GetSources{RepoURL: "https://gitlab-ci-token:mytoken@example.com:3443/project/repo.git"},
			expectInsteadOf: true,
		},
		"with gitaly correlation ID": {
			gs: GetSources{
				RepoURL:             "https://example.com/project/repo.git",
				GitalyCorrelationID: "abc-123-def",
			},
			expectGitaly: true,
		},
		"with credential helper": {
			gs: GetSources{
				RepoURL:             "https://example.com/project/repo.git",
				RemoteHost:          "https://example.com",
				UseCredentialHelper: true,
			},
			expectCredHelper: true,
		},
		"credential helper without remote host is skipped": {
			gs: GetSources{
				RepoURL:             "https://example.com/project/repo.git",
				UseCredentialHelper: true,
			},
			expectCredHelper: false,
		},
		"with additional insteadOfs": {
			gs: GetSources{
				RepoURL: "https://example.com/project/repo.git",
				InsteadOfs: [][2]string{
					{"https://token@example.com/sub.git", "git@example.com:sub.git"},
				},
			},
			expectInsteadOf: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			e := newTestEnv(t, "bash")

			configFile, cleanup, err := tc.gs.setupExternalGitConfig(t.Context(), e, nil)
			require.NoError(t, err)
			defer cleanup()

			assert.FileExists(t, configFile)

			content, err := os.ReadFile(configFile)
			require.NoError(t, err)
			text := string(content)

			if tc.expectInsteadOf {
				assert.Contains(t, text, "insteadOf")

				if tc.gs.RepoURL != "" {
					parsed, _ := url.Parse(tc.gs.RepoURL)
					if parsed.User != nil {
						assert.Contains(t, text, parsed.String())
						parsed.User = nil
						assert.Contains(t, text, parsed.String())
					}
				}

				for _, io := range tc.gs.InsteadOfs {
					assert.Contains(t, text, io[0])
					assert.Contains(t, text, io[1])
				}
			} else if len(tc.gs.InsteadOfs) == 0 {
				assert.NotContains(t, text, "insteadOf")
			}

			if tc.expectBundleURI {
				assert.Contains(t, text, "bundleURI = true")
			} else {
				assert.NotContains(t, text, "bundleURI")
			}

			if tc.expectGitaly {
				assert.Contains(t, text, "X-Gitaly-Correlation-ID: "+tc.gs.GitalyCorrelationID)
			}

			if tc.expectCredHelper {
				assert.Contains(t, text, fmt.Sprintf("[credential %q]", tc.gs.RemoteHost))
				assert.Contains(t, text, "helper")
				assert.Contains(t, text, "username = gitlab-ci-token")
			} else if !tc.gs.UseCredentialHelper {
				assert.NotContains(t, text, "[credential")
			}
		})
	}
}

func TestGetSources_SetupExternalGitConfig_Cleanup(t *testing.T) {
	e := newTestEnv(t, "bash")

	gs := GetSources{
		RepoURL: "https://gitlab-ci-token:token@example.com/repo.git",
	}

	configFile, cleanup, err := gs.setupExternalGitConfig(t.Context(), e, nil)
	require.NoError(t, err)
	assert.FileExists(t, configFile)

	cleanup()
	assert.NoFileExists(t, configFile)
}

func TestGetSources_SetupExternalGitConfig_BadURL(t *testing.T) {
	e := newTestEnv(t, "bash")

	gs := GetSources{
		RepoURL: "https://[invalid/",
	}

	_, _, err := gs.setupExternalGitConfig(t.Context(), e, nil)
	assert.Error(t, err)
}

func TestGetSources_SetupExternalGitConfig_DeduplicatesInsteadOfs(t *testing.T) {
	e := newTestEnv(t, "bash")

	gs := GetSources{
		RepoURL: "https://example.com/project/repo.git",
		InsteadOfs: [][2]string{
			{"https://token@example.com/sub.git", "git@example.com:sub.git"},
			{"https://token@example.com/sub.git", "git@example.com:sub.git"}, // duplicate
			{"https://token@example.com/other.git", "git@example.com:other.git"},
		},
	}

	configFile, cleanup, err := gs.setupExternalGitConfig(t.Context(), e, nil)
	require.NoError(t, err)
	defer cleanup()

	content, err := os.ReadFile(configFile)
	require.NoError(t, err)
	text := string(content)

	// The first insteadOf should appear exactly once.
	assert.Equal(t, 1, countOccurrences(text, "git@example.com:sub.git"))
	// The second distinct one should also appear.
	assert.Contains(t, text, "git@example.com:other.git")
}

func TestGetSources_SetupTemplateDir(t *testing.T) {
	e := newTestEnv(t, "bash")

	gs := GetSources{}

	tmpDir := e.WorkingDir + ".tmp"
	require.NoError(t, os.MkdirAll(tmpDir, 0o755))

	extConfigFile := filepath.Join(tmpDir, "ext.conf")
	require.NoError(t, os.WriteFile(extConfigFile, []byte("[test]\n"), 0o600))

	templateDir, cleanup, err := gs.setupTemplateDir(e, extConfigFile)
	require.NoError(t, err)
	defer cleanup()

	assert.DirExists(t, templateDir)

	configFile := filepath.Join(templateDir, "config")
	assert.FileExists(t, configFile)

	content, err := os.ReadFile(configFile)
	require.NoError(t, err)

	for _, expected := range []string{
		"defaultBranch = none",
		"recurseSubmodules = false",
		"interactive = never",
		"autoDetach = false",
	} {
		assert.Contains(t, string(content), expected)
	}

	absExt, _ := filepath.Abs(extConfigFile)
	assert.Contains(t, string(content), "path = "+filepath.ToSlash(absExt))
}

func TestGetSources_SetupTemplateDir_Cleanup(t *testing.T) {
	e := newTestEnv(t, "bash")

	gs := GetSources{}

	tmpDir := e.WorkingDir + ".tmp"
	require.NoError(t, os.MkdirAll(tmpDir, 0o755))

	extConfigFile := filepath.Join(tmpDir, "ext.conf")
	require.NoError(t, os.WriteFile(extConfigFile, nil, 0o600))

	templateDir, cleanup, err := gs.setupTemplateDir(e, extConfigFile)
	require.NoError(t, err)
	assert.DirExists(t, templateDir)

	cleanup()
	assert.NoDirExists(t, templateDir)
}

func TestGetSources_SetupTemplateDir_RemovesStale(t *testing.T) {
	e := newTestEnv(t, "bash")

	tmpDir := e.WorkingDir + ".tmp"
	require.NoError(t, os.MkdirAll(tmpDir, 0o755))

	// Seed a stale template dir.
	staleDir := filepath.Join(tmpDir, templateDirName)
	require.NoError(t, os.MkdirAll(staleDir, 0o755))
	staleFile := filepath.Join(staleDir, "stale-marker")
	require.NoError(t, os.WriteFile(staleFile, []byte("old"), 0o644))

	extConfigFile := filepath.Join(tmpDir, "ext.conf")
	require.NoError(t, os.WriteFile(extConfigFile, nil, 0o600))

	gs := GetSources{}
	templateDir, cleanup, err := gs.setupTemplateDir(e, extConfigFile)
	require.NoError(t, err)
	defer cleanup()

	// The stale marker should be gone, replaced by a fresh template.
	assert.NoFileExists(t, staleFile)
	assert.FileExists(t, filepath.Join(templateDir, "config"))
}

func TestGetSources_RemoteURLWithoutCreds(t *testing.T) {
	tests := map[string]struct {
		repoURL  string
		expected string
	}{
		"with creds": {
			repoURL:  "https://gitlab-ci-token:mytoken@example.com/project/repo.git",
			expected: "https://example.com/project/repo.git",
		},
		"without creds": {
			repoURL:  "https://example.com/project/repo.git",
			expected: "https://example.com/project/repo.git",
		},
		"with port": {
			repoURL:  "https://gitlab-ci-token:token@example.com:3443/project/repo.git",
			expected: "https://example.com:3443/project/repo.git",
		},
		"bad URL falls back to raw": {
			repoURL:  "not a url",
			expected: "not%20a%20url",
		},
		"ssh style URL": {
			repoURL:  "ssh://git@example.com/project/repo.git",
			expected: "ssh://example.com/project/repo.git",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gs := GetSources{RepoURL: tc.repoURL}
			assert.Equal(t, tc.expected, gs.remoteURLWithoutCreds())
		})
	}
}

func TestGetSources_UserAgentArgs(t *testing.T) {
	tests := map[string]struct {
		userAgent string
		expected  []string
	}{
		"empty": {
			userAgent: "",
			expected:  nil,
		},
		"set": {
			userAgent: "gitlab-runner 17.0.0 linux/amd64",
			expected:  []string{"-c", "http.userAgent=gitlab-runner 17.0.0 linux/amd64"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gs := GetSources{UserAgent: tc.userAgent}
			assert.Equal(t, tc.expected, gs.configArgs())
		})
	}
}

func TestGetSources_SubmodulePathArgs(t *testing.T) {
	tests := map[string]struct {
		paths    []string
		expected []string
	}{
		"empty":    {paths: nil, expected: nil},
		"single":   {paths: []string{"sub1"}, expected: []string{"--", "sub1"}},
		"multiple": {paths: []string{"sub1", "sub2", "sub3"}, expected: []string{"--", "sub1", "sub2", "sub3"}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gs := GetSources{SubmodulePaths: tc.paths}
			assert.Equal(t, tc.expected, gs.submodulePathArgs())
		})
	}
}

func TestGetSources_HasRemoteFlag(t *testing.T) {
	tests := map[string]struct {
		flags    []string
		expected bool
	}{
		"no flags":         {flags: nil, expected: false},
		"has remote":       {flags: []string{"--remote", "--progress"}, expected: true},
		"no remote":        {flags: []string{"--progress"}, expected: false},
		"case insensitive": {flags: []string{"--REMOTE"}, expected: true},
		"remote only":      {flags: []string{"--remote"}, expected: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gs := GetSources{SubmoduleUpdateFlags: tc.flags}
			assert.Equal(t, tc.expected, gs.hasRemoteFlag())
		})
	}
}

func TestIsShallowRepo(t *testing.T) {
	tests := map[string]struct {
		setup    func(t *testing.T) string
		expected bool
	}{
		"shallow": {
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				gitDir := filepath.Join(dir, ".git")
				require.NoError(t, os.MkdirAll(gitDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(gitDir, "shallow"), nil, 0o644))
				return dir
			},
			expected: true,
		},
		"not shallow": {
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
				return dir
			},
			expected: false,
		},
		"no git dir": {
			setup:    func(t *testing.T) string { return t.TempDir() },
			expected: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isShallowRepo(tc.setup(t)))
		})
	}
}

func TestGetSources_PrePostCloneSkippedForNonGitStrategies(t *testing.T) {
	steps := struct {
		pre  Step
		post Step
	}{
		pre: Step{
			Step:      "pre_clone_script",
			Script:    []string{"echo pre"},
			OnSuccess: true,
			OnFailure: true,
		},
		post: Step{
			Step:      "post_clone_script",
			Script:    []string{"echo post"},
			OnSuccess: true,
			OnFailure: true,
		},
	}

	for _, strategy := range []string{"none", "empty"} {
		t.Run(strategy, func(t *testing.T) {
			e := newTestEnv(t, "bash")
			projectDir := filepath.Join(e.WorkingDir, "project")
			e.WorkingDir = projectDir

			gs := GetSources{
				GitStrategy:   strategy,
				MaxAttempts:   1,
				PreCloneStep:  steps.pre,
				PostCloneStep: steps.post,
			}

			err := gs.Run(t.Context(), e)
			require.NoError(t, err)
			assert.DirExists(t, projectDir)

			stdout := e.Stdout.(*bytes.Buffer).String()
			assert.NotContains(t, stdout, "pre")
			assert.NotContains(t, stdout, "post")
		})
	}
}

func TestGetSources_Retry(t *testing.T) {
	tests := map[string]struct {
		maxAttempts          int
		clearWorktreeOnRetry bool
		expectRetryMsg       bool
		expectClearMsg       bool
	}{
		"single attempt does not retry": {
			maxAttempts: 1,
		},
		"multiple attempts retries": {
			maxAttempts:    3,
			expectRetryMsg: true,
		},
		"retry with clear worktree": {
			maxAttempts:          2,
			clearWorktreeOnRetry: true,
			expectRetryMsg:       true,
			expectClearMsg:       true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			e := newTestEnv(t, "bash")

			gs := GetSources{
				GitStrategy:          "fetch",
				RepoURL:              "https://invalid.example.com/nonexistent.git",
				Refspecs:             []string{"+refs/heads/*:refs/remotes/origin/*"},
				MaxAttempts:          tc.maxAttempts,
				ClearWorktreeOnRetry: tc.clearWorktreeOnRetry,
			}

			err := gs.Run(t.Context(), e)
			assert.Error(t, err)

			stderr := e.Stderr.(*bytes.Buffer).String()

			if tc.expectRetryMsg {
				assert.Contains(t, stderr, "Retrying")
			} else {
				assert.NotContains(t, stderr, "Retrying")
			}

			if tc.expectClearMsg {
				assert.Contains(t, stderr, "Deleting tracked and untracked files")
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := map[string]struct {
		a, b     string
		expected int
	}{
		"equal":               {a: "2.49", b: "2.49", expected: 0},
		"equal three parts":   {a: "2.49.0", b: "2.49.0", expected: 0},
		"a greater":           {a: "2.50", b: "2.49", expected: 1},
		"b greater":           {a: "2.48", b: "2.49", expected: -1},
		"major differs":       {a: "3.0", b: "2.99", expected: 1},
		"a shorter but equal": {a: "2.49", b: "2.49.0", expected: 0},
		"a shorter and less":  {a: "2.49", b: "2.49.1", expected: -1},
		"b shorter and less":  {a: "2.49.1", b: "2.49", expected: 1},
		"single component":    {a: "3", b: "2", expected: 1},
		"four components":     {a: "2.49.0.1", b: "2.49.0.0", expected: 1},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, compareVersions(tc.a, tc.b))
		})
	}
}

func TestDeduplicateInsteadOfs(t *testing.T) {
	tests := map[string]struct {
		input    [][2]string
		expected [][2]string
	}{
		"empty": {
			input:    nil,
			expected: [][2]string{},
		},
		"no duplicates": {
			input:    [][2]string{{"a", "b"}, {"c", "d"}},
			expected: [][2]string{{"a", "b"}, {"c", "d"}},
		},
		"removes duplicates preserving order": {
			input:    [][2]string{{"a", "b"}, {"c", "d"}, {"a", "b"}, {"e", "f"}, {"c", "d"}},
			expected: [][2]string{{"a", "b"}, {"c", "d"}, {"e", "f"}},
		},
		"same first element different second": {
			input:    [][2]string{{"a", "b"}, {"a", "c"}},
			expected: [][2]string{{"a", "b"}, {"a", "c"}},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, deduplicateInsteadOfs(tc.input))
		})
	}
}

func TestWalkRemove(t *testing.T) {
	tests := map[string]struct {
		setup      func(t *testing.T, root string)
		name       string
		bySuffix   bool
		assertGone []string
		assertKept []string
	}{
		"removes files by exact name": {
			name: "config",
			setup: func(t *testing.T, root string) {
				for _, sub := range []string{"a", "b"} {
					dir := filepath.Join(root, sub)
					require.NoError(t, os.MkdirAll(dir, 0o755))
					require.NoError(t, os.WriteFile(filepath.Join(dir, "config"), nil, 0o644))
					require.NoError(t, os.WriteFile(filepath.Join(dir, "other"), nil, 0o644))
				}
			},
			assertGone: []string{"a/config", "b/config"},
			assertKept: []string{"a/other", "b/other"},
		},
		"removes dirs by exact name": {
			name: "hooks",
			setup: func(t *testing.T, root string) {
				for _, sub := range []string{"a", "b"} {
					dir := filepath.Join(root, sub, "hooks")
					require.NoError(t, os.MkdirAll(dir, 0o755))
					require.NoError(t, os.WriteFile(filepath.Join(dir, "pre-commit"), nil, 0o755))
				}
				// "webhooks" should NOT match exact "hooks"
				other := filepath.Join(root, "a", "webhooks")
				require.NoError(t, os.MkdirAll(other, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(other, "file"), nil, 0o644))
			},
			assertGone: []string{"a/hooks", "b/hooks"},
			assertKept: []string{"a/webhooks/file"},
		},
		"removes files by suffix": {
			name:     ".lock",
			bySuffix: true,
			setup: func(t *testing.T, root string) {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "refs"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(root, "refs", "main.lock"), nil, 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(root, "refs", "main"), []byte("ref"), 0o644))
			},
			assertGone: []string{"refs/main.lock"},
			assertKept: []string{"refs/main"},
		},
		"no-op on missing dir": {
			name:  "config",
			setup: func(t *testing.T, root string) {},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)

			walkRemove(root, tc.name, tc.bySuffix)

			for _, rel := range tc.assertGone {
				p := filepath.Join(root, rel)
				assert.NoFileExists(t, p, "expected %s to be removed", rel)
				assert.NoDirExists(t, p, "expected %s to be removed", rel)
			}
			for _, rel := range tc.assertKept {
				_, err := os.Stat(filepath.Join(root, rel))
				assert.NoError(t, err, "expected %s to still exist", rel)
			}
		})
	}
}

//nolint:unparam
func newTestEnv(t *testing.T, shell string) *env.Env {
	t.Helper()

	baseDir := t.TempDir()
	workingDir := filepath.Join(baseDir, "project")
	require.NoError(t, os.MkdirAll(workingDir, 0o755))
	require.NoError(t, os.MkdirAll(workingDir+".tmp", 0o755))

	return &env.Env{
		WorkingDir: workingDir,
		Shell:      shell,
		Env:        map[string]string{},
		GitLabEnv:  map[string]string{},
		Stdout:     new(bytes.Buffer),
		Stderr:     new(bytes.Buffer),
	}
}

func countOccurrences(s, substr string) int {
	count := 0
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			count++
		}
	}
	return count
}

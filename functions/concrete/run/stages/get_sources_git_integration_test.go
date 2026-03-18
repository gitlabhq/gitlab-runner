//go:build integration

package stages_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/stages"
)

// testRepo creates a bare git repo with one commit containing a single file.
// Returns the file:// URL to use as RepoURL, the SHA of the commit, and the ref.
func testRepo(t *testing.T) (repoURL, sha, ref string) {
	t.Helper()

	bareDir := filepath.Join(t.TempDir(), "bare.git")
	workDir := filepath.Join(t.TempDir(), "work")

	run(t, "", "git", "init", "--bare", "-b", "main", bareDir)
	run(t, "", "git", "clone", bareDir, workDir)
	run(t, workDir, "git", "config", "user.email", "test@test.com")
	run(t, workDir, "git", "config", "user.name", "Test")

	require.NoError(t, os.WriteFile(filepath.Join(workDir, "hello.txt"), []byte("hello world\n"), 0o644))
	run(t, workDir, "git", "add", ".")
	run(t, workDir, "git", "commit", "-m", "initial commit")

	out := runOutput(t, workDir, "git", "rev-parse", "HEAD")
	run(t, workDir, "git", "push", "origin", "HEAD:refs/heads/main")

	return fileURL(bareDir), out, "main"
}

// testRepoWithHistory creates a bare repo with two commits so that
// shallow/unshallow behaviour can be verified.
func testRepoWithHistory(t *testing.T) (repoURL, sha, ref string) {
	t.Helper()

	bareDir := filepath.Join(t.TempDir(), "bare.git")
	workDir := filepath.Join(t.TempDir(), "work")

	run(t, "", "git", "init", "--bare", "-b", "main", bareDir)
	run(t, "", "git", "clone", bareDir, workDir)
	run(t, workDir, "git", "config", "user.email", "test@test.com")
	run(t, workDir, "git", "config", "user.name", "Test")

	require.NoError(t, os.WriteFile(filepath.Join(workDir, "first.txt"), []byte("first\n"), 0o644))
	run(t, workDir, "git", "add", ".")
	run(t, workDir, "git", "commit", "-m", "first commit")

	require.NoError(t, os.WriteFile(filepath.Join(workDir, "second.txt"), []byte("second\n"), 0o644))
	run(t, workDir, "git", "add", ".")
	run(t, workDir, "git", "commit", "-m", "second commit")

	out := runOutput(t, workDir, "git", "rev-parse", "HEAD")
	run(t, workDir, "git", "push", "origin", "HEAD:refs/heads/main")

	return fileURL(bareDir), out, "main"
}

// testRepoWithSubmodule creates a bare repo that has a submodule pointing
// to another bare repo.
func testRepoWithSubmodule(t *testing.T) (repoURL, sha, ref string) {
	t.Helper()

	subBareDir := filepath.Join(t.TempDir(), "sub-bare.git")
	subWorkDir := filepath.Join(t.TempDir(), "sub-work")

	run(t, "", "git", "init", "--bare", "-b", "main", subBareDir)
	run(t, "", "git", "clone", subBareDir, subWorkDir)
	run(t, subWorkDir, "git", "config", "user.email", "test@test.com")
	run(t, subWorkDir, "git", "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(subWorkDir, "sub.txt"), []byte("submodule content\n"), 0o644))
	run(t, subWorkDir, "git", "add", ".")
	run(t, subWorkDir, "git", "commit", "-m", "sub initial")
	run(t, subWorkDir, "git", "push", "origin", "HEAD:refs/heads/main")

	mainBareDir := filepath.Join(t.TempDir(), "main-bare.git")
	mainWorkDir := filepath.Join(t.TempDir(), "main-work")

	run(t, "", "git", "init", "--bare", "-b", "main", mainBareDir)
	run(t, "", "git", "clone", mainBareDir, mainWorkDir)
	run(t, mainWorkDir, "git", "config", "user.email", "test@test.com")
	run(t, mainWorkDir, "git", "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(mainWorkDir, "main.txt"), []byte("main content\n"), 0o644))
	run(t, mainWorkDir, "git", "add", ".")
	run(t, mainWorkDir, "git", "commit", "-m", "initial commit") // add this
	run(t, mainWorkDir, "git", "-c", "protocol.file.allow=always", "submodule", "add", fileURL(subBareDir), "mysub")
	run(t, mainWorkDir, "git", "add", ".")
	run(t, mainWorkDir, "git", "commit", "-m", "main initial with submodule")
	run(t, mainWorkDir, "git", "push", "origin", "HEAD:refs/heads/main")

	sha = runOutput(t, mainWorkDir, "git", "rev-parse", "HEAD")

	return fileURL(mainBareDir), sha, "main"
}

func fileURL(dir string) string {
	p := filepath.ToSlash(dir)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "file://" + p
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "command %s %v failed: %s", name, args, string(out))
}

func runOutput(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	return string(bytes.TrimSpace(out))
}

func gitEnv(t *testing.T, shell string) *env.Env {
	t.Helper()

	baseDir := t.TempDir()
	workingDir := filepath.Join(baseDir, "project")
	require.NoError(t, os.MkdirAll(workingDir, 0o755))
	require.NoError(t, os.MkdirAll(workingDir+".tmp", 0o755))

	return &env.Env{
		WorkingDir: workingDir,
		Shell:      shell,
		Env: map[string]string{
			"HOME":                t.TempDir(),
			"GIT_TERMINAL_PROMPT": "0",
			"GIT_CONFIG_NOSYSTEM": "1",
			"GIT_AUTHOR_NAME":     "Test",
			"GIT_AUTHOR_EMAIL":    "test@test.com",
			"GIT_COMMITTER_NAME":  "Test",
			"GIT_COMMITTER_EMAIL": "test@test.com",
			"GIT_CONFIG_COUNT":    "1",
			"GIT_CONFIG_KEY_0":    "protocol.file.allow",
			"GIT_CONFIG_VALUE_0":  "always",
		},
		GitLabEnv: map[string]string{},
		Stdout:    &bytes.Buffer{},
		Stderr:    &bytes.Buffer{},
	}
}

// commitCount returns the number of commits reachable from HEAD.
func commitCount(t *testing.T, dir string) int {
	t.Helper()
	out := runOutput(t, dir, "git", "rev-list", "--count", "HEAD")
	n := 0
	for _, c := range out {
		n = n*10 + int(c-'0')
	}
	return n
}

func TestGetSourcesGit_Clone(t *testing.T) {
	tests := map[string]struct {
		depth          int
		useNativeClone bool
		expectShallow  bool
		gitCloneFlags  []string
	}{
		"basic": {},
		"with depth": {
			depth:         1,
			expectShallow: true,
		},
		"native clone": {
			useNativeClone: true,
		},
		"native clone with depth": {
			depth:          1,
			useNativeClone: true,
			expectShallow:  true,
		},
		"with extra clone flags": {
			gitCloneFlags: []string{"--no-tags"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			repoURL, sha, ref := testRepo(t)
			e := gitEnv(t, "bash")

			gs := stages.GetSources{
				GitStrategy:    "clone",
				Checkout:       true,
				Depth:          tc.depth,
				RepoURL:        repoURL,
				SHA:            sha,
				Ref:            ref,
				Refspecs:       []string{"+refs/heads/*:refs/remotes/origin/*"},
				MaxAttempts:    1,
				UseNativeClone: tc.useNativeClone,
				GitCloneFlags:  tc.gitCloneFlags,
			}

			err := gs.Run(context.Background(), e)
			require.NoError(t, err, "stderr: %s", e.Stderr.(*bytes.Buffer).String())

			assert.FileExists(t, filepath.Join(e.WorkingDir, "hello.txt"))

			actual := runOutput(t, e.WorkingDir, "git", "rev-parse", "HEAD")
			assert.Equal(t, sha, actual)

			if tc.expectShallow {
				assert.FileExists(t, filepath.Join(e.WorkingDir, ".git", "shallow"))
			}
		})
	}
}

func TestGetSourcesGit_Fetch(t *testing.T) {
	tests := map[string]struct {
		depth         int
		checkout      bool
		runTwice      bool
		gitFetchFlags []string
		expectShallow bool
		expectFile    bool
	}{
		"basic": {
			checkout:   true,
			expectFile: true,
		},
		"idempotent (run twice)": {
			checkout:   true,
			runTwice:   true,
			expectFile: true,
		},
		"with depth": {
			depth:         1,
			checkout:      true,
			expectShallow: true,
			expectFile:    true,
		},
		"no checkout": {
			checkout:   false,
			expectFile: false,
		},
		"with extra fetch flags": {
			checkout:      true,
			gitFetchFlags: []string{"--no-tags"},
			expectFile:    true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			repoURL, sha, ref := testRepo(t)
			e := gitEnv(t, "bash")

			gs := stages.GetSources{
				GitStrategy:   "fetch",
				Checkout:      tc.checkout,
				Depth:         tc.depth,
				RepoURL:       repoURL,
				SHA:           sha,
				Ref:           ref,
				Refspecs:      []string{"+refs/heads/*:refs/remotes/origin/*"},
				GitFetchFlags: tc.gitFetchFlags,
				MaxAttempts:   1,
			}

			err := gs.Run(context.Background(), e)
			require.NoError(t, err, "stderr: %s", e.Stderr.(*bytes.Buffer).String())

			if tc.runTwice {
				err = gs.Run(context.Background(), e)
				require.NoError(t, err, "stderr: %s", e.Stderr.(*bytes.Buffer).String())
			}

			if tc.expectFile {
				assert.FileExists(t, filepath.Join(e.WorkingDir, "hello.txt"))
			} else {
				assert.DirExists(t, filepath.Join(e.WorkingDir, ".git"))
				assert.NoFileExists(t, filepath.Join(e.WorkingDir, "hello.txt"))
			}

			if tc.checkout {
				actual := runOutput(t, e.WorkingDir, "git", "rev-parse", "HEAD")
				assert.Equal(t, sha, actual)
			}

			if tc.expectShallow {
				assert.FileExists(t, filepath.Join(e.WorkingDir, ".git", "shallow"))
			}
		})
	}
}

func TestGetSourcesGit_Fetch_Unshallow(t *testing.T) {
	repoURL, sha, ref := testRepoWithHistory(t)

	e := gitEnv(t, "bash")

	// First: shallow fetch with depth=1
	shallow := stages.GetSources{
		GitStrategy: "fetch",
		Checkout:    true,
		Depth:       1,
		RepoURL:     repoURL,
		SHA:         sha,
		Ref:         ref,
		Refspecs:    []string{"+refs/heads/*:refs/remotes/origin/*"},
		MaxAttempts: 1,
	}

	require.NoError(t, shallow.Run(context.Background(), e))
	assert.FileExists(t, filepath.Join(e.WorkingDir, ".git", "shallow"))
	assert.Equal(t, 1, commitCount(t, e.WorkingDir))

	// Second: fetch with depth=0 should unshallow
	full := stages.GetSources{
		GitStrategy: "fetch",
		Checkout:    true,
		Depth:       0,
		RepoURL:     repoURL,
		SHA:         sha,
		Ref:         ref,
		Refspecs:    []string{"+refs/heads/*:refs/remotes/origin/*"},
		MaxAttempts: 1,
	}

	require.NoError(t, full.Run(context.Background(), e))
	assert.NoFileExists(t, filepath.Join(e.WorkingDir, ".git", "shallow"))
	assert.GreaterOrEqual(t, commitCount(t, e.WorkingDir), 2)
}

func TestGetSourcesGit_CleanFlags(t *testing.T) {
	tests := map[string]struct {
		strategy       string
		useNativeClone bool
	}{
		"fetch": {
			strategy: "fetch",
		},
		"clone": {
			strategy:       "clone",
			useNativeClone: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			repoURL, sha, ref := testRepo(t)
			e := gitEnv(t, "bash")

			gs := stages.GetSources{
				GitStrategy:    tc.strategy,
				Checkout:       true,
				RepoURL:        repoURL,
				SHA:            sha,
				Ref:            ref,
				Refspecs:       []string{"+refs/heads/*:refs/remotes/origin/*"},
				GitCleanFlags:  []string{"-ffdx"},
				MaxAttempts:    1,
				UseNativeClone: tc.useNativeClone,
			}

			// Initial run to establish the repo
			require.NoError(t, gs.Run(context.Background(), e))

			// Drop an untracked file
			require.NoError(t, os.WriteFile(filepath.Join(e.WorkingDir, "untracked.txt"), []byte("junk"), 0o644))

			// Re-run — clean should remove the untracked file.
			// For clone strategy this is a fresh clone, so the file is gone anyway.
			// For fetch strategy the clean step explicitly removes it.
			require.NoError(t, gs.Run(context.Background(), e))

			assert.NoFileExists(t, filepath.Join(e.WorkingDir, "untracked.txt"))
		})
	}
}

func TestGetSourcesGit_Submodules(t *testing.T) {
	tests := map[string]struct {
		strategy      string
		subDepth      int
		expectSubFile bool
	}{
		"normal": {
			strategy:      "normal",
			expectSubFile: true,
		},
		"recursive": {
			strategy:      "recursive",
			expectSubFile: true,
		},
		"none": {
			strategy:      "none",
			expectSubFile: false,
		},
		"normal with depth": {
			strategy:      "normal",
			subDepth:      1,
			expectSubFile: true,
		},
		"recursive with depth": {
			strategy:      "recursive",
			subDepth:      1,
			expectSubFile: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			repoURL, sha, ref := testRepoWithSubmodule(t)
			e := gitEnv(t, "bash")

			gs := stages.GetSources{
				GitStrategy:       "fetch",
				Checkout:          true,
				RepoURL:           repoURL,
				SHA:               sha,
				Ref:               ref,
				Refspecs:          []string{"+refs/heads/*:refs/remotes/origin/*"},
				SubmoduleStrategy: tc.strategy,
				SubmoduleDepth:    tc.subDepth,
				MaxAttempts:       1,
			}

			err := gs.Run(context.Background(), e)
			require.NoError(t, err, "stderr: %s", e.Stderr.(*bytes.Buffer).String())

			assert.FileExists(t, filepath.Join(e.WorkingDir, "main.txt"))

			if tc.expectSubFile {
				assert.FileExists(t, filepath.Join(e.WorkingDir, "mysub", "sub.txt"))
			} else {
				assert.NoFileExists(t, filepath.Join(e.WorkingDir, "mysub", "sub.txt"))
			}
		})
	}
}

func TestGetSourcesGit_Options(t *testing.T) {
	tests := map[string]struct {
		mutate func(gs *stages.GetSources)
		verify func(t *testing.T, e *env.Env)
	}{
		"LFS disabled": {
			mutate: func(gs *stages.GetSources) {
				gs.LFSDisabled = true
			},
			verify: func(t *testing.T, e *env.Env) {
				assert.FileExists(t, filepath.Join(e.WorkingDir, "hello.txt"))
			},
		},
		"safe directory checkout": {
			mutate: func(gs *stages.GetSources) {
				gs.SafeDirectoryCheckout = true
			},
			verify: func(t *testing.T, e *env.Env) {
				assert.FileExists(t, filepath.Join(e.WorkingDir, "hello.txt"))
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			repoURL, sha, ref := testRepo(t)
			e := gitEnv(t, "bash")

			gs := stages.GetSources{
				GitStrategy: "fetch",
				Checkout:    true,
				RepoURL:     repoURL,
				SHA:         sha,
				Ref:         ref,
				Refspecs:    []string{"+refs/heads/*:refs/remotes/origin/*"},
				MaxAttempts: 1,
			}

			tc.mutate(&gs)

			err := gs.Run(context.Background(), e)
			require.NoError(t, err, "stderr: %s", e.Stderr.(*bytes.Buffer).String())

			tc.verify(t, e)
		})
	}
}

func TestGetSourcesGit_ClearWorktreeOnRetry(t *testing.T) {
	repoURL, sha, ref := testRepo(t)
	e := gitEnv(t, "bash")

	// Establish a valid repo first.
	gs := stages.GetSources{
		GitStrategy: "fetch",
		Checkout:    true,
		RepoURL:     repoURL,
		SHA:         sha,
		Ref:         ref,
		Refspecs:    []string{"+refs/heads/*:refs/remotes/origin/*"},
		MaxAttempts: 1,
	}
	require.NoError(t, gs.Run(context.Background(), e))

	// Now fetch from an invalid URL with retry + clear worktree.
	gs2 := stages.GetSources{
		GitStrategy:          "fetch",
		Checkout:             true,
		RepoURL:              "file:///nonexistent/repo.git",
		SHA:                  sha,
		Ref:                  ref,
		Refspecs:             []string{"+refs/heads/*:refs/remotes/origin/*"},
		MaxAttempts:          2,
		ClearWorktreeOnRetry: true,
	}

	_ = gs2.Run(context.Background(), e)

	stderr := e.Stderr.(*bytes.Buffer).String()
	assert.Contains(t, stderr, "Deleting tracked and untracked files")
}

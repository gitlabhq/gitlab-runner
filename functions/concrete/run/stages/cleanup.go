package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
)

type Cleanup struct {
	GitStrategy       string   `json:"git_strategy,omitempty"`
	SubmoduleStrategy string   `json:"submodule_strategy,omitempty"`
	GitCleanFlags     []string `json:"git_clean_flags,omitempty"`
	EnableJobCleanup  bool     `json:"enable_job_cleanup,omitempty"`
	CleanGitConfig    bool     `json:"clean_git_config,omitempty"`
}

func (s Cleanup) Run(ctx context.Context, e *env.Env) error {
	if s.EnableJobCleanup {
		s.cleanBuildDirectory(ctx, e)
	}

	s.cleanGitState(e)

	return nil
}

func (s Cleanup) cleanBuildDirectory(ctx context.Context, e *env.Env) {
	projectDir := e.WorkingDir

	switch s.GitStrategy {
	case gitStrategyClone, gitStrategyEmpty:
		_ = os.RemoveAll(projectDir)

	case gitStrategyFetch:
		if len(s.GitCleanFlags) > 0 {
			_ = git(ctx, e, nil, append([]string{"clean"}, s.GitCleanFlags...)...)
		}

		_ = git(ctx, e, nil, "reset", "--hard")

		if s.hasSubmodules() {
			foreachArgs := []string{"submodule", "foreach"}
			if s.SubmoduleStrategy == submoduleStrategyRecursive {
				foreachArgs = append(foreachArgs, "--recursive")
			}

			if len(s.GitCleanFlags) > 0 {
				cleanCmd := "git clean " + strings.Join(s.GitCleanFlags, " ")
				_ = git(ctx, e, nil, append(foreachArgs, cleanCmd)...)
			}

			resetCmd := "git reset --hard"
			_ = git(ctx, e, nil, append(foreachArgs, resetCmd)...)
		}

	case gitStrategyNone:
		e.Noticef("Skipping build directory cleanup step")
	}
}

func (s Cleanup) cleanGitState(e *env.Env) {
	projectDir := e.WorkingDir
	dotGitDir := filepath.Join(projectDir, ".git")

	lockFiles := []string{"index.lock", "shallow.lock", "HEAD.lock", "config.lock"}
	for _, f := range lockFiles {
		_ = os.Remove(filepath.Join(dotGitDir, f))
	}
	_ = os.Remove(filepath.Join(dotGitDir, "hooks", "post-checkout"))

	if s.hasSubmodules() {
		modulesDir := filepath.Join(dotGitDir, "modules")
		for _, f := range lockFiles {
			walkRemove(modulesDir, f, false)
		}
		walkRemove(modulesDir, "post-checkout", false)
	}

	walkRemove(filepath.Join(dotGitDir, "refs"), ".lock", true)

	if !s.CleanGitConfig {
		return
	}

	tmpDir := e.WorkingDir + ".tmp"
	for _, dir := range []string{filepath.Join(tmpDir, templateDirName), dotGitDir} {
		_ = os.Remove(filepath.Join(dir, "config"))
		_ = os.RemoveAll(filepath.Join(dir, "hooks"))
	}

	if s.hasSubmodules() {
		modulesDir := filepath.Join(dotGitDir, "modules")
		walkRemove(modulesDir, "config", false)
		walkRemove(modulesDir, "hooks", false)
	}
}

func (s Cleanup) hasSubmodules() bool {
	return s.SubmoduleStrategy == submoduleStrategyNormal || s.SubmoduleStrategy == submoduleStrategyRecursive
}

package stages

import (
	"context"

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
	// todo: impl
	return nil
}

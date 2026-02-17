package stages

import (
	"context"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
)

type GetSources struct {
	AllowGitFetch     bool     `json:"allow_git_fetch,omitempty"`
	Checkout          bool     `json:"checkout,omitempty"`
	MaxAttempts       int      `json:"max_attempts,omitempty"`
	SubmoduleStrategy string   `json:"submodule_strategy,omitempty"`
	LFSDisabled       bool     `json:"lfs_disabled,omitempty"`
	Depth             int      `json:"depth,omitempty"`
	RepoURL           string   `json:"repo_url,omitempty"`
	Refspecs          []string `json:"refspecs,omitempty"`
	SHA               string   `json:"sha,omitempty"`
	ObjectFormat      string   `json:"object_format,omitempty"`

	GitStrategy   string   `json:"git_strategy,omitempty"`
	GitCloneFlags []string `json:"git_clone_flags,omitempty"`
	GitFetchFlags []string `json:"git_fetch_flags,omitempty"`
	GitCleanFlags []string `json:"git_clean_flags,omitempty"`

	Ref string `json:"ref,omitempty"`

	SubmoduleDepth       int      `json:"submodule_depth,omitempty"`
	SubmoduleUpdateFlags []string `json:"submodule_update_flags,omitempty"`
	SubmodulePaths       []string `json:"submodule_paths,omitempty"`

	PreCloneStep  Step `json:"pre_clone_step,omitempty"`
	PostCloneStep Step `json:"post_clone_step,omitempty"`

	ClearWorktreeOnRetry bool `json:"clear_worktree_on_retry,omitempty"`

	UseNativeClone        bool `json:"use_native_clone,omitempty"`
	UseBundleURIs         bool `json:"use_bundled_uris,omitempty"`
	SafeDirectoryCheckout bool `json:"safe_directory_checkout,omitempty"`

	UserAgent           string `json:"user_agent,omitempty"`
	GitalyCorrelationID string `json:"gitaly_correlation_id,omitempty"`

	RemoteHost  string `json:"remote_host,omitempty"`
	IsSharedEnv bool   `json:"is_shared_env,omitempty"`

	UseCredentialHelper bool `json:"use_credential_helper,omitempty"`

	InsteadOfs       [][2]string `json:"instead_ofs,omitempty"`
	CleanGitConfig   bool        `json:"clean_git_config,omitempty"`
	UseProactiveAuth bool        `json:"use_proactive_auth,omitempty"`
}

func (s GetSources) Run(ctx context.Context, e *env.Env) error {
	// todo: impl
	return nil
}

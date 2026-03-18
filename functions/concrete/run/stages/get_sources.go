package stages

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
)

const (
	credHelperCommand         = `!f(){ if [ "$1" = "get" ] ; then echo "password=${CI_JOB_TOKEN}" ; fi ; } ; f`
	gitMinVersionCloneWithRef = "2.49"
	templateDirName           = "git-template"

	gitStrategyNone  = "none"
	gitStrategyEmpty = "empty"
	gitStrategyFetch = "fetch"
	gitStrategyClone = "clone"

	submoduleStrategyNone      = "none"
	submoduleStrategyNormal    = "normal"
	submoduleStrategyRecursive = "recursive"
)

var gitVersionRe = regexp.MustCompile(`(\d+(?:\.\d+)+)`)

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

func (s GetSources) hasSubmodules() bool {
	return s.SubmoduleStrategy == submoduleStrategyNormal || s.SubmoduleStrategy == submoduleStrategyRecursive
}

//nolint:gocognit
func (s GetSources) Run(ctx context.Context, e *env.Env) error {
	switch s.GitStrategy {
	case gitStrategyNone:
		e.Noticef("Skipping Git repository setup")
		return os.MkdirAll(e.WorkingDir, 0o755)

	case gitStrategyEmpty:
		e.Noticef("Skipping Git repository setup and creating an empty build directory")
		if err := os.RemoveAll(e.WorkingDir); err != nil {
			return fmt.Errorf("removing project dir: %w", err)
		}
		return os.MkdirAll(e.WorkingDir, 0o755)

	case gitStrategyFetch, gitStrategyClone:
		// handled below

	default:
		return fmt.Errorf("unknown GIT_STRATEGY: %s", s.GitStrategy)
	}

	gitEnv := map[string]string{
		"GIT_TERMINAL_PROMPT": "0",
		"GCM_INTERACTIVE":     "Never",
	}
	if !s.LFSDisabled {
		gitEnv["GIT_LFS_SKIP_SMUDGE"] = "1"
	}

	if !s.IsSharedEnv {
		if err := s.writeGitSSLConfig(ctx, e, gitEnv, "--global"); err != nil {
			return fmt.Errorf("writing global git SSL config: %w", err)
		}
	}

	if err := s.PreCloneStep.Run(ctx, e); err != nil {
		return fmt.Errorf("pre_clone_script: %w", err)
	}

	s.cleanupGitState(e)

	var err error
	for attempt := 1; attempt <= s.MaxAttempts; attempt++ {
		if attempt > 1 {
			e.Warningf("Retrying git fetch (attempt %d/%d)...", attempt, s.MaxAttempts)
			if s.ClearWorktreeOnRetry && attempt == 2 {
				if clearErr := s.clearWorktree(ctx, e); clearErr != nil {
					e.Warningf("Failed to clear worktree: %v", clearErr)
				}
			}
		}

		err = s.getSourcesOnce(ctx, e, gitEnv)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}

	return s.PostCloneStep.Run(ctx, e)
}

//nolint:gocognit
func (s GetSources) getSourcesOnce(ctx context.Context, e *env.Env, gitEnv map[string]string) error {
	if s.GitStrategy == gitStrategyClone {
		if err := os.RemoveAll(e.WorkingDir); err != nil {
			return fmt.Errorf("removing project dir for clone: %w", err)
		}
		if err := os.MkdirAll(e.WorkingDir, 0o755); err != nil {
			return fmt.Errorf("recreating project dir: %w", err)
		}
	}

	globalCleanup, err := s.setupGlobalGitConfig(ctx, e, gitEnv)
	if err != nil {
		return err
	}
	defer globalCleanup()

	extConfigFile, cleanupConfig, err := s.setupExternalGitConfig(ctx, e, gitEnv)
	if err != nil {
		return fmt.Errorf("setting up git config: %w", err)
	}
	defer cleanupConfig()

	templateDir, cleanupTemplate, err := s.setupTemplateDir(e, extConfigFile)
	if err != nil {
		return fmt.Errorf("setting up template dir: %w", err)
	}
	defer cleanupTemplate()

	remoteURL := s.remoteURLWithoutCreds()

	if s.GitStrategy == gitStrategyClone && s.UseNativeClone && gitVersionAtLeast(ctx, gitMinVersionCloneWithRef) {
		if err := s.gitClone(ctx, e, templateDir, remoteURL, gitEnv); err != nil {
			return err
		}
	} else {
		if err := s.gitInit(ctx, e, templateDir, remoteURL, extConfigFile, gitEnv); err != nil {
			return err
		}
		if err := s.gitFetch(ctx, e, gitEnv); err != nil {
			return err
		}
	}

	if s.Checkout {
		if err := s.gitCheckout(ctx, e, gitEnv); err != nil {
			return err
		}
		if err := s.gitLFSPull(ctx, e, gitEnv); err != nil {
			return err
		}
	} else {
		e.Noticef("Skipping Git checkout")
	}

	return s.updateSubmodules(ctx, e, extConfigFile, gitEnv)
}

func (s GetSources) setupGlobalGitConfig(ctx context.Context, e *env.Env, gitEnv map[string]string) (func(), error) {
	tmpDir := e.WorkingDir + ".tmp"
	globalConfigFile := filepath.Join(tmpDir, ".gitconfig")

	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return func() {}, fmt.Errorf("creating tmp dir: %w", err)
	}

	// Seed with an include of the original global config if one exists.
	var content string
	if home := os.Getenv("HOME"); home != "" {
		existing := filepath.Join(home, ".gitconfig")
		if _, err := os.Stat(existing); err == nil {
			content = "[include]\n\tpath = " + existing + "\n"
		}
	}

	if err := os.WriteFile(globalConfigFile, []byte(content), 0o600); err != nil {
		return func() {}, fmt.Errorf("creating global config: %w", err)
	}

	cleanup := func() { _ = os.Remove(globalConfigFile) }

	// Point git at our writable global config.
	gitEnv["GIT_CONFIG_GLOBAL"] = globalConfigFile

	// safe.directory must be global — git ignores it at repo level.
	if s.SafeDirectoryCheckout {
		if err := git(ctx, e, gitEnv, "config", "--global", "--add", "safe.directory", e.WorkingDir); err != nil {
			return cleanup, fmt.Errorf("adding safe.directory: %w", err)
		}
	}

	return cleanup, nil
}

//nolint:gocognit
func (s GetSources) setupExternalGitConfig(ctx context.Context, e *env.Env, gitEnv map[string]string) (string, func(), error) {
	tmpDir := e.WorkingDir + ".tmp"
	extConfigFile := filepath.Join(tmpDir, ".gitlab-runner.ext.conf")
	noop := func() {}

	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", noop, fmt.Errorf("creating tmp dir: %w", err)
	}
	if err := os.WriteFile(extConfigFile, nil, 0o600); err != nil {
		return "", noop, fmt.Errorf("creating ext config file: %w", err)
	}

	cleanup := func() { _ = os.Remove(extConfigFile) }

	// Helper to set a config key in the external config file.
	setConfig := func(key, value, description string) error {
		if err := git(ctx, e, gitEnv, "config", "-f", extConfigFile, key, value); err != nil {
			return fmt.Errorf("setting %s: %w", description, err)
		}
		return nil
	}
	setConfigAll := func(key, value, pattern, description string) error {
		if err := git(ctx, e, gitEnv, "config", "-f", extConfigFile, "--replace-all", key, value, pattern); err != nil {
			return fmt.Errorf("setting %s: %w", description, err)
		}
		return nil
	}
	addConfig := func(key, value, description string) error {
		if err := git(ctx, e, gitEnv, "config", "-f", extConfigFile, "--add", key, value); err != nil {
			return fmt.Errorf("adding %s: %w", description, err)
		}
		return nil
	}

	if s.GitalyCorrelationID != "" {
		if err := setConfig("http.extraHeader", "X-Gitaly-Correlation-ID: "+s.GitalyCorrelationID, "gitaly correlation ID"); err != nil {
			return "", cleanup, err
		}
		e.Noticef("Gitaly correlation ID: %s", s.GitalyCorrelationID)
	}

	if s.UseBundleURIs {
		if err := setConfig("transfer.bundleURI", "true", "bundle URI config"); err != nil {
			return "", cleanup, err
		}
	}

	if s.IsSharedEnv {
		if err := s.writeGitSSLConfig(ctx, e, gitEnv, "-f", extConfigFile); err != nil {
			return "", cleanup, fmt.Errorf("writing git SSL config to ext config: %w", err)
		}
	}

	// Build and deduplicate insteadOf rules.
	parsed, err := url.Parse(s.RepoURL)
	if err != nil {
		return "", cleanup, fmt.Errorf("parsing repo URL: %w", err)
	}

	withCreds := parsed.String()
	without := *parsed
	without.User = nil
	withoutCreds := without.String()

	insteadOfs := make([][2]string, 0, 1+len(s.InsteadOfs))
	if withCreds != withoutCreds {
		insteadOfs = append(insteadOfs, [2]string{withCreds, withoutCreds})
	}
	insteadOfs = append(insteadOfs, s.InsteadOfs...)
	insteadOfs = deduplicateInsteadOfs(insteadOfs)

	for _, io := range insteadOfs {
		stanza := "url." + io[0] + ".insteadOf"
		pattern := "^" + regexp.QuoteMeta(io[1]) + "$"
		if err := setConfigAll(stanza, io[1], pattern, "insteadOf for "+io[1]); err != nil {
			return "", cleanup, err
		}
	}

	// Set up the credential helper matching the bash implementation:
	//   1. --replace-all helper to "" (resets the helper chain, ignoring higher-scope helpers)
	//   2. --add helper with the actual credential command
	//   3. set the username
	if s.UseCredentialHelper && s.RemoteHost != "" {
		credKey := "credential." + s.RemoteHost

		if err := setConfigAll(credKey+".helper", "", ".*", "credential helper reset"); err != nil {
			return "", cleanup, err
		}
		if err := addConfig(credKey+".helper", credHelperCommand, "credential helper command"); err != nil {
			return "", cleanup, err
		}
		if err := setConfig(credKey+".username", "gitlab-ci-token", "credential username"); err != nil {
			return "", cleanup, err
		}
	}

	return extConfigFile, cleanup, nil
}

func (s GetSources) setupTemplateDir(e *env.Env, extConfigFile string) (string, func(), error) {
	templateDir := filepath.Join(e.WorkingDir+".tmp", templateDirName)
	_ = os.RemoveAll(templateDir)

	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		return "", func() {}, fmt.Errorf("creating template dir: %w", err)
	}

	cleanup := func() { _ = os.RemoveAll(templateDir) }

	absExtConfig, err := filepath.Abs(extConfigFile)
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("resolving ext config path: %w", err)
	}

	content := strings.Join([]string{
		"[init]", "\tdefaultBranch = none",
		"[fetch]", "\trecurseSubmodules = false",
		"[credential]", "\tinteractive = never",
		"[gc]", "\tautoDetach = false",
		"[include]", fmt.Sprintf("\tpath = %s", filepath.ToSlash(absExtConfig)),
	}, "\n") + "\n"

	if err := os.WriteFile(filepath.Join(templateDir, "config"), []byte(content), 0o644); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("writing template config: %w", err)
	}

	return templateDir, cleanup, nil
}

// writeGitSSLConfig writes per-host SSL/TLS configuration. The where args are
// prepended to the git config invocation (e.g. "--global" or "-f", path).
func (s GetSources) writeGitSSLConfig(ctx context.Context, e *env.Env, gitEnv map[string]string, where ...string) error {
	if s.RemoteHost == "" {
		return nil
	}
	if e.Env == nil {
		return nil
	}

	for _, entry := range []struct{ file, key string }{
		{e.Env["CI_SERVER_TLS_CA_FILE"], "sslCAInfo"},
		{e.Env["CI_SERVER_TLS_CERT_FILE"], "sslCert"},
		{e.Env["CI_SERVER_TLS_KEY_FILE"], "sslKey"},
	} {
		if entry.file == "" {
			continue
		}
		args := append([]string{"config"}, where...)
		args = append(args, fmt.Sprintf("http.%s.%s", s.RemoteHost, entry.key), entry.file)
		if err := git(ctx, e, gitEnv, args...); err != nil {
			return fmt.Errorf("setting git SSL config %s: %w", entry.key, err)
		}
	}

	return nil
}

func (s GetSources) remoteURLWithoutCreds() string {
	parsed, err := url.Parse(s.RepoURL)
	if err != nil {
		return s.RepoURL
	}
	parsed.User = nil
	return parsed.String()
}

// cleanupGitState removes stale lock files and (when CleanGitConfig is set)
// potentially-malicious git configs and hooks from prior jobs.
func (s GetSources) cleanupGitState(e *env.Env) {
	dotGitDir := filepath.Join(e.WorkingDir, ".git")

	// Remove lock files and stale post-checkout hook.
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
		// The old shell code also removed post-checkout recursively in modules.
		walkRemove(modulesDir, "post-checkout", false)
	}

	walkRemove(filepath.Join(dotGitDir, "refs"), ".lock", true)

	// Clean configs and hooks if requested.
	if !s.CleanGitConfig {
		return
	}

	for _, dir := range []string{filepath.Join(e.WorkingDir+".tmp", templateDirName), dotGitDir} {
		_ = os.Remove(filepath.Join(dir, "config"))
		_ = os.RemoveAll(filepath.Join(dir, "hooks"))
	}
	if s.hasSubmodules() {
		modulesDir := filepath.Join(dotGitDir, "modules")
		walkRemove(modulesDir, "config", false)
		walkRemove(modulesDir, "hooks", false)
	}
}

func (s GetSources) gitInit(ctx context.Context, e *env.Env, templateDir, remoteURL, extConfigFile string, extraEnv map[string]string) error {
	args := []string{"init", ".", "--template", templateDir}
	if s.ObjectFormat != "" && s.ObjectFormat != "sha1" {
		args = append(args, "--object-format", s.ObjectFormat)
	}

	if err := git(ctx, e, extraEnv, args...); err != nil {
		return fmt.Errorf("git init: %w", err)
	}

	if err := git(ctx, e, extraEnv, "remote", "add", "origin", remoteURL); err != nil {
		if err := git(ctx, e, extraEnv, "remote", "set-url", "origin", remoteURL); err != nil {
			return fmt.Errorf("setting remote URL: %w", err)
		}
		// For existing repos the template isn't reapplied — explicitly include
		// the external config.
		absExtConfig, _ := filepath.Abs(extConfigFile)
		pattern := regexp.QuoteMeta(filepath.Base(extConfigFile)) + "$"
		if err := git(ctx, e, extraEnv,
			"config", "--file", filepath.Join(e.WorkingDir, ".git", "config"),
			"--replace-all", "include.path", absExtConfig, pattern,
		); err != nil {
			e.Warningf("Failed to configure include.path for existing repo: %v", err)
		}
	} else {
		e.Noticef("Created fresh repository.")
	}

	return nil
}

func (s GetSources) gitFetch(ctx context.Context, e *env.Env, extraEnv map[string]string) error {
	if s.Depth > 0 {
		e.Noticef("Fetching changes with git depth set to %d...", s.Depth)
	} else {
		e.Noticef("Fetching changes...")
	}

	fetchArgs := s.configArgs()
	fetchArgs = append(fetchArgs, "fetch", "origin", "--no-recurse-submodules")
	fetchArgs = append(fetchArgs, s.Refspecs...)
	if s.Depth > 0 {
		fetchArgs = append(fetchArgs, "--depth", strconv.Itoa(s.Depth))
	}
	fetchArgs = append(fetchArgs, s.GitFetchFlags...)

	if s.Depth <= 0 && isShallowRepo(e.WorkingDir) {
		if err := git(ctx, e, extraEnv, append(fetchArgs, "--unshallow")...); err == nil {
			return nil
		}
	}

	return git(ctx, e, extraEnv, fetchArgs...)
}

func (s GetSources) gitClone(ctx context.Context, e *env.Env, templateDir, remoteURL string, extraEnv map[string]string) error {
	switch {
	case s.Depth > 0:
		e.Noticef("Cloning repository for %s with git depth set to %d...", s.Ref, s.Depth)
	case s.Ref != "":
		e.Noticef("Cloning repository for %s...", s.Ref)
	default:
		e.Noticef("Cloning repository...")
	}

	cloneArgs := s.configArgs()
	cloneArgs = append(cloneArgs, "clone", "--no-checkout", remoteURL, ".", "--template", templateDir)
	if s.Depth > 0 {
		cloneArgs = append(cloneArgs, "--depth", strconv.Itoa(s.Depth))
	}
	if strings.HasPrefix(s.Ref, "refs/") {
		cloneArgs = append(cloneArgs, "--revision", s.Ref)
	} else if s.Ref != "" {
		cloneArgs = append(cloneArgs, "--branch", s.Ref)
	}
	cloneArgs = append(cloneArgs, s.GitCloneFlags...)

	if err := git(ctx, e, extraEnv, cloneArgs...); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	return nil
}

func (s GetSources) gitCheckout(ctx context.Context, e *env.Env, extraEnv map[string]string) error {
	short := s.SHA
	if len(short) > 8 {
		short = short[:8]
	}
	e.Noticef("Checking out %s as detached HEAD (ref is %s)...", short, s.Ref)

	if err := git(ctx, e, extraEnv, "-c", "submodule.recurse=false", "checkout", "-f", "-q", s.SHA); err != nil {
		return fmt.Errorf("git checkout: %w", err)
	}

	if len(s.GitCleanFlags) > 0 {
		if err := git(ctx, e, extraEnv, append([]string{"clean"}, s.GitCleanFlags...)...); err != nil {
			return fmt.Errorf("git clean: %w", err)
		}
	}

	return nil
}

func (s GetSources) gitLFSPull(ctx context.Context, e *env.Env, extraEnv map[string]string) error {
	if s.LFSDisabled || !hasCommand(ctx, "git", "lfs", "version") {
		return nil
	}
	return git(ctx, e, extraEnv, "lfs", "pull")
}

func (s GetSources) updateSubmodules(ctx context.Context, e *env.Env, extConfigFile string, extraEnv map[string]string) error {
	switch s.SubmoduleStrategy {
	case submoduleStrategyNone, "":
		e.Noticef("Skipping Git submodules setup")
		return nil
	case submoduleStrategyNormal:
		return s.doSubmoduleUpdate(ctx, e, extConfigFile, extraEnv, false)
	case submoduleStrategyRecursive:
		return s.doSubmoduleUpdate(ctx, e, extConfigFile, extraEnv, true)
	default:
		return fmt.Errorf("unknown GIT_SUBMODULE_STRATEGY: %s", s.SubmoduleStrategy)
	}
}

//nolint:gocognit
func (s GetSources) doSubmoduleUpdate(ctx context.Context, e *env.Env, extConfigFile string, extraEnv map[string]string, recursive bool) error {
	switch {
	case recursive && s.SubmoduleDepth > 0:
		e.Noticef("Updating/initializing submodules recursively with git depth set to %d...", s.SubmoduleDepth)
	case recursive:
		e.Noticef("Updating/initializing submodules recursively...")
	case s.SubmoduleDepth > 0:
		e.Noticef("Updating/initializing submodules with git depth set to %d...", s.SubmoduleDepth)
	default:
		e.Noticef("Updating/initializing submodules...")
	}

	if err := git(ctx, e, extraEnv, "submodule", "init"); err != nil {
		return fmt.Errorf("submodule init: %w", err)
	}

	syncArgs := []string{"submodule", "sync"}
	if recursive {
		syncArgs = append(syncArgs, "--recursive")
	}
	syncArgs = append(syncArgs, s.submodulePathArgs()...)

	if err := git(ctx, e, extraEnv, syncArgs...); err != nil {
		return fmt.Errorf("submodule sync: %w", err)
	}

	foreachArgs := []string{"submodule", "foreach"}
	if recursive {
		foreachArgs = append(foreachArgs, "--recursive")
	}

	// foreach runs a shell command via git submodule foreach.
	foreach := func(cmd string) error {
		return git(ctx, e, extraEnv, append(foreachArgs, cmd)...)
	}

	cleanFlags := s.GitCleanFlags
	if len(cleanFlags) == 0 {
		cleanFlags = []string{"-ffdx"}
	}
	cleanCmd := "git clean " + strings.Join(cleanFlags, " ")

	_ = foreach(cleanCmd)
	_ = foreach("git reset --hard")

	absExtConfig, _ := filepath.Abs(extConfigFile)
	withCreds := func(args []string) []string {
		return append([]string{"-c", "include.path=" + absExtConfig}, args...)
	}

	updateArgs := []string{"submodule", "update", "--init"}
	if recursive {
		updateArgs = append(updateArgs, "--recursive")
	}
	if s.SubmoduleDepth > 0 {
		updateArgs = append(updateArgs, "--depth", strconv.Itoa(s.SubmoduleDepth))
	}
	updateArgs = append(updateArgs, s.SubmoduleUpdateFlags...)
	updateArgs = append(updateArgs, s.submodulePathArgs()...)

	if err := git(ctx, e, extraEnv, withCreds(updateArgs)...); err != nil {
		e.Warningf("Updating submodules failed. Retrying...")

		if s.hasRemoteFlag() {
			_ = git(ctx, e, extraEnv, withCreds(append(foreachArgs, "git fetch origin +refs/heads/*:refs/remotes/origin/*"))...)
		}

		_ = git(ctx, e, extraEnv, syncArgs...)
		if err := git(ctx, e, extraEnv, withCreds(updateArgs)...); err != nil {
			return fmt.Errorf("submodule update (retry): %w", err)
		}
		_ = foreach("git reset --hard")
	} else {
		e.Noticef("Updated submodules")
		_ = git(ctx, e, extraEnv, syncArgs...)
	}

	_ = foreach(cleanCmd)

	// Configure all submodules (always recursive) to include the external git
	// config so that git operations in submodule dirs authenticate properly.
	e.Noticef("Configuring submodules to use parent git credentials...")
	credCmd := fmt.Sprintf("git config --replace-all include.path '%s'", absExtConfig)
	_ = git(ctx, e, extraEnv, "submodule", "foreach", "--recursive", credCmd)

	if !s.LFSDisabled && hasCommand(ctx, "git", "lfs", "version") {
		e.Noticef("Pulling LFS files for submodules...")
		_ = git(ctx, e, extraEnv, withCreds(append(foreachArgs, "git lfs pull"))...)
	}

	return nil
}

func (s GetSources) submodulePathArgs() []string {
	if len(s.SubmodulePaths) == 0 {
		return nil
	}
	return append([]string{"--"}, s.SubmodulePaths...)
}

func (s GetSources) hasRemoteFlag() bool {
	for _, f := range s.SubmoduleUpdateFlags {
		if strings.EqualFold(f, "--remote") {
			return true
		}
	}
	return false
}

func (s GetSources) configArgs() []string {
	var args []string
	if s.UserAgent != "" {
		args = append(args, "-c", "http.userAgent="+s.UserAgent)
	}
	if s.UseProactiveAuth {
		args = append(args, "-c", "http.proactiveAuth=basic")
	}
	return args
}

func (s GetSources) clearWorktree(ctx context.Context, e *env.Env) error {
	e.Noticef("Deleting tracked and untracked files...")

	info, err := os.Stat(e.WorkingDir)
	if err != nil || !info.IsDir() {
		return nil
	}

	if err := git(ctx, e, nil, "rm", "-rf", "--ignore-unmatch", "."); err != nil {
		return err
	}

	return git(ctx, e, nil, "clean", "-ffdx")
}

// --- helpers ---

func git(ctx context.Context, e *env.Env, extraEnv map[string]string, args ...string) error {
	return e.Command(ctx, "git", extraEnv, args...)
}

func hasCommand(ctx context.Context, name string, args ...string) bool {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func isShallowRepo(projectDir string) bool {
	_, err := os.Stat(filepath.Join(projectDir, ".git", "shallow"))
	return err == nil
}

func deduplicateInsteadOfs(insteadOfs [][2]string) [][2]string {
	seen := make(map[[2]string]bool, len(insteadOfs))
	result := make([][2]string, 0, len(insteadOfs))
	for _, io := range insteadOfs {
		if !seen[io] {
			seen[io] = true
			result = append(result, io)
		}
	}
	return result
}

func gitVersionAtLeast(ctx context.Context, minVersion string) bool {
	cmd := exec.CommandContext(ctx, "git", "--version")
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	match := gitVersionRe.FindString(string(out))
	if match == "" {
		return false
	}

	return compareVersions(match, minVersion) >= 0
}

func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := range max(len(aParts), len(bParts)) {
		var aNum, bNum int
		if i < len(aParts) {
			aNum, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bNum, _ = strconv.Atoi(bParts[i])
		}
		if aNum != bNum {
			if aNum < bNum {
				return -1
			}
			return 1
		}
	}

	return 0
}

// walkRemove walks dir and removes entries matching name. If bySuffix is true,
// it matches files/dirs whose name ends with the given suffix; otherwise it
// matches exactly. Directories are removed entirely (os.RemoveAll).
func walkRemove(dir, name string, bySuffix bool) {
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}

		base := filepath.Base(p)
		match := base == name
		if bySuffix {
			match = strings.HasSuffix(base, name)
		}
		if !match {
			return nil
		}

		if info.IsDir() {
			_ = os.RemoveAll(p)
			return filepath.SkipDir
		}
		_ = os.Remove(p)
		return nil
	})
}

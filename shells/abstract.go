package shells

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
)

const (
	// When umask is disabled for the Kubernetes executor,
	// a hidden file, .gitlab-build-uid-gid, is created in the `builds_dir` directory to assist the helper container
	// in retrieving the build image's configured `uid:gid`.
	// This information is then applied to the working directories to prevent them from being writable by anyone.
	BuildUidGidFile           = ".gitlab-build-uid-gid"
	StartupProbeFile          = ".gitlab-startup-marker"
	gitlabEnvFileName         = "gitlab_runner_env"
	gitlabCacheEnvFileName    = "gitlab_runner_cache_env"
	gitDir                    = ".git"
	gitTemplateDir            = "git-template"
	gitMinVersionCloneWithRef = "2.49"
)

const (
	// externalGitConfigFile is the base name for externalized git config.
	// The externalized config file holds
	//	- config specific for the repo
	//	- insteadOf configs
	// so that we can
	//	- drop creds in there, without exposing them inside the build dir
	//	- support alternative URL formats (git+ssh, ...) for repo
	externalGitConfigFile = ".gitlab-runner.ext.conf"

	// credHelperCommand is the command to use as a git credential helper to pull credentials from the environment.
	// git always comes (or depends) on a POSIX shell, so any helper can rely on that, regardless of the OS, git distribution, ...
	credHelperCommand = `!f(){ if [ "$1" = "get" ] ; then echo "password=${CI_JOB_TOKEN}" ; fi ; } ; f`

	// envVarExternalGitConfigFile is the environment variable name for the variable holding the **absolute** path to the
	// externalized git configuration. This can only be known at runtime (e.g. see: relative builds_dir), and therefore
	// needs to be set up when the main repo is configured/pulled, so that it can then be used explicitly for submodule
	// operations with auth.
	envVarExternalGitConfigFile = "GLR_EXT_GIT_CONFIG_PATH"
)

var errUnknownGitStrategy = errors.New("unknown GIT_STRATEGY")

type stringQuoter func(string) string

func singleQuote(s string) string {
	return fmt.Sprintf(`'%s'`, s)
}

func doubleQuote(s string) string {
	return `"` + s + `"`
}

type AbstractShell struct{}

func (b *AbstractShell) GetFeatures(features *common.FeaturesInfo) {
	features.Artifacts = true
	features.UploadMultipleArtifacts = true
	features.UploadRawArtifacts = true
	features.Cache = true
	features.FallbackCacheKeys = true
	features.Refspecs = true
	features.Masking = true
	features.RawVariables = true
	features.ArtifactsExclude = true
	features.MultiBuildSteps = true
	features.VaultSecrets = true
	features.ReturnExitCode = true
}

func (b *AbstractShell) writeCdBuildDir(w ShellWriter, info common.ShellScriptInfo) {
	w.Cd(info.Build.FullProjectDir())
}

type cacheConfig struct {
	// the human readable key, which can be used for logging. It might be sanitized, if not running in hasehd mode.
	HumanKey string
	// the hashed key, which can be used to build URLs. It might be the same as the human-readable key, if not running in
	// hashed mode.
	HashedKey string
	// the archive file path, where the local cache is (to be) stored.
	ArchiveFile string
}

// newCacheConfig creates a cacheConfig for a provided build and userKey.
// If the userKey is empty, it is defaulted to `${jobName}/${gitRef}`.
// Based on the build configuration (ie. FFs), the cacheConfig provides either a sanitized/human-readable cache
// key, or raw/hashed cache key.
// Additionally, keyChecks can be provided, which validate cache keys just after sanitation.
func newCacheConfig(build *common.Build, userKey string, keyChecks ...func(string) bool) (*cacheConfig, string, error) {
	if build.CacheDir == "" {
		return nil, "", fmt.Errorf("unset cache directory")
	}

	rawKey := path.Join("/", build.JobInfo.Name, build.GitInfo.Ref)[1:]
	if userKey != "" {
		rawKey = build.GetAllVariables().ExpandValue(userKey)
	}

	// hashers per mode: nop in unhashed mode, sha256sum in hashed mode
	hashers := map[bool]func(string) string{
		false: func(s string) string { return s },
		true:  func(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) },
	}
	// sanitizers per mode: real sanitizer in unhashed mode, nop sanitizer in hashed mode
	sanitizers := map[bool]func(string) (string, error){
		false: sanitizeCacheKey,
		true:  func(s string) (string, error) { return s, nil },
	}

	hashCacheKeys := build.IsFeatureFlagOn(featureflags.HashCacheKeys)
	hasher, sanitizer := hashers[hashCacheKeys], sanitizers[hashCacheKeys]

	var warning string
	humanKey, err := sanitizer(rawKey)
	if err != nil {
		warning = err.Error()
	}

	for _, check := range keyChecks {
		if !check(humanKey) {
			// if a key check does not succeed, we drop out immediately
			return nil, warning, nil
		}
	}

	if humanKey == "" {
		return nil, warning, fmt.Errorf("empty cache key")
	}

	hashedKey := hasher(humanKey)

	getArchivePath := func(key string) (string, error) {
		var err error
		file := path.Join(build.CacheDir, key, "cache.zip")
		if !build.IsFeatureFlagOn(featureflags.UsePowershellPathResolver) {
			file, err = filepath.Rel(build.BuildDir, file)
			if err != nil {
				return "", fmt.Errorf("inability to make the cache file path relative to the build directory (is the build directory absolute?)")
			}
		}
		return file, err
	}

	archiveFile, err := getArchivePath(hashedKey)
	if err != nil {
		return nil, warning, err
	}

	cacheConfig := &cacheConfig{
		HumanKey:    humanKey,
		HashedKey:   hashedKey,
		ArchiveFile: archiveFile,
	}

	return cacheConfig, warning, nil
}

func (b *AbstractShell) guardRunnerCommand(w ShellWriter, runnerCommand string, action string, f func()) {
	if runnerCommand == "" {
		w.Warningf("%s is not supported by this executor.", action)
		return
	}

	w.IfCmd(runnerCommand, "--version")
	f()
	w.Else()
	w.Warningf("Missing %s. %s is disabled.", runnerCommand, action)
	w.EndIf()
}

func (b *AbstractShell) cacheExtractor(ctx context.Context, w ShellWriter, info common.ShellScriptInfo) error {
	skipRestoreCache := true

	for _, cacheOptions := range info.Build.Cache {
		// Create list of files to extract
		var archiverArgs []string
		for _, path := range cacheOptions.Paths {
			archiverArgs = append(archiverArgs, "--path", path)
		}

		if cacheOptions.Untracked {
			archiverArgs = append(archiverArgs, "--untracked")
		}

		// Skip restoring cache if no cache is defined
		if len(archiverArgs) < 1 {
			continue
		}

		skipRestoreCache = false

		// Skip extraction if no cache is defined
		cacheConfig, warning, err := newCacheConfig(info.Build, cacheOptions.Key)
		if warning != "" {
			w.Warningf(warning)
		}
		if err != nil {
			w.Noticef("Skipping cache extraction due to %v", err)
			continue
		}

		cacheOptions.Policy = spec.CachePolicy(info.Build.GetAllVariables().ExpandValue(string(cacheOptions.Policy)))

		if ok, err := cacheOptions.CheckPolicy(spec.CachePolicyPull); err != nil {
			return fmt.Errorf("%w for %s", err, cacheConfig.HumanKey)
		} else if !ok {
			w.Noticef("Not downloading cache %s due to policy", cacheConfig.HumanKey)
			continue
		}

		b.extractCacheOrFallbackCachesWrapper(ctx, w, info, *cacheConfig, cacheOptions)
	}

	if skipRestoreCache {
		return common.ErrSkipBuildStage
	}

	// Caches and artifacts are managed in the helper container, thus all files
	// would be owned by the user of the helper container.
	// We want all the files to be owned by the user of the build container. Thus we
	// change the ownership in the helper container (to the uid/gid we discovered via
	// the `init-build-uid-gid-collector` container) before the build in the build
	// container runs.
	// We change the directories
	// - project root dir, after extracting the cache/artifact files
	// - the cache dir
	// so that all dirs/files eventually have the correct ownership.
	b.changeFilesOwnership(w, info, info.Build.CacheDir, info.Build.RootDir)

	return nil
}

func (b *AbstractShell) extractCacheOrFallbackCachesWrapper(
	ctx context.Context,
	w ShellWriter,
	info common.ShellScriptInfo,
	initialCacheConfig cacheConfig,
	cacheOptions spec.Cache,
) {
	// the "default" cache key
	cacheConfigs := []cacheConfig{initialCacheConfig}

	build := info.Build
	buildVars := build.GetAllVariables()
	addCacheConfig := func(key string, keyChecks ...func(s string) bool) {
		if key == "" {
			return
		}
		cc, warning, err := newCacheConfig(build, key, keyChecks...)
		if warning != "" {
			w.Warningf(warning)
		}
		if err != nil {
			w.Noticef("Skipping cache extraction due to %v", err)
			return
		}
		if cc != nil {
			cacheConfigs = append(cacheConfigs, *cc)
		}
	}

	// the fallback cache keys from the cache config
	for _, cacheKey := range cacheOptions.FallbackKeys {
		addCacheConfig(buildVars.ExpandValue(cacheKey))
	}

	// the fallback key from CACHE_FALLBACK_KEY
	blockProtectedFallback := func(key string) bool {
		const blockedSuffix = "-protected"
		allowed := !strings.HasSuffix(key, blockedSuffix)
		if !allowed {
			w.Warningf("CACHE_FALLBACK_KEY %q not allowed to end in %q", key, blockedSuffix)
		}
		return allowed
	}
	addCacheConfig(buildVars.Value("CACHE_FALLBACK_KEY"), blockProtectedFallback)

	// Execute cache-extractor command. Failure is not fatal.
	b.guardRunnerCommand(w, info.RunnerCommand, "Extracting cache", func() {
		b.addExtractCacheCommand(ctx, w, info, cacheConfigs, cacheOptions.Paths)
	})
}

// sanitizeCacheKey replicates some cache key rules from GitLab and adds additional validations for known-bad cache
// keys.
// It accepts that the cache keys can be paths and:
//   - replaces the URL encoded version of `/` & `.` with their ASCII ones
//   - replaces all `\` with `/`
//   - ensures there are no path traversals possible "outside of the base path"
//   - ensures the last path element is not empty or ends in a space
func sanitizeCacheKey(cacheKey string) (sanitizedKey string, err error) {
	if cacheKey == "" {
		return "", nil
	}

	replaceEncodedSlashes := func(s string) string {
		for _, slash := range []string{"%2f", "%2F"} {
			s = strings.ReplaceAll(s, slash, "/")
		}
		return s
	}
	replaceEncodedDots := func(s string) string {
		for _, dot := range []string{"%2e", "%2E"} {
			s = strings.ReplaceAll(s, dot, ".")
		}
		return s
	}
	toSlash := func(s string) string {
		return strings.ReplaceAll(s, `\`, `/`)
	}
	trimSpaceRight := func(s string) string {
		return strings.TrimRightFunc(s, unicode.IsSpace)
	}

	// We root the path, so that path traversals outside of the base path, e.g. resulting in a key like `../../foo/bar`,
	// aren't possible
	// Note: path.Join calls path.Clean internally
	cleaned := path.Join(
		"/", toSlash(replaceEncodedSlashes(replaceEncodedDots(cacheKey))),
	)

	var key string
	for cleaned != "" {
		dir, file := path.Split(cleaned)
		file = trimSpaceRight(file)

		if file == "" {
			cleaned = dir[:len(dir)-1] // cut off the trailing `/` from dir and continue with that
			continue
		}

		key = path.Join(dir[1:], file) // cut off the leading `/` from dir, because we rooted the path initially
		break
	}

	if key == "" {
		return "", fmt.Errorf("cache key %q could not be sanitized", cacheKey)
	}
	if key != cacheKey {
		return key, fmt.Errorf("cache key %q sanitized to %q", cacheKey, key)
	}

	return key, nil
}

func (b *AbstractShell) addExtractCacheCommand(
	ctx context.Context,
	w ShellWriter,
	info common.ShellScriptInfo,
	cacheConfigs []cacheConfig,
	cachePaths []string,
) {
	cacheConfig := cacheConfigs[0]

	args := []string{
		"cache-extractor",
		"--file", cacheConfig.ArchiveFile,
		"--timeout", strconv.Itoa(info.Build.GetCacheRequestTimeout()),
	}

	w.Noticef("Checking cache for %s...", cacheConfig.HumanKey)

	extraArgs, env, err := getCacheDownloadURLAndEnv(ctx, info.Build, cacheConfig.HashedKey)
	args = append(args, extraArgs...)
	if err != nil {
		w.Warningf("Failed to obtain environment for cache %s: %v", cacheConfig.HumanKey, err)
	}
	if env != nil {
		cacheEnvFilename := b.writeCacheExports(w, env)
		args = append(args, "--env-file", cacheEnvFilename)
		defer w.RmFile(cacheEnvFilename)
	}

	w.IfCmdWithOutput(info.RunnerCommand, args...)
	w.Noticef("Successfully extracted cache")
	w.Else()
	w.Warningf("Failed to extract cache")

	// When extraction fails, remove the cache directories to avoid problems in cases
	// where archives may have been partially extracted, leaving the cache in an inconsistent
	// state. If we attempt to extract from fallback caches below, we'll remove the same set
	// of directories if that fails.
	if info.Build.IsFeatureFlagOn(featureflags.CleanUpFailedCacheExtract) {
		for _, cachePath := range cachePaths {
			w.Printf("Removing %s", cachePath)
			w.RmDir(cachePath)
		}
	}

	// We check that there is another key than the one we just used
	if len(cacheConfigs) > 1 {
		b.addExtractCacheCommand(ctx, w, info, cacheConfigs[1:], cachePaths)
	}
	w.EndIf()
}

// getCacheDownloadURLAndEnv will first try to generate the GoCloud URL if it's
// available then fallback to a pre-signed URL.
func getCacheDownloadURLAndEnv(ctx context.Context, build *common.Build, cacheKey string) ([]string, map[string]string, error) {
	adapter := cache.GetAdapter(build.Runner.Cache, build.GetBuildTimeout(), build.Runner.ShortDescription(), fmt.Sprintf("%d", build.JobInfo.ProjectID), cacheKey)

	// Prefer Go Cloud URL if supported
	goCloudURL, err := adapter.GetGoCloudURL(ctx, false)

	if goCloudURL.URL != nil {
		return []string{"--gocloud-url", goCloudURL.URL.String()}, goCloudURL.Environment, err
	}

	if url := adapter.GetDownloadURL(ctx); url.URL != nil {
		return []string{"--url", url.URL.String()}, nil, nil
	}

	return []string{}, nil, nil
}

func (b *AbstractShell) downloadArtifacts(w ShellWriter, job spec.Dependency, info common.ShellScriptInfo) {
	args := []string{
		"artifacts-downloader",
		"--url",
		info.Build.Runner.URL,
		"--token",
		job.Token,
		"--id",
		strconv.FormatInt(job.ID, 10),
	}

	w.Noticef("Downloading artifacts for %s (%d)...", job.Name, job.ID)
	w.Command(info.RunnerCommand, args...)
}

func (b *AbstractShell) jobArtifacts(info common.ShellScriptInfo) (otherJobs []spec.Dependency) {
	for _, otherJob := range info.Build.Dependencies {
		if otherJob.ArtifactsFile.Filename == "" {
			continue
		}

		otherJobs = append(otherJobs, otherJob)
	}
	return
}

func (b *AbstractShell) downloadAllArtifacts(w ShellWriter, info common.ShellScriptInfo) error {
	otherJobs := b.jobArtifacts(info)
	if len(otherJobs) == 0 {
		return common.ErrSkipBuildStage
	}

	b.guardRunnerCommand(w, info.RunnerCommand, "Artifacts downloading", func() {
		for _, otherJob := range otherJobs {
			b.downloadArtifacts(w, otherJob, info)
		}
	})

	// Caches and artifacts are managed in the helper container, thus all files
	// would be owned by the user of the helper container.
	// We want all the files to be owned by the user of the build container. Thus we
	// change the ownership in the helper container (to the uid/gid we discovered via
	// the `init-build-uid-gid-collector` container) before the build in the build
	// container runs.
	// We change the directories
	// - project root dir, after extracting the cache/artifact files
	// - the cache dir
	// so that all dirs/files eventually have the correct ownership.
	b.changeFilesOwnership(w, info, info.Build.CacheDir, info.Build.RootDir)

	return nil
}

func (b *AbstractShell) writePrepareScript(_ context.Context, w ShellWriter, _ common.ShellScriptInfo) error {
	w.RmFile(w.TmpFile(gitlabEnvFileName))
	w.RmFile(w.TmpFile("masking.db"))
	return nil
}

func (b *AbstractShell) writeGetSourcesScript(_ context.Context, w ShellWriter, info common.ShellScriptInfo) error {
	b.writeExports(w, info)

	w.Variable(spec.Variable{Key: "GIT_TERMINAL_PROMPT", Value: "0"})
	w.Variable(spec.Variable{Key: "GCM_INTERACTIVE", Value: "Never"})

	if !info.Build.IsSharedEnv() {
		b.writeGitSSLConfig(w, info.Build, []string{"--global"})
	}

	b.guardGetSourcesScriptHooks(w, info, "pre_clone_script", func() []string {
		var s []string

		if info.PreGetSourcesScript != "" {
			s = append(s, info.PreGetSourcesScript)
		}

		h := info.Build.Hooks.Get(spec.HookPreGetSourcesScript)
		if len(h.Script) > 0 {
			s = append(s, h.Script...)
		}

		return s
	})

	if err := b.writeCloneFetchCmds(w, info); err != nil {
		return err
	}

	if err := b.writeSubmoduleUpdateCmds(w, info); err != nil {
		return err
	}

	b.guardGetSourcesScriptHooks(w, info, "post_clone_script", func() []string {
		var s []string

		h := info.Build.Hooks.Get(spec.HookPostGetSourcesScript)
		if len(h.Script) > 0 {
			s = append(s, h.Script...)
		}

		if info.PostGetSourcesScript != "" {
			s = append(s, info.PostGetSourcesScript)
		}

		return s
	})

	b.changeFilesOwnership(w, info, info.Build.RootDir)

	return nil
}

func (b *AbstractShell) writeClearWorktreeScript(_ context.Context, w ShellWriter, info common.ShellScriptInfo) error {
	// Sometimes repos can get into a state where `git clean` isn't enough. A simple
	// example is if you have an untracked file in an uninitialised submodule.
	// In this case `git rm -rf .` will delete the entire submodule, allowing
	// a subsequent fetch to succeed.
	w.Noticef("Deleting tracked and untracked files...")

	projectDir := info.Build.FullProjectDir()

	w.IfDirectory(projectDir)
	w.Cd(projectDir)
	w.Command("git", "rm", "-rf", "--ignore-unmatch", ".")
	w.Command("git", "clean", "-ffdx")
	w.EndIf()

	return nil
}

func (b *AbstractShell) guardGetSourcesScriptHooks(
	w ShellWriter,
	info common.ShellScriptInfo,
	prefix string,
	script func() []string,
) {
	s := script()
	if len(s) == 0 || info.Build.GetGitStrategy() == common.GitNone || info.Build.GetGitStrategy() == common.GitEmpty {
		return
	}

	b.writeCommands(w, info, prefix, s...)
}

func (b *AbstractShell) writeExports(w ShellWriter, info common.ShellScriptInfo) {
	for _, variable := range info.Build.GetAllVariables() {
		w.Variable(variable)
	}

	gitlabEnvFile := w.TmpFile(gitlabEnvFileName)

	w.Variable(spec.Variable{
		Key:   "GITLAB_ENV",
		Value: gitlabEnvFile,
	})

	w.SourceEnv(gitlabEnvFile)
}

func (b *AbstractShell) writeCacheExports(w ShellWriter, variables map[string]string) string {
	return w.DotEnvVariables(gitlabCacheEnvFileName, variables)
}

func (b *AbstractShell) writeGitSSLConfig(w ShellWriter, build *common.Build, where []string) {
	host, err := b.getRemoteHost(build)
	if err != nil {
		w.Warningf("git SSL config: Can't get repository host. %w", err)
		return
	}

	variables := build.GetCITLSVariables()
	args := append([]string{"config"}, where...)

	for variable, config := range map[string]string{
		tls.VariableCAFile:   "sslCAInfo",
		tls.VariableCertFile: "sslCert",
		tls.VariableKeyFile:  "sslKey",
	} {
		if variables.Get(variable) == "" {
			continue
		}

		key := fmt.Sprintf("http.%s.%s", host, config)
		w.CommandArgExpand("git", append(args, key, w.EnvVariableKey(variable))...)
	}
}

// getRemoteHost gets the remote URL of the build, but removes the path and auth data; Thus leaving us with only the
// host name and scheme.
func (b *AbstractShell) getRemoteHost(build *common.Build) (string, error) {
	remoteURL, err := build.GetRemoteURL()
	if err != nil {
		return "", fmt.Errorf("getting remote URL: %w", err)
	}

	return url_helpers.OnlySchemeAndHost(remoteURL).String(), nil
}

func (b *AbstractShell) writeCloneFetchCmds(w ShellWriter, info common.ShellScriptInfo) error {
	build := info.Build

	// If LFS smudging was disabled by the user (by setting the GIT_LFS_SKIP_SMUDGE variable
	// when defining the job) we're skipping this step.
	//
	// In other case we're disabling smudging here to prevent us from memory
	// allocation failures.
	//
	// Please read https://gitlab.com/gitlab-org/gitlab-runner/issues/3366 and
	// https://github.com/git-lfs/git-lfs/issues/3524 for context.
	if !build.IsLFSSmudgeDisabled() {
		w.Variable(spec.Variable{Key: "GIT_LFS_SKIP_SMUDGE", Value: "1"})
	}

	err := b.handleGetSourcesStrategy(w, info)
	if err != nil {
		return err
	}

	if build.GetGitCheckout() {
		b.writeCheckoutCmd(w, build)

		// If LFS smudging was disabled by the user (by setting the GIT_LFS_SKIP_SMUDGE variable
		// when defining the job) we're skipping this step.
		//
		// In other case, because we've disabled LFS smudging above, we need now manually call
		// `git lfs pull` to fetch and checkout all LFS objects that may be present in
		// the repository.
		//
		// Repositories without LFS objects (and without any LFS metadata) will be not
		// affected by this command.
		//
		// Please read https://gitlab.com/gitlab-org/gitlab-runner/issues/3366 and
		// https://github.com/git-lfs/git-lfs/issues/3524 for context.
		if !build.IsLFSSmudgeDisabled() {
			w.IfCmd("git", "lfs", "version")
			w.Command("git", "lfs", "pull")
			w.EmptyLine()
			w.EndIf()
		}
	} else {
		w.Noticef("Skipping Git checkout")
	}

	return nil
}

func (b *AbstractShell) changeFilesOwnership(w ShellWriter, info common.ShellScriptInfo, dir ...string) {
	// The shell is not set the same way depending on the unit test
	// Some unit tests use info->Build->Runner->Shell while other use info->Shell
	shellName := info.Shell
	if shellName == "" {
		// GetDefaultShell will panic, if it can't find a default shell
		shellName = info.Build.Runner.Shell
	}

	// umask 0000 disabling is only support for UNIX-Like shells
	// We therefore don't do anything for PowerShell/pwsh
	if slices.Contains([]string{SNPowershell, SNPwsh}, shellName) {
		return
	}

	// ensure all parts are quoted with single quotes, so that whitespaces
	// and all don't trip us up and to ensure no unwanted variable expansion is happening
	unquotedUidGidFile := fmt.Sprintf(`%s/%s`, info.Build.RootDir, BuildUidGidFile)
	quotedUidGidFile := fmt.Sprintf(`'%s'`, unquotedUidGidFile)

	// unquotedUidGidFile file is only created when FF_DISABLE_UMASK_FOR_KUBERNETES_EXECUTOR is enabled
	w.IfFile(unquotedUidGidFile) // IfFIle does use `%q` internally
	for _, d := range dir {
		if strings.TrimSpace(d) == "" {
			continue
		}
		w.IfDirectory(d)
		w.Line(fmt.Sprintf(`chown -R "$(stat -c '%%u:%%g' %s)" '%s'`, quotedUidGidFile, d))
		if info.Build.IsDebugModeEnabled() {
			w.Line(fmt.Sprintf(`echo "Setting ownership for %s to $(stat -c '%%u:%%g' %s)"`, d, quotedUidGidFile))
		}
		w.EndIf()
	}
	w.EndIf()
}

func (b *AbstractShell) handleGetSourcesStrategy(w ShellWriter, info common.ShellScriptInfo) error {
	build := info.Build
	projectDir := build.FullProjectDir()

	switch strategy := build.GetGitStrategy(); strategy {
	case common.GitFetch, common.GitClone:
		b.writeGitCleanup(w, build)

		templateDir, remoteURL, err := b.setupTemplateDir(w, build, projectDir)
		if err != nil {
			return err
		}

		getCmdWriter := b.writeRefspecFetchCmd
		if strategy == common.GitClone {
			getCmdWriter = b.writeCloneCmdIfPossible
		}

		getCmdWriter(w, info, templateDir, remoteURL)
		return nil
	case common.GitNone:
		w.Noticef("Skipping Git repository setup")
		w.MkDir(projectDir)
		return nil
	case common.GitEmpty:
		w.Noticef("Skipping Git repository setup and creating an empty build directory")
		w.RmDir(projectDir)
		w.MkDir(projectDir)
		return nil
	default:
		return errUnknownGitStrategy
	}
}

// deduplicateInsteadOfs removes duplicate insteadOf entries, keeping the first occurrence.
// This prevents redundant git config rules when the same URL rewrite appears multiple times.
func deduplicateInsteadOfs(insteadOfs [][2]string) [][2]string {
	seen := make(map[[2]string]bool)
	result := make([][2]string, 0, len(insteadOfs))
	for _, io := range insteadOfs {
		if !seen[io] {
			seen[io] = true
			result = append(result, io)
		}
	}
	return result
}

// setupExternalGitConfig sets up a git config file, holding the externalized config for the git repo.
// This file is meant to be "included" (either via CLI flag or via include from another git config).
// It holds some configuration specific to the main repo.
// It holds insteadOf stanzas, which allows git to replace a repo URL without credentials by one with credentials, on
// the fly. With this we can avoid dumping creds into the build directory.
// It also has the configuration for the git credential helper, if enabled.
func (b *AbstractShell) setupExternalGitConfig(w ShellWriter, build *common.Build, extConfigFile string) (remoteURL string, err error) {
	w.RmFile(extConfigFile)

	if build.IsFeatureFlagOn(featureflags.UseGitalyCorrelationId) {
		w.CommandArgExpand("git", "config", "-f", extConfigFile, "http.extraHeader", "X-Gitaly-Correlation-ID: "+build.JobRequestCorrelationID)
		w.Noticef("Gitaly correlation ID: %s", build.JobRequestCorrelationID)
	}

	if build.IsFeatureFlagOn(featureflags.UseGitBundleURIs) {
		w.CommandArgExpand("git", "config", "-f", extConfigFile, "transfer.bundleURI", "true")
	}

	if build.IsSharedEnv() {
		b.writeGitSSLConfig(w, build, []string{"-f", extConfigFile})
	}

	urlWithCreds, err := build.GetRemoteURL()
	if err != nil {
		return "", err
	}

	urlWithoutCreds := *urlWithCreds
	urlWithoutCreds.User = nil

	withCreds, withoutCreds := urlWithCreds.String(), urlWithoutCreds.String()
	insteadOfs := [][2]string{}

	if withoutCreds != withCreds {
		insteadOfs = append(insteadOfs, [2]string{withCreds, withoutCreds})
	}

	if ios, err := build.GetInsteadOfs(); err != nil {
		return "", err
	} else {
		insteadOfs = append(insteadOfs, ios...)
	}

	// De-duplicate insteadOfs entries to avoid redundant git config rules
	insteadOfs = deduplicateInsteadOfs(insteadOfs)

	for _, io := range insteadOfs {
		replaceStanza := "url." + io[0] + ".insteadOf"
		orgURL := io[1]
		pattern := "^" + regexp.QuoteMeta(orgURL) + "$"
		w.CommandArgExpand("git", "config", "--file", extConfigFile, "--replace-all", replaceStanza, orgURL, pattern)
	}

	if build.IsFeatureFlagOn(featureflags.GitURLsWithoutTokens) {
		remoteHost, err := b.getRemoteHost(build)
		if err != nil {
			return "", err
		}
		w.SetupGitCredHelper(extConfigFile, "credential."+remoteHost, "gitlab-ci-token")
	}

	return withoutCreds, nil
}

//nolint:funlen
func (b *AbstractShell) writeRefspecFetchCmd(w ShellWriter, info common.ShellScriptInfo, templateDir, remoteURL string) {
	build := info.Build
	projectDir := build.FullProjectDir()
	depth := build.GitInfo.Depth

	if depth > 0 {
		w.Noticef("Fetching changes with git depth set to %d...", depth)
	} else {
		w.Noticef("Fetching changes...")
	}

	objectFormat := build.GetRepositoryObjectFormat()

	if objectFormat != common.DefaultObjectFormat {
		w.Command("git", "init", projectDir, "--template", templateDir, "--object-format", objectFormat)
	} else {
		w.Command("git", "init", projectDir, "--template", templateDir)
	}

	w.Cd(projectDir)

	// Add `git remote` or update existing
	w.IfCmd("git", "remote", "add", "origin", remoteURL)
	w.Noticef("Created fresh repository.")
	w.Else()
	w.Command("git", "remote", "set-url", "origin", remoteURL)
	// For existing repositories, the template isn't reapplied, so we need to explicitly
	// configure the repository to use the external git config
	b.setupExistingRepoConfig(w)
	w.EndIf()

	v := common.AppVersion
	userAgent := fmt.Sprintf("http.userAgent=%s %s %s/%s", v.Name, v.Version, v.OS, v.Architecture)

	fetchArgs := []string{"-c", userAgent, "fetch", "origin", "--no-recurse-submodules"}
	fetchArgs = append(fetchArgs, build.GitInfo.Refspecs...)
	if depth > 0 {
		fetchArgs = append(fetchArgs, "--depth", strconv.Itoa(depth))
	}

	fetchArgs = append(fetchArgs, build.GetGitFetchFlags()...) //nolint:gocritic

	if depth <= 0 {
		fetchUnshallowArgs := append(fetchArgs, "--unshallow") //nolint:gocritic

		w.IfFile(".git/shallow")
		w.Command("git", fetchUnshallowArgs...)
		w.Else()
		w.Command("git", fetchArgs...)
		w.EndIf()
	} else {
		w.Command("git", fetchArgs...)
	}
}

func (b *AbstractShell) writeCloneCmdIfPossible(w ShellWriter, info common.ShellScriptInfo, templateDir, remoteURL string) {
	build := info.Build
	projectDir := build.FullProjectDir()

	// always ensure the old clone is gone
	w.RmDir(projectDir)

	if !build.IsFeatureFlagOn(featureflags.UseGitNativeClone) {
		b.writeRefspecFetchCmd(w, info, templateDir, remoteURL)
		return
	}

	w.IfGitVersionIsAtLeast(gitMinVersionCloneWithRef)
	b.writeCloneRevisionCmd(w, info, templateDir, remoteURL)
	w.Else()
	b.writeRefspecFetchCmd(w, info, templateDir, remoteURL)
	w.EndIf()
}

func (b *AbstractShell) writeCloneRevisionCmd(w ShellWriter, info common.ShellScriptInfo, templateDir, remoteURL string) {
	build := info.Build
	projectDir := build.FullProjectDir()
	depth := build.GitInfo.Depth

	switch {
	case depth > 0:
		w.Noticef("Cloning repository for %s with git depth set to %d...", build.GitInfo.Ref, depth)
	case build.GitInfo.Ref != "":
		w.Noticef("Cloning repository for %s...", build.GitInfo.Ref)
	default:
		w.Noticef("Cloning repository...")
	}

	v := common.AppVersion
	userAgent := fmt.Sprintf("http.userAgent=%s %s %s/%s", v.Name, v.Version, v.OS, v.Architecture)

	cloneArgs := []string{"-c", userAgent, "clone", "--no-checkout", remoteURL, projectDir, "--template", templateDir}

	if depth > 0 {
		cloneArgs = append(cloneArgs, "--depth", strconv.Itoa(depth))
	}

	if strings.HasPrefix(build.GitInfo.Ref, "refs/") {
		cloneArgs = append(cloneArgs, "--revision", build.GitInfo.Ref)
	} else if build.GitInfo.Ref != "" {
		cloneArgs = append(cloneArgs, "--branch", build.GitInfo.Ref)
	}

	cloneArgs = append(cloneArgs, build.GetGitCloneFlags()...)

	w.Command("git", cloneArgs...)

	w.Cd(projectDir)
}

// includeExternalGitConfig runs the git config command, to add a include.path setting to targetFile to include fileToInclude.
// The path to fileToInclude must be absolute, so that it does not matter where git is eventually called from.
// It will replace all existing include.path settings which point to the same base name. This ensures that we only have
// one include to the same file, do not mess with user-/pre-defined includes, and support the buildsDir to change.
// It also sets a env variable with the **absolute** path to the included file for later use.
func includeExternalGitConfig(w ShellWriter, targetFile, fileToInclude string) {
	baseName := path.Base(helpers.ToSlash(fileToInclude))
	pattern := regexp.QuoteMeta(baseName) + "$"
	w.CommandArgExpand("git", "config", "--file", targetFile, "--replace-all", "include.path", fileToInclude, pattern)
	w.ExportRaw(envVarExternalGitConfigFile, fileToInclude)
}

// setupExistingRepoConfig configures an existing Git repository to use the external Git config.
// This is needed because git init with a template doesn't re-apply the template to existing repositories.
// Note: This function relies on the GLR_EXT_GIT_CONFIG_PATH environment variable being set by setupTemplateDir().
func (b *AbstractShell) setupExistingRepoConfig(w ShellWriter) {
	// We're already in projectDir (after cd), so .git/config is relative to current directory
	gitConfigFile := w.Join(gitDir, "config")
	// Use the environment variable that was set in setupTemplateDir() to get the absolute path.
	// This ensures the path is correct even when BuildDir is relative and we've already changed directories.
	extConfigFile := w.EnvVariableKey(envVarExternalGitConfigFile)
	baseName := path.Base(helpers.ToSlash(externalGitConfigFile))
	pattern := regexp.QuoteMeta(baseName) + "$"
	w.CommandArgExpand("git", "config", "--file", gitConfigFile, "--replace-all", "include.path", extConfigFile, pattern)
}

func (b *AbstractShell) setupTemplateDir(w ShellWriter, build *common.Build, projectDir string) (string, string, error) {
	templateDir := w.MkTmpDir(gitTemplateDir)
	templateFile := w.Join(templateDir, "config")
	extConfigFile := w.TmpFile(externalGitConfigFile)

	if build.SafeDirectoryCheckout {
		// Solves problem with newer Git versions when files existing in the working directory
		// are owned by different system owners. This may happen for example with Docker executor,
		// a root-less image used in previous job and the working directory being persisted between
		// jobs. More details can be found at https://gitlab.com/gitlab-org/gitlab/-/issues/368133.
		w.Command("git", "config", "--global", "--add", "safe.directory", projectDir)
	}

	w.Command("git", "config", "-f", templateFile, "init.defaultBranch", "none")
	w.Command("git", "config", "-f", templateFile, "fetch.recurseSubmodules", "false")
	w.Command("git", "config", "-f", templateFile, "credential.interactive", "never")
	w.Command("git", "config", "-f", templateFile, "gc.autoDetach", "false")

	// for externalized configs & creds
	remoteURL, err := b.setupExternalGitConfig(w, build, extConfigFile)
	if err != nil {
		return "", "", fmt.Errorf("setting up external git config: %w", err)
	}

	// Note: We need to ensure that credConfigFile is rendered into the template with the absolute path!
	includeExternalGitConfig(w, templateFile, extConfigFile)

	return templateDir, remoteURL, nil
}

func (b *AbstractShell) writeGitCleanup(w ShellWriter, build *common.Build) {
	projectDir := build.FullProjectDir()
	submoduleStrategy := build.GetSubmoduleStrategy()
	cleanForSubmodules := submoduleStrategy == common.SubmoduleNormal || submoduleStrategy == common.SubmoduleRecursive

	// Remove .git/{index,shallow,HEAD,config}.lock files from .git, which can fail the fetch command
	// The file can be left if previous build was terminated during git operation.
	// If the git submodule strategy is defined as normal or recursive, also remove these files
	// inside .git/modules/**/
	files := []string{
		"index.lock",
		"shallow.lock",
		"HEAD.lock",
		"hooks/post-checkout",
		"config.lock",
	}

	for _, f := range files {
		w.RmFile(path.Join(projectDir, gitDir, f))
		if cleanForSubmodules {
			w.RmFilesRecursive(path.Join(projectDir, gitDir, "modules"), path.Base(f))
		}
	}

	w.RmFilesRecursive(path.Join(projectDir, gitDir, "refs"), "*.lock")

	b.writeGitCleanupAllConfigs(w, build, cleanForSubmodules)
}

// writeGitCleanupAllConfigs removes all git configs which are potentially open to malicious code injection:
// - the main git config & hooks
// - the template git config & hooks
// - any submodule's git config & hooks
// It's by default disabled for the shell executor or when the git strategy is "none", and enabled otherwise; explicit
// configuration however always has precedence.
func (b *AbstractShell) writeGitCleanupAllConfigs(sw ShellWriter, build *common.Build, cleanForSubmodules bool) {
	executor := build.Runner.Executor
	shouldCleanUp := (executor != "shell" && executor != "shell-integration-test" && build.GetGitStrategy() != common.GitNone)
	if config := build.Runner.CleanGitConfig; config != nil {
		shouldCleanUp = *config
	}
	if !shouldCleanUp {
		return
	}

	projectDir := build.FullProjectDir()

	// clean out configs in the main git dir and in the template dir
	for _, dir := range []string{sw.TmpFile(gitTemplateDir), sw.Join(projectDir, gitDir)} {
		sw.RmFile(sw.Join(dir, "config"))
		sw.RmDir(sw.Join(dir, "hooks"))
	}

	// clean out configs in the modules' git dirs
	if cleanForSubmodules {
		modulesDir := sw.Join(projectDir, gitDir, "modules")
		sw.RmFilesRecursive(modulesDir, "config")
		sw.RmDirsRecursive(modulesDir, "hooks")
	}
}

func (b *AbstractShell) writeCheckoutCmd(w ShellWriter, build *common.Build) {
	w.Noticef("Checking out %s as detached HEAD (ref is %s)...", build.GitInfo.Sha[0:8], build.GitInfo.Ref)
	w.Command("git", "-c", "submodule.recurse=false", "checkout", "-f", "-q", build.GitInfo.Sha)

	cleanFlags := build.GetGitCleanFlags()
	if len(cleanFlags) > 0 {
		cleanArgs := append([]string{"clean"}, cleanFlags...)
		w.Command("git", cleanArgs...)
	}
}

func (b *AbstractShell) writeSubmoduleUpdateCmds(w ShellWriter, info common.ShellScriptInfo) error {
	build := info.Build

	switch build.GetSubmoduleStrategy() {
	case common.SubmoduleNormal:
		return b.writeSubmoduleUpdateCmd(w, build, false)

	case common.SubmoduleRecursive:
		return b.writeSubmoduleUpdateCmd(w, build, true)

	case common.SubmoduleNone:
		w.Noticef("Skipping Git submodules setup")

	default:
		return errors.New("unknown GIT_SUBMODULE_STRATEGY")
	}

	return nil
}

func (b *AbstractShell) writeSubmoduleUpdateCmd(w ShellWriter, build *common.Build, recursive bool) error {
	depth := build.GetSubmoduleDepth()

	b.writeSubmoduleUpdateNoticeMsg(w, recursive, depth)

	var pathArgs []string

	submodulePaths, err := build.GetSubmodulePaths()
	if err != nil {
		return err
	}

	if len(submodulePaths) != 0 {
		pathArgs = append(pathArgs, "--")
		pathArgs = append(pathArgs, submodulePaths...)
	}

	// Init submodules must occur prior to sync to ensure completeness of .git/config
	w.Command("git", "submodule", "init")

	// Sync .git/config to .gitmodules in case URL changes (e.g. new build token)
	syncArgs := []string{"submodule", "sync"}
	if recursive {
		syncArgs = append(syncArgs, "--recursive")
	}
	syncArgs = append(syncArgs, pathArgs...)
	w.Command("git", syncArgs...)

	updateArgs := []string{"submodule", "update", "--init"}
	foreachArgs := []string{"submodule", "foreach"}
	if recursive {
		updateArgs = append(updateArgs, "--recursive")
		foreachArgs = append(foreachArgs, "--recursive")
	}
	if depth > 0 {
		updateArgs = append(updateArgs, "--depth", strconv.Itoa(depth))
	}
	submoduleUpdateFlags := build.GetGitSubmoduleUpdateFlags()
	updateArgs = append(updateArgs, submoduleUpdateFlags...)
	updateArgs = append(updateArgs, pathArgs...)

	// Clean changed files in submodules
	cleanFlags := []string{"-ffdx"}
	if len(build.GetGitCleanFlags()) > 0 {
		cleanFlags = build.GetGitCleanFlags()
	}
	cleanCommand := []string{"git clean " + strings.Join(cleanFlags, " ")}

	w.Command("git", append(foreachArgs, cleanCommand...)...)
	w.Command("git", append(foreachArgs, "git reset --hard")...)

	// Some submodule operations need creds configured, but don't pick up config from the main repo. For those, we
	// explicitly "include.path" the externalized git config. For the "include.path" value, we use an env var, thus we
	// need to ensure that those commands run with arg expansion.
	withExplicitSubmoduleCreds := func(orgArgs []string) []string {
		return slices.Concat(
			[]string{"-c", "include.path=" + w.EnvVariableKey(envVarExternalGitConfigFile)},
			orgArgs,
		)
	}

	w.IfCmdWithOutputArgExpand("git", withExplicitSubmoduleCreds(updateArgs)...)
	w.Noticef("Updated submodules")
	w.Command("git", syncArgs...)
	w.Else()
	// call sync and update again if the initial update fails
	w.Warningf("Updating submodules failed. Retrying...")

	hasSubmoduleRemoteFlag := slices.ContainsFunc(submoduleUpdateFlags, func(s string) bool {
		return strings.EqualFold(s, "--remote")
	})
	if hasSubmoduleRemoteFlag {
		// We've observed issues like
		//	fatal: Unable to find refs/remotes/origin/dev revision in submodule path 'subs-1'
		// when updating submodule with `--remote` and `branch` was set in `.gitmodules` (which is not the default branch). To
		// work around that, we explicitly pull in the remote heads.
		// We only do this as a fallback / on retry *and* when the `--remote` update flag is used, so that we don't
		// unnecessarily pull in a ton of remote heads.
		// This renders a command similar to:
		//	git submodule foreach 'git fetch origin +refs/heads/*:refs/remotes/origin/*'
		w.CommandArgExpand("git", withExplicitSubmoduleCreds(slices.Concat(
			foreachArgs, []string{"git fetch origin +refs/heads/*:refs/remotes/origin/*"},
		))...)
	}

	w.Command("git", syncArgs...)
	w.CommandArgExpand("git", withExplicitSubmoduleCreds(updateArgs)...)
	w.Command("git", append(foreachArgs, "git reset --hard")...)
	w.EndIf()

	w.Command("git", append(foreachArgs, cleanCommand...)...)

	// Configure each submodule to include the external git config.
	// This allows git operations inside submodule directories (e.g., cd patches && git pull)
	// to authenticate properly using the parent repo's credentials.
	// This is done once at the end, after all submodule operations are complete.
	// See: https://gitlab.com/gitlab-org/gitlab-runner/-/issues/39133
	w.Noticef("Configuring submodules to use parent git credentials...")
	b.configureSubmoduleCredentials(w, foreachArgs, recursive)

	if !build.IsLFSSmudgeDisabled() {
		w.IfCmd("git", "lfs", "version")
		w.Noticef("Pulling LFS files...")
		w.CommandArgExpand("git", withExplicitSubmoduleCreds(append(foreachArgs, "git lfs pull"))...)
		w.EndIf()
	}

	return nil
}

func (b *AbstractShell) writeSubmoduleUpdateNoticeMsg(w ShellWriter, recursive bool, depth int) {
	switch {
	case recursive && depth > 0:
		w.Noticef("Updating/initializing submodules recursively with git depth set to %d...", depth)
	case recursive && depth == 0:
		w.Noticef("Updating/initializing submodules recursively...")
	case depth > 0:
		w.Noticef("Updating/initializing submodules with git depth set to %d...", depth)
	default:
		w.Noticef("Updating/initializing submodules...")
	}
}

// configureSubmoduleCredentials configures each submodule to include the external git config
// from the parent repository. This allows git operations inside submodule directories
// (e.g., cd patches && git pull) to authenticate properly using the parent repo's credentials.
func (b *AbstractShell) configureSubmoduleCredentials(w ShellWriter, foreachArgs []string, recursive bool) {
	// Use the GLR_EXT_GIT_CONFIG_PATH environment variable that was set earlier.
	// We need to quote the variable expansion to handle paths with spaces.
	cmd := fmt.Sprintf(`git config --replace-all include.path '%s'`, w.EnvVariableKey(envVarExternalGitConfigFile))
	args := foreachArgs
	// Even if `GIT_SUBMODULE_STRATEGY: normal` is used, we should set up the credentials
	// for all the Git submodules to preserve existing workflows.
	if !recursive {
		args = append(args, "--recursive")
	}
	args = append(args, cmd)
	w.CommandArgExpand("git", args...)
}

func (b *AbstractShell) writeRestoreCacheScript(
	ctx context.Context,
	w ShellWriter,
	info common.ShellScriptInfo,
) error {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	// Try to restore from main cache, if not found cache for default branch
	return b.cacheExtractor(ctx, w, info)
}

func (b *AbstractShell) writeDownloadArtifactsScript(
	_ context.Context,
	w ShellWriter,
	info common.ShellScriptInfo,
) error {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	return b.downloadAllArtifacts(w, info)
}

// Write the given string of commands using the provided ShellWriter object.
func (b *AbstractShell) writeCommands(w ShellWriter, info common.ShellScriptInfo, prefix string, commands ...string) {
	writeCommand := func(i int, command string) {
		command = strings.TrimSpace(command)
		defer func() {
			w.Line(command)
			w.CheckForErrors()
		}()

		if command == "" {
			w.EmptyLine()
			return
		}

		nlIndex := strings.Index(command, "\n")
		if nlIndex == -1 {
			w.Noticef("$ %s", command)
			return
		}

		if info.Build.IsFeatureFlagOn(featureflags.ScriptSections) &&
			info.Build.Job.Features.TraceSections {
			b.writeMultilineCommand(w, fmt.Sprintf("%s_%d", prefix, i), command)
		} else {
			w.Noticef("$ %s # collapsed multi-line command", command[:nlIndex])
		}
	}

	for i, command := range commands {
		writeCommand(i, command)
	}
}

func stringifySectionOptions(opts []string) string {
	if len(opts) == 0 {
		return ""
	}
	return fmt.Sprintf("[%s]", strings.Join(opts, ","))
}

func (b *AbstractShell) writeMultilineCommand(w ShellWriter, sectionName, command string) {
	w.SectionStart(sectionName, fmt.Sprintf("$ %s", command), []string{"hide_duration=true", "collapsed=true"})
	w.SectionEnd(sectionName)
}

func (b *AbstractShell) writeUserScript(
	w ShellWriter,
	info common.ShellScriptInfo,
	buildStage common.BuildStage,
) error {
	var scriptStep *spec.Step
	for _, step := range info.Build.Steps {
		if common.StepToBuildStage(step) == buildStage {
			scriptStep = &step
			break
		}
	}

	if scriptStep == nil {
		return common.ErrSkipBuildStage
	}

	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	if info.PreBuildScript != "" {
		b.writeCommands(w, info, "pre_build_script", info.PreBuildScript)
	}

	script := scriptStep.Script
	// handles the release yaml field that gets converted to a step by the backend
	if scriptStep.Name == "release" {
		for i, s := range scriptStep.Script {
			script[i] = info.Build.GetAllVariables().ExpandValue(s)
		}
	}

	b.writeCommands(w, info, "script_step", script...)

	if info.PostBuildScript != "" {
		b.writeCommands(w, info, "post_build_script", info.PostBuildScript)
	}

	return nil
}

func (b *AbstractShell) cacheArchiver(
	ctx context.Context,
	w ShellWriter,
	info common.ShellScriptInfo,
	onSuccess bool,
) error {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	skipArchiveCache, err := b.archiveCache(ctx, w, info, onSuccess)
	if err != nil {
		return err
	}

	if skipArchiveCache {
		return common.ErrSkipBuildStage
	}

	return nil
}

func (b *AbstractShell) archiveCache(
	ctx context.Context,
	w ShellWriter,
	info common.ShellScriptInfo,
	onSuccess bool,
) (bool, error) {
	skipArchiveCache := true

	for _, cacheOptions := range info.Build.Cache {
		if !cacheOptions.When.ShouldCache(onSuccess) {
			continue
		}

		// Create list of files to archive
		archiverArgs := b.getArchiverArgs(cacheOptions, info)

		if len(archiverArgs) < 1 {
			// Skip creating archive
			continue
		}

		skipArchiveCache = false

		cacheConfig, warning, err := newCacheConfig(info.Build, cacheOptions.Key)
		if warning != "" {
			w.Warningf(warning)
		}
		// Skip archiving if no cache is defined
		if err != nil {
			w.Noticef("Skipping cache archiving due to %v", err)
			continue
		}

		cacheOptions.Policy = spec.CachePolicy(info.Build.GetAllVariables().ExpandValue(string(cacheOptions.Policy)))

		if ok, err := cacheOptions.CheckPolicy(spec.CachePolicyPush); err != nil {
			return false, fmt.Errorf("%w for %s", err, cacheConfig.HumanKey)
		} else if !ok {
			w.Noticef("Not uploading cache %s due to policy", cacheConfig.HumanKey)
			continue
		}

		b.addCacheUploadCommand(ctx, w, info, *cacheConfig, archiverArgs)
	}

	return skipArchiveCache, nil
}

func (b *AbstractShell) getArchiverArgs(cacheOptions spec.Cache, _ common.ShellScriptInfo) []string {
	var archiverArgs []string
	for _, path := range cacheOptions.Paths {
		archiverArgs = append(archiverArgs, "--path", path)
	}

	if cacheOptions.Untracked {
		archiverArgs = append(archiverArgs, "--untracked")
	}

	return archiverArgs
}

func (b *AbstractShell) addCacheUploadCommand(
	ctx context.Context,
	w ShellWriter,
	info common.ShellScriptInfo,
	cacheConfig cacheConfig,
	archiverArgs []string,
) {
	// add metadata to the local metadata file and for GoCloud uploads
	args := []string{
		"cache-archiver",
		"--file", cacheConfig.ArchiveFile,
		"--timeout", strconv.Itoa(info.Build.GetCacheRequestTimeout()),
	}

	metadata := map[string]string{
		"cachekey": cacheConfig.HumanKey,
	}

	if info.Build.Runner.Cache != nil && info.Build.Runner.Cache.MaxUploadedArchiveSize > 0 {
		args = append(
			args,
			"--max-uploaded-archive-size",
			strconv.FormatInt(info.Build.Runner.Cache.MaxUploadedArchiveSize, 10),
		)
	}

	env := map[string]string{}

	// We pass the metadata via environment rather than via CLI flags, so that we are backwards compatible, e.g. for
	// user who have pinned the helper image / helper binary to an older version.
	// Note: Marshaling map[string]string wont error ever, thus we ignore the error here.
	metaJsonBlob, _ := json.Marshal(metadata)
	env["CACHE_METADATA"] = string(metaJsonBlob)

	args = append(args, archiverArgs...)

	// Generate cache upload address
	extraArgs, extraEnv, err := getCacheUploadURLAndEnv(ctx, info.Build, cacheConfig.HashedKey, metadata)
	args = append(args, extraArgs...)
	maps.Copy(env, extraEnv)

	if err != nil {
		w.Warningf("Unable to generate cache upload environment: %v", err)
	}

	// Execute cache-archiver command. Failure is not fatal.
	b.guardRunnerCommand(w, info.RunnerCommand, "Creating cache", func() {
		w.Noticef("Creating cache %s...", cacheConfig.HumanKey)

		if env != nil {
			cacheEnvFilename := b.writeCacheExports(w, env)
			args = append(args, "--env-file", cacheEnvFilename)
			defer w.RmFile(cacheEnvFilename)
		}

		w.IfCmdWithOutput(info.RunnerCommand, args...)
		w.Noticef("Created cache")
		w.Else()
		w.Warningf("Failed to create cache")
		w.EndIf()
	})
}

// getCacheUploadURLAndEnv will first try to generate the GoCloud URL if it's
// available then fallback to a pre-signed URL.
func getCacheUploadURLAndEnv(ctx context.Context, build *common.Build, cacheKey string, metadata map[string]string) ([]string, map[string]string, error) {
	adapter := cache.GetAdapter(build.Runner.Cache, build.GetBuildTimeout(), build.Runner.ShortDescription(), fmt.Sprintf("%d", build.JobInfo.ProjectID), cacheKey)

	// Prefer Go Cloud URL if supported
	goCloudURL, err := adapter.GetGoCloudURL(ctx, true)
	if goCloudURL.URL != nil {
		uploadArgs := []string{"--gocloud-url", goCloudURL.URL.String()}
		return uploadArgs, goCloudURL.Environment, err
	}

	adapter.WithMetadata(metadata)
	uploadURL := adapter.GetUploadURL(ctx)
	if uploadURL.URL == nil {
		return []string{}, nil, nil
	}

	uploadArgs := []string{"--url", uploadURL.URL.String()}
	for key, values := range uploadURL.Headers {
		for _, value := range values {
			uploadArgs = append(uploadArgs, "--header", fmt.Sprintf("%s: %s", key, value))
		}
	}

	return uploadArgs, nil, err
}

func (b *AbstractShell) writeUploadArtifact(w ShellWriter, info common.ShellScriptInfo, artifact spec.Artifact) bool {
	args := []string{
		"artifacts-uploader",
		"--url",
		info.Build.Runner.URL,
		"--token",
		info.Build.Token,
		"--id",
		strconv.FormatInt(info.Build.ID, 10),
	}

	if b.shouldGenerateArtifactsMetadata(info, artifact) {
		args = append(args, b.generateArtifactsMetadataArgs(info)...)
	}

	// Create list of files to archive
	var archiverArgs []string
	for _, path := range artifact.Paths {
		archiverArgs = append(archiverArgs, "--path", path)
	}

	// Create list of paths to be excluded from the archive
	for _, path := range artifact.Exclude {
		archiverArgs = append(archiverArgs, "--exclude", path)
	}

	if artifact.Untracked {
		archiverArgs = append(archiverArgs, "--untracked")
	}

	if len(archiverArgs) < 1 {
		// Skip creating archive
		return false
	}

	args = append(args, archiverArgs...)

	if artifact.Name != "" {
		args = append(args, "--name", artifact.Name)
	}

	if artifact.ExpireIn != "" {
		args = append(args, "--expire-in", artifact.ExpireIn)
	}

	if artifact.Format != "" {
		args = append(args, "--artifact-format", string(artifact.Format))
	}

	if artifact.Type != "" {
		args = append(args, "--artifact-type", artifact.Type)
	}

	b.guardRunnerCommand(w, info.RunnerCommand, "Uploading artifacts", func() {
		w.Noticef("Uploading artifacts...")
		w.Command(info.RunnerCommand, args...)
	})

	return true
}

func (b *AbstractShell) shouldGenerateArtifactsMetadata(info common.ShellScriptInfo, artifact spec.Artifact) bool {
	generateArtifactsMetadata := info.Build.Variables.Bool(common.GenerateArtifactsMetadataVariable)
	// Currently only zip artifacts are supported as artifact metadata effectively adds another file to the archive
	// https://gitlab.com/gitlab-org/gitlab/-/issues/367203#note_1059841610
	metadataArtifactsFormatSupported := artifact.Format == spec.ArtifactFormatZip
	return generateArtifactsMetadata && metadataArtifactsFormatSupported
}

func (b *AbstractShell) generateArtifactsMetadataArgs(info common.ShellScriptInfo) []string {
	schemaVersion := info.Build.Variables.Get("SLSA_PROVENANCE_SCHEMA_VERSION")
	if schemaVersion == "" {
		// specify a value so the CLI can parse the arguments correctly
		// avoid specifying a proper default here to avoid duplication
		// the artifact metadata command will handle that separately
		schemaVersion = "unknown"
	}

	args := []string{
		"--generate-artifacts-metadata",
		"--runner-id",
		info.Build.Variables.Value("CI_RUNNER_ID"),
		"--repo-url",
		strings.TrimSuffix(info.Build.RepoCleanURL(), ".git"),
		"--repo-digest",
		info.Build.GitInfo.Sha,
		"--job-name",
		info.Build.JobInfo.Name,
		"--executor-name",
		info.Build.Runner.Executor,
		"--runner-name",
		info.Build.Runner.Name,
		"--started-at",
		info.Build.StartedAt().Format(time.RFC3339),
		"--ended-at",
		time.Now().Format(time.RFC3339),
		"--schema-version",
		schemaVersion,
	}

	for _, variable := range info.Build.Variables {
		args = append(args, "--metadata-parameter", variable.Key)
	}

	return args
}

func (b *AbstractShell) writeUploadArtifacts(w ShellWriter, info common.ShellScriptInfo, onSuccess bool) error {
	if info.Build.Runner.URL == "" {
		return common.ErrSkipBuildStage
	}

	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	skipUploadArtifacts := true

	for _, artifact := range info.Build.Artifacts {
		if onSuccess && !artifact.When.OnSuccess() {
			continue
		}
		if !onSuccess && !artifact.When.OnFailure() {
			continue
		}

		if b.writeUploadArtifact(w, info, artifact) {
			skipUploadArtifacts = false
		}
	}

	if skipUploadArtifacts {
		return common.ErrSkipBuildStage
	}

	return nil
}

func (b *AbstractShell) writeAfterScript(_ context.Context, w ShellWriter, info common.ShellScriptInfo) error {
	var afterScriptStep *spec.Step
	for _, step := range info.Build.Steps {
		if step.Name == spec.StepNameAfterScript {
			afterScriptStep = &step
			break
		}
	}

	if afterScriptStep == nil || len(afterScriptStep.Script) == 0 {
		return common.ErrSkipBuildStage
	}

	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	w.Noticef("Running after script...")

	b.writeCommands(w, info, "after_script_step", afterScriptStep.Script...)

	return nil
}

func (b *AbstractShell) writeUploadArtifactsOnSuccessScript(
	_ context.Context,
	w ShellWriter,
	info common.ShellScriptInfo,
) error {
	return b.writeUploadArtifacts(w, info, true)
}

func (b *AbstractShell) writeUploadArtifactsOnFailureScript(
	_ context.Context,
	w ShellWriter,
	info common.ShellScriptInfo,
) error {
	return b.writeUploadArtifacts(w, info, false)
}

func (b *AbstractShell) writeArchiveCacheOnSuccessScript(
	ctx context.Context,
	w ShellWriter,
	info common.ShellScriptInfo,
) error {
	return b.cacheArchiver(ctx, w, info, true)
}

func (b *AbstractShell) writeArchiveCacheOnFailureScript(
	ctx context.Context,
	w ShellWriter,
	info common.ShellScriptInfo,
) error {
	return b.cacheArchiver(ctx, w, info, false)
}

func (b *AbstractShell) writeCleanupScript(_ context.Context, w ShellWriter, info common.ShellScriptInfo) error {
	w.RmFile(w.TmpFile(gitlabEnvFileName))
	w.RmFile(w.TmpFile("masking.db"))
	w.RmFile(w.TmpFile(externalGitConfigFile))

	for _, variable := range info.Build.GetAllVariables() {
		if !variable.File {
			continue
		}
		w.RmFile(w.TmpFile(variable.Key))
	}

	if info.Build.IsFeatureFlagOn(featureflags.EnableJobCleanup) {
		if err := b.writeCleanupBuildDirectoryScript(w, info); err != nil {
			return err
		}
	}

	b.writeGitCleanup(w, info.Build)

	w.RmFile(w.Join(info.Build.RootDir, BuildUidGidFile))

	return nil
}

func (b *AbstractShell) writeCleanupBuildDirectoryScript(w ShellWriter, info common.ShellScriptInfo) error {
	switch info.Build.GetGitStrategy() {
	case common.GitClone, common.GitEmpty:
		w.RmDir(info.Build.FullProjectDir())
	case common.GitFetch:
		b.writeCdBuildDir(w, info)

		var cleanArgs []string
		cleanFlags := info.Build.GetGitCleanFlags()
		if len(cleanFlags) > 0 {
			cleanArgs = append([]string{"clean"}, cleanFlags...)
			w.Command("git", cleanArgs...)
		}

		resetArgs := []string{"reset", "--hard"}
		w.Command("git", resetArgs...)

		if info.Build.GetSubmoduleStrategy() == common.SubmoduleNormal ||
			info.Build.GetSubmoduleStrategy() == common.SubmoduleRecursive {
			submoduleArgs := []string{"submodule", "foreach"}

			if info.Build.GetSubmoduleStrategy() == common.SubmoduleRecursive {
				submoduleArgs = append(submoduleArgs, "--recursive")
			}

			if len(cleanFlags) > 0 {
				submoduleCleanArgs := append(submoduleArgs, append([]string{"git"}, cleanArgs...)...) //nolint:gocritic
				w.Command("git", submoduleCleanArgs...)
			}

			submoduleResetArgs := append(submoduleArgs, append([]string{"git"}, resetArgs...)...) //nolint:gocritic
			w.Command("git", submoduleResetArgs...)
		}
	case common.GitNone:
		w.Noticef("Skipping build directory cleanup step")

	default:
		return errUnknownGitStrategy
	}

	return nil
}

func (b *AbstractShell) writeScript(
	ctx context.Context,
	w ShellWriter,
	buildStage common.BuildStage,
	info common.ShellScriptInfo,
) error {
	methods := map[common.BuildStage]func(context.Context, ShellWriter, common.ShellScriptInfo) error{
		common.BuildStagePrepare:                  b.writePrepareScript,
		common.BuildStageGetSources:               b.writeGetSourcesScript,
		common.BuildStageClearWorktree:            b.writeClearWorktreeScript,
		common.BuildStageRestoreCache:             b.writeRestoreCacheScript,
		common.BuildStageDownloadArtifacts:        b.writeDownloadArtifactsScript,
		common.BuildStageAfterScript:              b.writeAfterScript,
		common.BuildStageArchiveOnSuccessCache:    b.writeArchiveCacheOnSuccessScript,
		common.BuildStageArchiveOnFailureCache:    b.writeArchiveCacheOnFailureScript,
		common.BuildStageUploadOnSuccessArtifacts: b.writeUploadArtifactsOnSuccessScript,
		common.BuildStageUploadOnFailureArtifacts: b.writeUploadArtifactsOnFailureScript,
		common.BuildStageCleanup:                  b.writeCleanupScript,
	}

	fn, ok := methods[buildStage]
	if !ok {
		return b.writeUserScript(w, info, buildStage)
	}
	return fn(ctx, w, info)
}

package shells

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
)

const (
	// When umask is disabled for the Kubernetes executor,
	// a hidden file, .gitlab-build-uid-gid, is created in the `builds_dir` directory to assist the helper container
	// in retrieving the build image's configured `uid:gid`.
	// This information is then applied to the working directories to prevent them from being writable by anyone.
	BuildUidGidFile  = ".gitlab-build-uid-gid"
	StartupProbeFile = ".gitlab-startup-marker"

	gitlabEnvFileName      = "gitlab_runner_env"
	gitlabCacheEnvFileName = "gitlab_runner_cache_env"
	credHelperConfFile     = "cred-helper.conf"
	// The same cred helper is used across all shells & OSs
	// git always comes (or depends) on a POSIX shell, so any helper can rely on that, regardless of the OS, git distribution, ...
	credHelperCommand = `!f(){ if [ "$1" = "get" ] ; then echo "password=${CI_JOB_TOKEN}" ; fi ; } ; f`
	gitDir            = ".git"
	gitTemplateDir    = "git-template"
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

func (b *AbstractShell) cacheFile(build *common.Build, userKey string) (string, string, error) {
	if build.CacheDir == "" {
		return "", "", fmt.Errorf("unset cache directory")
	}

	// Deduce cache key
	key := path.Join(build.JobInfo.Name, build.GitInfo.Ref)
	if userKey != "" {
		key = build.GetAllVariables().ExpandValue(userKey)
	}

	// Ignore cache without the key
	if key == "" {
		return "", "", fmt.Errorf("empty cache key")
	}

	file := path.Join(build.CacheDir, key, "cache.zip")
	if build.IsFeatureFlagOn(featureflags.UsePowershellPathResolver) {
		return key, file, nil
	}

	file, err := filepath.Rel(build.BuildDir, file)
	if err != nil {
		return "", "", fmt.Errorf("inability to make the cache file path relative to the build directory" +
			" (is the build directory absolute?)")
	}

	return key, file, nil
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
		cacheKey, cacheFile, err := b.cacheFile(info.Build, cacheOptions.Key)
		if err != nil {
			w.Noticef("Skipping cache extraction due to %v", err)
			continue
		}

		cacheOptions.Policy = common.CachePolicy(info.Build.GetAllVariables().ExpandValue(string(cacheOptions.Policy)))

		if ok, err := cacheOptions.CheckPolicy(common.CachePolicyPull); err != nil {
			return fmt.Errorf("%w for %s", err, cacheKey)
		} else if !ok {
			w.Noticef("Not downloading cache %s due to policy", cacheKey)
			continue
		}

		b.extractCacheOrFallbackCachesWrapper(ctx, w, info, cacheFile, cacheKey, cacheOptions)
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
	cacheFile string,
	cacheKey string,
	cacheOptions common.Cache,
) {
	allowedCacheKeys := []string{cacheKey}

	for _, cacheKey := range cacheOptions.FallbackKeys {
		if cacheKey != "" {
			allowedCacheKeys = append(allowedCacheKeys, info.Build.GetAllVariables().ExpandValue(cacheKey))
		}
	}

	// We make sure that the CACHE_FALLBACK_KEY doesn't end with characters like space, slash and backslash
	// TrimRight remove all occurrences of those character at the end of the fallback key
	defaultFallbackCacheKey := sanitizeCacheFallbackKey(info.Build.GetAllVariables().Value("CACHE_FALLBACK_KEY"))
	if defaultFallbackCacheKey != "" && !strings.HasSuffix(defaultFallbackCacheKey, "-protected") {
		// The `-protected` suffix is reserved for protected refs, so we disallow it in the global variable.
		allowedCacheKeys = append(allowedCacheKeys, defaultFallbackCacheKey)
	}

	// Execute cache-extractor command. Failure is not fatal.
	b.guardRunnerCommand(w, info.RunnerCommand, "Extracting cache", func() {
		b.addExtractCacheCommand(ctx, w, info, cacheFile, allowedCacheKeys, cacheOptions.Paths)
	})
}

func sanitizeCacheFallbackKey(fallbackKey string) string {
	return strings.TrimRight(fallbackKey, " /\\")
}

func (b *AbstractShell) addExtractCacheCommand(
	ctx context.Context,
	w ShellWriter,
	info common.ShellScriptInfo,
	cacheFile string,
	cacheKeys []string,
	cachePaths []string,
) {
	cacheKey := cacheKeys[0]
	args := []string{
		"cache-extractor",
		"--file", cacheFile,
		"--timeout", strconv.Itoa(info.Build.GetCacheRequestTimeout()),
	}

	extraArgs, env, err := getCacheDownloadURLAndEnv(ctx, info.Build, cacheKey)
	args = append(args, extraArgs...)

	if err != nil {
		w.Warningf("Failed to obtain environment for cache %s: %v", cacheKey, err)
	}

	w.Noticef("Checking cache for %s...", cacheKey)
	cacheEnvFilename := ""

	if env != nil {
		cacheEnvFilename = b.writeCacheExports(w, env)
		args = append(args, "--env-file", cacheEnvFilename)
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
	if len(cacheKeys) > 1 {
		_, cacheFile, err := b.cacheFile(info.Build, cacheKeys[1])
		if err != nil {
			w.Noticef("Skipping cache extraction due to %v", err)
		} else {
			b.addExtractCacheCommand(ctx, w, info, cacheFile, cacheKeys[1:], cachePaths)
		}
	}
	w.EndIf()

	if cacheEnvFilename != "" {
		w.RmFile(cacheEnvFilename)
	}
}

// getCacheDownloadURLAndEnv will first try to generate the GoCloud URL if it's
// available then fallback to a pre-signed URL.
func getCacheDownloadURLAndEnv(ctx context.Context, build *common.Build, cacheKey string) ([]string, map[string]string, error) {
	// Prefer Go Cloud URL if supported
	goCloudURL, err := cache.GetCacheGoCloudURL(ctx, build, cacheKey, false)

	if goCloudURL.URL != nil {
		return []string{"--gocloud-url", goCloudURL.URL.String()}, goCloudURL.Environment, err
	}

	if url := cache.GetCacheDownloadURL(ctx, build, cacheKey); url.URL != nil {
		return []string{"--url", url.URL.String()}, nil, nil
	}

	return []string{}, nil, nil
}

func (b *AbstractShell) downloadArtifacts(w ShellWriter, job common.Dependency, info common.ShellScriptInfo) {
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

func (b *AbstractShell) jobArtifacts(info common.ShellScriptInfo) (otherJobs []common.Dependency) {
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

	w.Variable(common.JobVariable{Key: "GIT_TERMINAL_PROMPT", Value: "0"})
	w.Variable(common.JobVariable{Key: "GCM_INTERACTIVE", Value: "Never"})

	if !info.Build.IsSharedEnv() {
		b.writeGitSSLConfig(w, info.Build, []string{"--global"})
	}

	b.guardGetSourcesScriptHooks(w, info, "pre_clone_script", func() []string {
		var s []string

		if info.PreGetSourcesScript != "" {
			s = append(s, info.PreGetSourcesScript)
		}

		h := info.Build.Hooks.Get(common.HookPreGetSourcesScript)
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

		h := info.Build.Hooks.Get(common.HookPostGetSourcesScript)
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

// credConfigFile returns the file path were we expect configuration for a credential helper setup. If an empty string
// is returned, we are not expected to neither setup a cred helper nor use one.
func (b *AbstractShell) credConfigFile(build *common.Build, w ShellWriter) string {
	if !build.IsFeatureFlagOn(featureflags.GitURLsWithoutTokens) {
		return ""
	}

	return w.TmpFile(credHelperConfFile)
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
	if len(s) == 0 || info.Build.GetGitStrategy() == common.GitNone {
		return
	}

	b.writeCommands(w, info, prefix, s...)
}

func (b *AbstractShell) writeExports(w ShellWriter, info common.ShellScriptInfo) {
	for _, variable := range info.Build.GetAllVariables() {
		w.Variable(variable)
	}

	gitlabEnvFile := w.TmpFile(gitlabEnvFileName)

	w.Variable(common.JobVariable{
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

	u, err := url.Parse(remoteURL)
	if err != nil {
		return "", fmt.Errorf("parsing remote URL: %w", err)
	}

	return url_helpers.OnlySchemeAndHost(u).String(), nil
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
		w.Variable(common.JobVariable{Key: "GIT_LFS_SKIP_SMUDGE", Value: "1"})
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

	switch build.GetGitStrategy() {
	case common.GitFetch:
		return b.writeRefspecFetchCmd(w, info)
	case common.GitClone:
		w.RmDir(projectDir)
		return b.writeRefspecFetchCmd(w, info)
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

//nolint:funlen
func (b *AbstractShell) writeRefspecFetchCmd(w ShellWriter, info common.ShellScriptInfo) error {
	build := info.Build
	projectDir := build.FullProjectDir()
	depth := build.GitInfo.Depth

	if depth > 0 {
		w.Noticef("Fetching changes with git depth set to %d...", depth)
	} else {
		w.Noticef("Fetching changes...")
	}

	// initializing
	templateDir := w.MkTmpDir(gitTemplateDir)
	templateFile := w.Join(templateDir, "config")
	objectFormat := build.GetRepositoryObjectFormat()

	b.writeGitCleanup(w, build)

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

	if build.IsFeatureFlagOn(featureflags.UseGitBundleURIs) {
		w.Command("git", "config", "-f", templateFile, "transfer.bundleURI", "true")
	}

	if build.IsSharedEnv() {
		b.writeGitSSLConfig(w, build, []string{"-f", templateFile})
	}

	if objectFormat != common.DefaultObjectFormat {
		w.Command("git", "init", projectDir, "--template", templateDir, "--object-format", objectFormat)
	} else {
		w.Command("git", "init", projectDir, "--template", templateDir)
	}

	w.Cd(projectDir)

	remoteURL, err := build.GetRemoteURL()
	if err != nil {
		return fmt.Errorf("writing fetch commands: %w", err)
	}

	if credConfigFile := b.credConfigFile(build, w); credConfigFile != "" {
		err := b.configureGitCredHelper(w, info, credConfigFile)
		if err != nil {
			return fmt.Errorf("writing fetch commands: %w", err)
		}
	}

	// Add `git remote` or update existing
	w.IfCmd("git", "remote", "add", "origin", remoteURL)
	w.Noticef("Created fresh repository.")
	w.Else()
	w.Command("git", "remote", "set-url", "origin", remoteURL)
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

	return nil
}

func (b *AbstractShell) configureGitCredHelper(w ShellWriter, info common.ShellScriptInfo, credConfigFile string) error {
	w.RmFile(credConfigFile)
	build := info.Build
	shellName := build.Runner.Shell
	if shellName == "" {
		// GetDefaultShell will panic, if it can't find a default shell
		shellName = common.GetDefaultShell()
	}

	shell := common.GetShell(shellName)
	if shell == nil {
		return fmt.Errorf("unknown shell %q", shellName)
	}

	remoteHost, err := b.getRemoteHost(build)
	if err != nil {
		return fmt.Errorf("getting remote host: %w", err)
	}

	credSection := "credential." + remoteHost

	w.Command("git", "config", "-f", credConfigFile, credSection+".username", "gitlab-ci-token")
	// To not have global / system-wide cred helpers interfere, we disable all current helpers and set up our own one.
	// With this any other cred helper (e.g. GCM) will keep their creds as are, we will neither add/update nor delete any creds thereof.
	w.Command("git", "config", "-f", credConfigFile, "--replace-all", credSection+".helper", shell.GetExternalCommandEmptyArgument())
	w.Command("git", "config", "-f", credConfigFile, "--add", credSection+".helper", shell.GetGitCredHelperCommand())

	w.Command("git", "config", "include.path", credConfigFile)

	return nil
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

	// Update / initialize submodules
	gitURLArgs, err := build.GetURLInsteadOfArgs()
	if err != nil {
		return fmt.Errorf("writing submodule update commands: %w", err)
	}

	if credConfigFile := b.credConfigFile(build, w); credConfigFile != "" {
		gitURLArgs = append(gitURLArgs, "-c", "include.path="+credConfigFile)
	}

	updateArgs := append(gitURLArgs, "submodule", "update", "--init") //nolint:gocritic
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

	w.IfCmdWithOutput("git", updateArgs...)
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
		//	git \
		//		-c url.https://test.local.insteadOf=ssh://git@test.local \
		//		-c include.path=blipp/blarp/blarz.conf \
		//		submodule foreach 'git fetch origin +refs/heads/*:refs/remotes/origin/*'
		w.Command("git", slices.Concat(
			gitURLArgs, foreachArgs, []string{"git fetch origin +refs/heads/*:refs/remotes/origin/*"},
		)...)
	}

	w.Command("git", syncArgs...)
	w.Command("git", updateArgs...)
	w.Command("git", append(foreachArgs, "git reset --hard")...)
	w.EndIf()

	w.Command("git", append(foreachArgs, cleanCommand...)...)

	if !build.IsLFSSmudgeDisabled() {
		w.IfCmd("git", "lfs", "version")
		w.Command("git", append(append(gitURLArgs, foreachArgs...), "git lfs pull")...)
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
			info.Build.JobResponse.Features.TraceSections {
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
	var scriptStep *common.Step
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

		// Skip archiving if no cache is defined
		cacheKey, cacheFile, err := b.cacheFile(info.Build, cacheOptions.Key)
		if err != nil {
			w.Noticef("Skipping cache archiving due to %v", err)
			continue
		}

		cacheOptions.Policy = common.CachePolicy(info.Build.GetAllVariables().ExpandValue(string(cacheOptions.Policy)))

		if ok, err := cacheOptions.CheckPolicy(common.CachePolicyPush); err != nil {
			return false, fmt.Errorf("%w for %s", err, cacheKey)
		} else if !ok {
			w.Noticef("Not uploading cache %s due to policy", cacheKey)
			continue
		}

		b.addCacheUploadCommand(ctx, w, info, cacheFile, archiverArgs, cacheKey)
	}

	return skipArchiveCache, nil
}

func (b *AbstractShell) getArchiverArgs(cacheOptions common.Cache, _ common.ShellScriptInfo) []string {
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
	cacheFile string,
	archiverArgs []string,
	cacheKey string,
) {
	args := []string{
		"cache-archiver",
		"--file", cacheFile,
		"--timeout", strconv.Itoa(info.Build.GetCacheRequestTimeout()),
	}

	if info.Build.Runner.Cache != nil && info.Build.Runner.Cache.MaxUploadedArchiveSize > 0 {
		args = append(
			args,
			"--max-uploaded-archive-size",
			strconv.FormatInt(info.Build.Runner.Cache.MaxUploadedArchiveSize, 10),
		)
	}

	args = append(args, archiverArgs...)

	// Generate cache upload address
	extraArgs, env, err := getCacheUploadURLAndEnv(ctx, info.Build, cacheKey)
	args = append(args, extraArgs...)

	if err != nil {
		w.Warningf("Unable to generate cache upload environment: %v", err)
	}

	// Execute cache-archiver command. Failure is not fatal.
	b.guardRunnerCommand(w, info.RunnerCommand, "Creating cache", func() {
		w.Noticef("Creating cache %s...", cacheKey)
		cacheEnvFilename := ""

		if env != nil {
			cacheEnvFilename = b.writeCacheExports(w, env)
			args = append(args, "--env-file", cacheEnvFilename)
		}

		w.IfCmdWithOutput(info.RunnerCommand, args...)
		w.Noticef("Created cache")
		w.Else()
		w.Warningf("Failed to create cache")
		w.EndIf()

		if cacheEnvFilename != "" {
			w.RmFile(cacheEnvFilename)
		}
	})
}

// getCacheUploadURLAndEnv will first try to generate the GoCloud URL if it's
// available then fallback to a pre-signed URL.
func getCacheUploadURLAndEnv(ctx context.Context, build *common.Build, cacheKey string) ([]string, map[string]string, error) {
	// Prefer Go Cloud URL if supported
	goCloudURL, err := cache.GetCacheGoCloudURL(ctx, build, cacheKey, true)
	if goCloudURL.URL != nil {
		return []string{"--gocloud-url", goCloudURL.URL.String()}, goCloudURL.Environment, err
	}

	uploadURL := cache.GetCacheUploadURL(ctx, build, cacheKey)
	if uploadURL.URL == nil {
		return []string{}, nil, nil
	}

	urlArgs := []string{"--url", uploadURL.URL.String()}
	for key, values := range uploadURL.Headers {
		for _, value := range values {
			urlArgs = append(urlArgs, "--header", fmt.Sprintf("%s: %s", key, value))
		}
	}

	return urlArgs, nil, err
}

func (b *AbstractShell) writeUploadArtifact(w ShellWriter, info common.ShellScriptInfo, artifact common.Artifact) bool {
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

func (b *AbstractShell) shouldGenerateArtifactsMetadata(info common.ShellScriptInfo, artifact common.Artifact) bool {
	generateArtifactsMetadata := info.Build.Variables.Bool(common.GenerateArtifactsMetadataVariable)
	// Currently only zip artifacts are supported as artifact metadata effectively adds another file to the archive
	// https://gitlab.com/gitlab-org/gitlab/-/issues/367203#note_1059841610
	metadataArtifactsFormatSupported := artifact.Format == common.ArtifactFormatZip
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
	var afterScriptStep *common.Step
	for _, step := range info.Build.Steps {
		if step.Name == common.StepNameAfterScript {
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

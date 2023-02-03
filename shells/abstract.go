package shells

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls"
)

var errUnknownGitStrategy = errors.New("unknown GIT_STRATEGY")

type stringQuoter func(string) string

func singleQuote(s string) string {
	return fmt.Sprintf(`'%s'`, s)
}

func doubleQuote(s string) string {
	return `"` + s + `"`
}

type AbstractShell struct {
}

func (b *AbstractShell) GetFeatures(features *common.FeaturesInfo) {
	features.Artifacts = true
	features.UploadMultipleArtifacts = true
	features.UploadRawArtifacts = true
	features.Cache = true
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

func (b *AbstractShell) cacheFile(build *common.Build, userKey string) (key, file string) {
	if build.CacheDir == "" {
		return
	}

	// Deduce cache key
	key = path.Join(build.JobInfo.Name, build.GitInfo.Ref)
	if userKey != "" {
		key = build.GetAllVariables().ExpandValue(userKey)
	}

	// Ignore cache without the key
	if key == "" {
		return
	}

	file = path.Join(build.CacheDir, key, "cache.zip")
	if build.IsFeatureFlagOn(featureflags.UsePowershellPathResolver) {
		return key, file
	}

	file, err := filepath.Rel(build.BuildDir, file)
	if err != nil {
		return "", ""
	}
	return
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

func (b *AbstractShell) cacheExtractor(w ShellWriter, info common.ShellScriptInfo) error {
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
		cacheKey, cacheFile := b.cacheFile(info.Build, cacheOptions.Key)
		if cacheKey == "" {
			w.Noticef("Skipping cache extraction due to empty cache key")
			continue
		}

		if ok, err := cacheOptions.CheckPolicy(common.CachePolicyPull); err != nil {
			return fmt.Errorf("%w for %s", err, cacheKey)
		} else if !ok {
			w.Noticef("Not downloading cache %s due to policy", cacheKey)
			continue
		}

		b.extractCacheOrFallbackCacheWrapper(w, info, cacheFile, cacheKey)
	}

	if skipRestoreCache {
		return common.ErrSkipBuildStage
	}

	return nil
}

func (b *AbstractShell) extractCacheOrFallbackCacheWrapper(
	w ShellWriter,
	info common.ShellScriptInfo,
	cacheFile string,
	cacheKey string,
) {
	cacheFallbackKey := info.Build.GetAllVariables().Value("CACHE_FALLBACK_KEY")
	if strings.HasSuffix(cacheFallbackKey, "-protected") {
		// The `-protected` suffix is reserved for protected refs, so we disallow it from user-specified values.
		cacheFallbackKey = ""
	}

	// Execute cache-extractor command. Failure is not fatal.
	b.guardRunnerCommand(w, info.RunnerCommand, "Extracting cache", func() {
		b.addExtractCacheCommand(w, info, cacheFile, cacheKey, cacheFallbackKey)
	})
}

func (b *AbstractShell) addExtractCacheCommand(
	w ShellWriter,
	info common.ShellScriptInfo,
	cacheFile string,
	cacheKey string,
	cacheFallbackKey string,
) {
	args := []string{
		"cache-extractor",
		"--file", cacheFile,
		"--timeout", strconv.Itoa(info.Build.GetCacheRequestTimeout()),
	}

	if url := cache.GetCacheDownloadURL(info.Build, cacheKey); url != nil {
		args = append(args, "--url", url.String())
	}

	w.Noticef("Checking cache for %s...", cacheKey)
	w.IfCmdWithOutput(info.RunnerCommand, args...)
	w.Noticef("Successfully extracted cache")
	w.Else()
	w.Warningf("Failed to extract cache")
	if cacheFallbackKey != "" {
		b.addExtractCacheCommand(w, info, cacheFile, cacheFallbackKey, "")
	}
	w.EndIf()
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

	return nil
}

func (b *AbstractShell) writePrepareScript(w ShellWriter, info common.ShellScriptInfo) error {
	return nil
}

func (b *AbstractShell) writeGetSourcesScript(w ShellWriter, info common.ShellScriptInfo) error {
	b.writeExports(w, info)

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
}

func (b *AbstractShell) writeGitSSLConfig(w ShellWriter, build *common.Build, where []string) {
	repoURL, err := url.Parse(build.GetRemoteURL())
	if err != nil {
		w.Warningf("git SSL config: Can't parse repository URL. %s", err)
		return
	}

	repoURL.Path = ""
	repoURL.User = nil
	host := repoURL.String()
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

	err := b.handleGetSourcesStrategy(w, build)
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

func (b *AbstractShell) handleGetSourcesStrategy(w ShellWriter, build *common.Build) error {
	projectDir := build.FullProjectDir()

	switch build.GetGitStrategy() {
	case common.GitFetch:
		b.writeRefspecFetchCmd(w, build, projectDir)
	case common.GitClone:
		w.RmDir(projectDir)
		b.writeRefspecFetchCmd(w, build, projectDir)
	case common.GitNone:
		w.Noticef("Skipping Git repository setup")
		w.MkDir(projectDir)
	default:
		return errUnknownGitStrategy
	}

	return nil
}

func (b *AbstractShell) writeRefspecFetchCmd(w ShellWriter, build *common.Build, projectDir string) {
	depth := build.GitInfo.Depth

	if depth > 0 {
		w.Noticef("Fetching changes with git depth set to %d...", depth)
	} else {
		w.Noticef("Fetching changes...")
	}

	// initializing
	templateDir := w.MkTmpDir("git-template")
	templateFile := w.Join(templateDir, "config")

	w.Command("git", "config", "-f", templateFile, "init.defaultBranch", "none")
	w.Command("git", "config", "-f", templateFile, "fetch.recurseSubmodules", "false")
	if build.IsSharedEnv() {
		b.writeGitSSLConfig(w, build, []string{"-f", templateFile})
	}

	b.writeGitCleanup(w, build.GetSubmoduleStrategy(), projectDir)

	w.Command("git", "init", projectDir, "--template", templateDir)
	w.Cd(projectDir)

	// Add `git remote` or update existing
	w.IfCmd("git", "remote", "add", "origin", build.GetRemoteURL())
	w.Noticef("Created fresh repository.")
	w.Else()
	w.Command("git", "remote", "set-url", "origin", build.GetRemoteURL())
	w.EndIf()

	v := common.AppVersion
	userAgent := fmt.Sprintf("http.userAgent=%s %s %s/%s", v.Name, v.Version, v.OS, v.Architecture)

	fetchArgs := []string{"-c", userAgent, "fetch", "origin"}
	fetchArgs = append(fetchArgs, build.GitInfo.Refspecs...)
	if depth > 0 {
		fetchArgs = append(fetchArgs, "--depth", strconv.Itoa(depth))
	}

	fetchArgs = append(fetchArgs, build.GetGitFetchFlags()...)

	w.Command("git", fetchArgs...)
}

func (b *AbstractShell) writeGitCleanup(w ShellWriter, submoduleStrategy common.SubmoduleStrategy, projectDir string) {
	const gitDir = ".git"

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
		if submoduleStrategy == common.SubmoduleNormal || submoduleStrategy == common.SubmoduleRecursive {
			w.RmFilesRecursive(path.Join(projectDir, gitDir, "modules"), path.Base(f))
		}
	}
}

func (b *AbstractShell) writeCheckoutCmd(w ShellWriter, build *common.Build) {
	w.Noticef("Checking out %s as detached HEAD (ref is %s)...", build.GitInfo.Sha[0:8], build.GitInfo.Ref)
	w.Command("git", "checkout", "-f", "-q", build.GitInfo.Sha)

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
	args := []string{"submodule", "sync"}
	if recursive {
		args = append(args, "--recursive")
	}
	args = append(args, pathArgs...)
	w.Command("git", args...)

	// Update / initialize submodules
	updateArgs := []string{"submodule", "update", "--init"}
	foreachArgs := []string{"submodule", "foreach"}
	gitSubmoduleUpdateFlags := build.GetGitSubmoduleUpdateFlags()
	if recursive {
		updateArgs = append(updateArgs, "--recursive")
		foreachArgs = append(foreachArgs, "--recursive")
	}
	if depth > 0 {
		updateArgs = append(updateArgs, "--depth", strconv.Itoa(depth))
	}
	updateArgs = append(updateArgs, gitSubmoduleUpdateFlags...)
	updateArgs = append(updateArgs, pathArgs...)

	// Clean changed files in submodules
	w.Command("git", append(foreachArgs, "git clean -ffxd")...)
	w.Command("git", append(foreachArgs, "git reset --hard")...)
	w.Command("git", updateArgs...)
	// Clean changed files in sub-submodules
	w.Command("git", append(foreachArgs, "git clean -ffxd")...)

	if !build.IsLFSSmudgeDisabled() {
		w.IfCmd("git", "lfs", "version")
		w.Command("git", append(foreachArgs, "git lfs pull")...)
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

func (b *AbstractShell) writeRestoreCacheScript(w ShellWriter, info common.ShellScriptInfo) error {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	// Try to restore from main cache, if not found cache for default branch
	return b.cacheExtractor(w, info)
}

func (b *AbstractShell) writeDownloadArtifactsScript(w ShellWriter, info common.ShellScriptInfo) error {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	return b.downloadAllArtifacts(w, info)
}

// Write the given string of commands using the provided ShellWriter object.
func (b *AbstractShell) writeCommands(w ShellWriter, info common.ShellScriptInfo, prefix string, commands ...string) {
	for i, command := range commands {
		command = strings.TrimSpace(command)

		if info.Build.IsFeatureFlagOn(featureflags.ScriptSections) &&
			info.Build.JobResponse.Features.TraceSections {
			b.writeCommandWithSection(w, fmt.Sprintf("%s_%d", prefix, i), command)
			continue
		}

		if command != "" {
			lines := strings.SplitN(command, "\n", 2)
			if len(lines) > 1 {
				// TODO: this should be collapsable once we introduce that in GitLab
				w.Noticef("$ %s # collapsed multi-line command", lines[0])
			} else {
				w.Noticef("$ %s", lines[0])
			}
		} else {
			w.EmptyLine()
		}
		w.Line(command)
		w.CheckForErrors()
	}
}

func (b *AbstractShell) writeCommandWithSection(w ShellWriter, sectionName, command string) {
	if command == "" {
		w.EmptyLine()
	}

	lines := strings.SplitN(command, "\n", 2)
	if len(lines) > 1 {
		w.SectionStart(sectionName, fmt.Sprintf("$ %s # collapsed multi-line command", lines[0]))
	} else {
		w.SectionStart(sectionName, fmt.Sprintf("$ %s", lines[0]))
	}

	w.Line(command)
	w.CheckForErrors()
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

func (b *AbstractShell) cacheArchiver(w ShellWriter, info common.ShellScriptInfo, onSuccess bool) error {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	skipArchiveCache, err := b.archiveCache(w, info, onSuccess)
	if err != nil {
		return err
	}

	if skipArchiveCache {
		return common.ErrSkipBuildStage
	}

	return nil
}

func (b *AbstractShell) archiveCache(w ShellWriter, info common.ShellScriptInfo, onSuccess bool) (bool, error) {
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
		cacheKey, cacheFile := b.cacheFile(info.Build, cacheOptions.Key)
		if cacheKey == "" {
			w.Noticef("Skipping cache archiving due to empty cache key")
			continue
		}

		if ok, err := cacheOptions.CheckPolicy(common.CachePolicyPush); err != nil {
			return false, fmt.Errorf("%w for %s", err, cacheKey)
		} else if !ok {
			w.Noticef("Not uploading cache %s due to policy", cacheKey)
			continue
		}

		b.addCacheUploadCommand(w, info, cacheFile, archiverArgs, cacheKey)
	}

	return skipArchiveCache, nil
}

func (b *AbstractShell) getArchiverArgs(cacheOptions common.Cache, info common.ShellScriptInfo) []string {
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
	args = append(args, getCacheUploadURL(info.Build, cacheKey)...)

	env := cache.GetCacheUploadEnv(info.Build, cacheKey)

	// Execute cache-archiver command. Failure is not fatal.
	b.guardRunnerCommand(w, info.RunnerCommand, "Creating cache", func() {
		w.Noticef("Creating cache %s...", cacheKey)

		for key, value := range env {
			w.Variable(common.JobVariable{Key: key, Value: value})
		}

		w.IfCmdWithOutput(info.RunnerCommand, args...)
		w.Noticef("Created cache")
		w.Else()
		w.Warningf("Failed to create cache")
		w.EndIf()
	})
}

// getCacheUploadURL will first try to generate the GoCloud URL if it's
// available then fallback to a pre-signed URL.
func getCacheUploadURL(build *common.Build, cacheKey string) []string {
	// Prefer Go Cloud URL if supported
	goCloudURL := cache.GetCacheGoCloudURL(build, cacheKey)
	if goCloudURL != nil {
		return []string{"--gocloud-url", goCloudURL.String()}
	}

	uploadURL := cache.GetCacheUploadURL(build, cacheKey)
	if uploadURL == nil {
		return []string{}
	}

	urlArgs := []string{"--url", uploadURL.String()}
	httpHeaders := cache.GetCacheUploadHeaders(build, cacheKey)
	for key, values := range httpHeaders {
		for _, value := range values {
			urlArgs = append(urlArgs, "--header", fmt.Sprintf("%s: %s", key, value))
		}
	}

	return urlArgs
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
		info.Build.ExecutorName(),
		"--runner-name",
		info.Build.Runner.Name,
		"--started-at",
		info.Build.StartedAt().Format(time.RFC3339),
		"--ended-at",
		time.Now().Format(time.RFC3339),
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

func (b *AbstractShell) writeAfterScript(w ShellWriter, info common.ShellScriptInfo) error {
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

func (b *AbstractShell) writeUploadArtifactsOnSuccessScript(w ShellWriter, info common.ShellScriptInfo) error {
	return b.writeUploadArtifacts(w, info, true)
}

func (b *AbstractShell) writeUploadArtifactsOnFailureScript(w ShellWriter, info common.ShellScriptInfo) error {
	return b.writeUploadArtifacts(w, info, false)
}

func (b *AbstractShell) writeArchiveCacheOnSuccessScript(w ShellWriter, info common.ShellScriptInfo) error {
	return b.cacheArchiver(w, info, true)
}

func (b *AbstractShell) writeArchiveCacheOnFailureScript(w ShellWriter, info common.ShellScriptInfo) error {
	return b.cacheArchiver(w, info, false)
}

func (b *AbstractShell) writeCleanupScript(w ShellWriter, info common.ShellScriptInfo) error {
	skipCleanupStage := true

	for _, variable := range info.Build.GetAllVariables() {
		if !variable.File {
			continue
		}

		skipCleanupStage = false
		w.RmFile(w.TmpFile(variable.Key))
	}

	if info.Build.IsFeatureFlagOn(featureflags.EnableJobCleanup) {
		skipCleanupStage = false

		if err := b.writeCleanupBuildDirectoryScript(w, info); err != nil {
			return err
		}

		w.RmFile(filepath.Join(info.Build.FullProjectDir(), ".git", "config"))
	}

	if skipCleanupStage {
		return common.ErrSkipBuildStage
	}

	return nil
}

func (b *AbstractShell) writeCleanupBuildDirectoryScript(w ShellWriter, info common.ShellScriptInfo) error {
	switch info.Build.GetGitStrategy() {
	case common.GitClone:
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
				submoduleCleanArgs := append(submoduleArgs, append([]string{"git"}, cleanArgs...)...)
				w.Command("git", submoduleCleanArgs...)
			}

			submoduleResetArgs := append(submoduleArgs, append([]string{"git"}, resetArgs...)...)
			w.Command("git", submoduleResetArgs...)
		}
	case common.GitNone:
		w.Noticef("Skipping build directory cleanup step")

	default:
		return errUnknownGitStrategy
	}

	return nil
}

func (b *AbstractShell) writeScript(w ShellWriter, buildStage common.BuildStage, info common.ShellScriptInfo) error {
	methods := map[common.BuildStage]func(ShellWriter, common.ShellScriptInfo) error{
		common.BuildStagePrepare:                  b.writePrepareScript,
		common.BuildStageGetSources:               b.writeGetSourcesScript,
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
	return fn(w, info)
}

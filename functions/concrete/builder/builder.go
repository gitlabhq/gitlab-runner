package builder

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/cache/cachekey"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/builder/variables"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/cacheprovider"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/stages"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
)

var (
	gitCleanFlagsDefault = []string{"-ffdx"}
	gitFetchFlagsDefault = []string{"--prune", "--quiet"}
)

func Build(job spec.Job, vars variables.Provider, options ...Option) ([]byte, error) {
	opts, err := newOptions(options)
	if err != nil {
		return nil, err
	}

	b := builder{opts: opts, meta: job, variables: vars}

	config := run.Config{
		CacheDir:           opts.cacheDir,
		ArchiverStagingDir: opts.archiverStagingDir,
		Shell:              opts.shell,
		LoginShell:         opts.loginShell,
		Timeout:            time.Duration(b.meta.RunnerInfo.Timeout) * time.Second,
		ID:                 job.ID,
		Token:              job.Token,
		BaseURL:            vars.Get("CI_SERVER_URL"),
	}

	config.GetSources, err = b.buildGetSources()
	if err != nil {
		return nil, err
	}

	config.CacheExtract, err = b.buildCacheExtract()
	if err != nil {
		return nil, err
	}

	config.ArtifactExtract = b.buildArtifactDownloads()
	config.Steps = b.buildSteps()
	config.ScriptTimeout = b.buildScriptTimeout()
	config.AfterScriptTimeout = b.buildAfterScriptTimeout()
	config.AfterScriptIgnoreErrors = variables.DefaultBool(
		b.variables, "AFTER_SCRIPT_IGNORE_ERRORS", true,
	)
	config.TraceSections = b.meta.Features.TraceSections

	config.CacheArchive, err = b.buildCacheArchive()
	if err != nil {
		return nil, err
	}

	config.ArtifactsArchive = b.buildArtifactUploads()

	config.Cleanup = b.buildCleanup(config.GetSources)

	return json.Marshal(config)
}

type builder struct {
	opts      options
	variables variables.Provider
	meta      spec.Job
}

func (b *builder) buildGetSources() (stages.GetSources, error) {
	gitAuthHelper := url_helpers.NewGitAuthHelper(url_helpers.GitAuthConfig{
		CloneURL:               b.opts.cloneURL,
		CredentialsURL:         b.variables.Get("CI_SERVER_URL"),
		RepoURL:                b.meta.GitInfo.RepoURL,
		GitSubmoduleForceHTTPS: variables.DefaultBool(b.variables, "GIT_SUBMODULE_FORCE_HTTPS", false),
		Token:                  b.meta.Token,
		ProjectPath:            b.variables.Get("CI_PROJECT_PATH"),
		Server: url_helpers.GitAuthServerConfig{
			Host:    b.variables.Get("CI_SERVER_HOST"),
			SSHHost: b.variables.Get("CI_SERVER_SHELL_SSH_HOST"),
			SSHPort: b.variables.Get("CI_SERVER_SHELL_SSH_PORT"),
		},
	}, !b.isFeatureFlagOn(featureflags.GitURLsWithoutTokens))

	remoteURL, err := gitAuthHelper.GetRemoteURL()
	if err != nil {
		return stages.GetSources{}, err
	}

	insteadOfs, err := gitAuthHelper.GetInsteadOfs()
	if err != nil {
		return stages.GetSources{}, err
	}

	defaultGitStrategy := "clone"
	if b.meta.AllowGitFetch {
		defaultGitStrategy = "fetch"
	}

	return stages.GetSources{
		AllowGitFetch:     b.meta.AllowGitFetch,
		Checkout:          variables.DefaultBool(b.variables, "GIT_CHECKOUT", true),
		MaxAttempts:       variables.DefaultIntClamp(b.variables, "GET_SOURCES_ATTEMPTS", 1, 1, 10),
		SubmoduleStrategy: variables.Default(b.variables, "GIT_SUBMODULE_STRATEGY", "none", "none", "normal", "recursive"),
		LFSDisabled:       variables.DefaultBool(b.variables, "GIT_LFS_SKIP_SMUDGE", false),
		Depth:             b.meta.GitInfo.Depth,
		RepoURL:           b.meta.GitInfo.RepoURL,
		Refspecs:          b.meta.GitInfo.Refspecs,
		SHA:               b.meta.GitInfo.Sha,
		Ref:               b.meta.GitInfo.Ref,
		GitStrategy:       variables.Default(b.variables, "GIT_STRATEGY", defaultGitStrategy, "empty", "none", "fetch", "clone"),
		GitCloneFlags:     b.splitVarFlagsDefault("GIT_CLONE_EXTRA_FLAGS", nil),
		GitFetchFlags:     b.splitVarFlagsDefault("GIT_FETCH_EXTRA_FLAGS", gitFetchFlagsDefault),
		GitCleanFlags:     b.splitVarFlagsDefault("GIT_CLEAN_FLAGS", gitCleanFlagsDefault),
		ObjectFormat:      variables.Default(b.variables, "GIT_OBJECT_FORMAT", "sha1"),

		SubmoduleDepth:       variables.DefaultIntClamp(b.variables, "GIT_SUBMODULE_DEPTH", b.meta.GitInfo.Depth, 0, 10000),
		SubmoduleUpdateFlags: b.splitVarFlags("GIT_SUBMODULE_UPDATE_FLAGS"),
		SubmodulePaths:       b.splitVarFlags("GIT_SUBMODULE_PATHS"),

		PreCloneStep: stages.Step{
			Step:              "pre_clone_script",
			Script:            b.opts.preCloneScript,
			OnSuccess:         true,
			BashExitCodeCheck: b.isFeatureFlagOn(featureflags.EnableBashExitCodeCheck),
			Debug:             b.opts.debug,
		},
		PostCloneStep: stages.Step{
			Step:              "post_clone_script",
			Script:            b.opts.postCloneScript,
			OnSuccess:         true,
			BashExitCodeCheck: b.isFeatureFlagOn(featureflags.EnableBashExitCodeCheck),
			Debug:             b.opts.debug,
		},

		ClearWorktreeOnRetry:  true,
		UseNativeClone:        b.isFeatureFlagOn(featureflags.UseGitNativeClone),
		UseBundleURIs:         b.isFeatureFlagOn(featureflags.UseGitBundleURIs),
		SafeDirectoryCheckout: b.opts.safeDirectoryCheckout,
		CleanGitConfig:        b.opts.gitCleanConfig,
		UseProactiveAuth:      b.isFeatureFlagOn(featureflags.UseGitProactiveAuth),
		IsSharedEnv:           b.opts.isSharedEnv,
		UseCredentialHelper:   b.isFeatureFlagOn(featureflags.GitURLsWithoutTokens),
		RemoteHost:            url_helpers.OnlySchemeAndHost(remoteURL).String(),
		InsteadOfs:            insteadOfs,
		GitalyCorrelationID:   b.opts.gitalyCorrelationID,
		UserAgent:             b.opts.userAgent,
	}, nil
}

func (b *builder) buildCacheExtract() ([]stages.CacheExtract, error) {
	var extracts []stages.CacheExtract

	for _, cache := range b.meta.Cache {
		if len(cache.Paths) == 0 && !cache.Untracked {
			continue
		}

		// Expand $VAR before classifying — without this, cache.policy: $MY_POLICY
		// would fall into the default arm and the cache stage would be skipped.
		policy := spec.CachePolicy(b.variables.ExpandValue(string(cache.Policy)))
		switch policy {
		case spec.CachePolicyUndefined, spec.CachePolicyPullPush, spec.CachePolicyPull:
		default:
			continue
		}

		sources, warnings, err := b.buildCacheSources(cache)
		if err != nil {
			return nil, err
		}
		if len(sources) == 0 {
			continue
		}

		extracts = append(extracts, stages.CacheExtract{
			Sources:     sources,
			Warnings:    warnings,
			Timeout:     variables.DefaultIntClamp(b.variables, "CACHE_REQUEST_TIMEOUT", 10, 1, 120),
			Concurrency: variables.DefaultIntClamp(b.variables, "FASTZIP_EXTRACTOR_CONCURRENCY", 0, 0, 128),
			Paths:       cache.Paths,
			MaxAttempts: variables.DefaultIntClamp(b.variables, "RESTORE_CACHE_ATTEMPTS", 1, 1, 10),
		})
	}

	return extracts, nil
}

func (b *builder) buildCacheSources(cache spec.Cache) ([]stages.CacheSource, []string, error) {
	var sources []stages.CacheSource
	var warnings []string

	addSource := func(key string) error {
		humanKey, resolvedKey, keyWarnings, err := b.cacheKey(key)
		if err != nil {
			warnings = append(warnings, keyWarnings...)
			warnings = append(warnings, fmt.Sprintf("Skipping cache extraction due to %v", err))
			return nil // non-fatal: skip this source
		}

		var desc cacheprovider.Descriptor
		if b.opts.cacheDownloadDescriptor != nil {
			desc, err = b.opts.cacheDownloadDescriptor(resolvedKey)
			if err != nil {
				return err
			}
		}

		sources = append(sources, stages.CacheSource{
			Name:       humanKey,
			Key:        resolvedKey,
			Descriptor: desc,
			Warnings:   keyWarnings,
		})
		return nil
	}

	if err := addSource(cache.Key); err != nil {
		return nil, nil, err
	}

	for _, fk := range cache.FallbackKeys {
		_ = addSource(fk)
	}

	if fk := b.variables.Get("CACHE_FALLBACK_KEY"); fk != "" {
		if strings.HasSuffix(strings.TrimRight(fk, ". "), "-protected") {
			warnings = append(warnings,
				fmt.Sprintf("CACHE_FALLBACK_KEY %q not allowed to end in %q", fk, "-protected"),
			)
		} else {
			_ = addSource(fk)
		}
	}

	return sources, warnings, nil
}

//nolint:gocognit
func (b *builder) buildCacheArchive() ([]stages.CacheArchive, error) {
	var archives []stages.CacheArchive

	for _, cache := range b.meta.Cache {
		if len(cache.Paths) == 0 && !cache.Untracked {
			continue
		}

		humanKey, resolvedKey, warnings, err := b.cacheKey(cache.Key)
		if err != nil {
			continue
		}

		policy := spec.CachePolicy(b.variables.ExpandValue(string(cache.Policy)))
		switch policy {
		case spec.CachePolicyUndefined, spec.CachePolicyPullPush, spec.CachePolicyPush:
		default:
			continue
		}

		if cache.When == "" {
			cache.When = spec.CacheWhenOnSuccess
		}

		archive := stages.CacheArchive{
			Name:                   humanKey,
			Key:                    resolvedKey,
			Warnings:               warnings,
			Untracked:              cache.Untracked,
			Paths:                  cache.Paths,
			ArchiverFormat:         variables.Default(b.variables, "CACHE_COMPRESSION_FORMAT", b.opts.cacheArchiveFormat),
			CompressionLevel:       variables.Default(b.variables, "CACHE_COMPRESSION_LEVEL", "default"),
			Timeout:                variables.DefaultIntClamp(b.variables, "CACHE_REQUEST_TIMEOUT", 10, 1, 120),
			MaxUploadedArchiveSize: b.opts.cacheMaxUploadArchiveSize,
			OnSuccess:              cache.When == spec.CacheWhenAlways || cache.When == spec.CacheWhenOnSuccess,
			OnFailure:              cache.When == spec.CacheWhenAlways || cache.When == spec.CacheWhenOnFailure,
		}

		if b.opts.cacheUploadDescriptor != nil {
			archive.Descriptor, err = b.opts.cacheUploadDescriptor(resolvedKey)
			if err != nil {
				return nil, err
			}
		}

		archives = append(archives, archive)
	}

	return archives, nil
}

func (b *builder) buildArtifactDownloads() []stages.ArtifactDownload {
	var downloads []stages.ArtifactDownload

	for _, dep := range b.meta.Dependencies {
		if dep.ArtifactsFile.Filename == "" {
			continue
		}

		downloads = append(downloads, stages.ArtifactDownload{
			ID:               dep.ID,
			Token:            dep.Token,
			ArtifactName:     dep.Name,
			Filename:         dep.ArtifactsFile.Filename,
			DownloadAttempts: variables.DefaultIntClamp(b.variables, "ARTIFACT_DOWNLOAD_ATTEMPTS", 1, 1, 10),
			Concurrency:      variables.DefaultIntClamp(b.variables, "FASTZIP_EXTRACTOR_CONCURRENCY", 0, 0, 128),
		})
	}

	return downloads
}

func (b *builder) buildSteps() []stages.Step {
	var steps []stages.Step
	var afterScript []stages.Step

	configure := func(step stages.Step) stages.Step {
		step.BashExitCodeCheck = b.isFeatureFlagOn(featureflags.EnableBashExitCodeCheck)
		step.Debug = b.opts.debug
		step.ScriptSections = b.isFeatureFlagOn(featureflags.ScriptSections) && b.meta.Features.TraceSections
		return step
	}

	for _, step := range b.meta.Steps {
		script := step.Script

		// release step has special handling: expand values
		if step.Name == "release" {
			script = slices.Clone(script)
			for i, s := range script {
				script[i] = b.variables.ExpandValue(s)
			}
		}

		s := configure(stages.Step{
			Step:         string(step.Name),
			Script:       script,
			AllowFailure: step.AllowFailure,
			OnSuccess:    step.When == spec.StepWhenAlways || step.When == spec.StepWhenOnSuccess,
			OnFailure:    step.When == spec.StepWhenAlways || step.When == spec.StepWhenOnFailure,
		})

		if step.Name == spec.StepNameAfterScript {
			s.AllowFailure = true
			s.OnSuccess = true
			s.OnFailure = true
			afterScript = append(afterScript, s)
			continue
		}

		// Match abstract shell semantics: pre_build_script and post_build_script
		// run inside the user step's shell, so shell-only state (exports, set
		// options, function definitions, cd) carries over to the user script.
		s.Script = slices.Concat(b.opts.preBuildScript, s.Script, b.opts.postBuildScript)

		steps = append(steps, s)
	}

	steps = append(steps, afterScript...)

	return steps
}

// buildScriptTimeout returns the script-phase timeout.
// Zero means "use the job-level timeout" (Config.Timeout).
func (b *builder) buildScriptTimeout() time.Duration {
	if v := b.variables.Get("RUNNER_SCRIPT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 0
}

func (b *builder) buildAfterScriptTimeout() time.Duration {
	if v := b.variables.Get("RUNNER_AFTER_SCRIPT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 5 * time.Minute
}

func (b *builder) buildArtifactUploads() []stages.ArtifactUpload {
	var uploads []stages.ArtifactUpload

	for _, artifact := range b.meta.Artifacts {
		if len(artifact.Paths) == 0 && !artifact.Untracked {
			continue
		}

		if artifact.When == "" {
			artifact.When = spec.ArtifactWhenOnSuccess
		}

		upload := stages.ArtifactUpload{
			Untracked:             artifact.Untracked,
			Paths:                 artifact.Paths,
			Exclude:               artifact.Exclude,
			ArtifactName:          artifact.Name,
			ExpireIn:              artifact.ExpireIn,
			Format:                string(artifact.Format),
			CompressionLevel:      variables.Default(b.variables, "ARTIFACT_COMPRESSION_LEVEL", "default"),
			Type:                  artifact.Type,
			Timeout:               b.opts.artifactUploadTimeout,
			ResponseHeaderTimeout: b.opts.artifactResponseHeaderTimeout,
			OnSuccess:             artifact.When == spec.ArtifactWhenAlways || artifact.When == spec.ArtifactWhenOnSuccess,
			OnFailure:             artifact.When == spec.ArtifactWhenAlways || artifact.When == spec.ArtifactWhenOnFailure,
		}

		if b.shouldGenerateArtifactMetadata(artifact) {
			upload.Metadata = b.buildArtifactMetadata()
		}

		uploads = append(uploads, upload)
	}

	return uploads
}

func (b *builder) shouldGenerateArtifactMetadata(artifact spec.Artifact) bool {
	enabled := variables.DefaultBool(b.variables, "RUNNER_GENERATE_ARTIFACTS_METADATA", false)
	// Currently only zip artifacts are supported as artifact metadata effectively
	// adds another file to the archive.
	// https://gitlab.com/gitlab-org/gitlab/-/issues/367203#note_1059841610
	return enabled && artifact.Format == spec.ArtifactFormatZip
}

func (b *builder) buildArtifactMetadata() *stages.ArtifactMetadata {
	schemaVersion := variables.Default(b.variables, "SLSA_PROVENANCE_SCHEMA_VERSION", "unknown")

	meta := &stages.ArtifactMetadata{
		RunnerID:      b.variables.Get("CI_RUNNER_ID"),
		RepoURL:       strings.TrimSuffix(b.meta.GitInfo.RepoURL, ".git"),
		RepoDigest:    b.meta.GitInfo.Sha,
		JobName:       b.meta.JobInfo.Name,
		ExecutorName:  b.opts.executorName,
		RunnerName:    b.opts.runnerName,
		StartedAt:     b.opts.startedAt.Format(time.RFC3339),
		SchemaVersion: schemaVersion,
	}

	for _, v := range b.meta.Variables {
		meta.Parameters = append(meta.Parameters, v.Key)
	}

	return meta
}

func (b *builder) buildCleanup(getSources stages.GetSources) stages.Cleanup {
	return stages.Cleanup{
		GitStrategy:       getSources.GitStrategy,
		SubmoduleStrategy: getSources.SubmoduleStrategy,
		GitCleanFlags:     getSources.GitCleanFlags,
		EnableJobCleanup:  b.isFeatureFlagOn(featureflags.EnableJobCleanup),
		CleanGitConfig:    b.opts.gitCleanConfig,
	}
}

func (b *builder) cacheKey(name string) (string, string, []string, error) {
	rawKey := path.Join(b.meta.JobInfo.Name, b.meta.GitInfo.Ref)
	if name != "" {
		rawKey = b.variables.ExpandValue(name)
	}

	var warnings []string
	var humanKey string
	if b.isFeatureFlagOn(featureflags.HashCacheKeys) {
		humanKey = rawKey
	} else {
		sanitized, err := cachekey.Sanitize(rawKey)
		switch {
		case err != nil:
			warnings = append(warnings, err.Error())
		case sanitized != rawKey:
			warnings = append(warnings, fmt.Sprintf("cache key %q sanitized to %q", rawKey, sanitized))
		}
		humanKey = sanitized
	}

	if humanKey == "" {
		return "", "", warnings, fmt.Errorf("empty cache key")
	}

	resolvedKey := humanKey
	if b.isFeatureFlagOn(featureflags.HashCacheKeys) {
		resolvedKey = fmt.Sprintf("%x", sha256.Sum256([]byte(humanKey)))
	}

	return humanKey, resolvedKey, warnings, nil
}

func (b *builder) isFeatureFlagOn(flag string) bool {
	if b.opts.isFeatureFlagOn != nil {
		return b.opts.isFeatureFlagOn(flag)
	}
	return variables.DefaultBool(b.variables, flag, false)
}

func (b *builder) splitVarFlags(varName string) []string {
	v := b.variables.Get(varName)
	if v == "" {
		return nil
	}
	return strings.Fields(v)
}

// splitVarFlagsDefault returns the split flags for varName, falling back to
// def when the variable is unset. The literal value "none" yields an empty
// slice, allowing users to opt out of any default flags.
func (b *builder) splitVarFlagsDefault(varName string, def []string) []string {
	v := b.variables.Get(varName)
	if v == "" {
		return def
	}
	if v == "none" {
		return nil
	}
	return strings.Fields(v)
}

package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/dns"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls"
	"gitlab.com/gitlab-org/gitlab-runner/referees"
	"gitlab.com/gitlab-org/gitlab-runner/session"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

type BuildRuntimeState string

const (
	BuildRunStatePending      BuildRuntimeState = "pending"
	BuildRunRuntimeRunning    BuildRuntimeState = "running"
	BuildRunRuntimeSuccess    BuildRuntimeState = "success"
	BuildRunRuntimeFailed     BuildRuntimeState = "failed"
	BuildRunRuntimeCanceled   BuildRuntimeState = "canceled"
	BuildRunRuntimeTerminated BuildRuntimeState = "terminated"
	BuildRunRuntimeTimedout   BuildRuntimeState = "timedout"
)

type BuildStage string

// WithContext is an interface that some Executor's ExecutorData will implement as a
// mechanism for extending the build context and canceling if the executor cannot
// complete the job. For example, the Autoscaler Executor will cancel the returned
// context if the instance backing the job disappears.
type WithContext interface {
	WithContext(context.Context) (context.Context, context.CancelFunc)
}

const (
	BuildStageResolveSecrets           BuildStage = "resolve_secrets"
	BuildStagePrepareExecutor          BuildStage = "prepare_executor"
	BuildStagePrepare                  BuildStage = "prepare_script"
	BuildStageGetSources               BuildStage = "get_sources"
	BuildStageClearWorktree            BuildStage = "clear_worktree"
	BuildStageRestoreCache             BuildStage = "restore_cache"
	BuildStageDownloadArtifacts        BuildStage = "download_artifacts"
	BuildStageAfterScript              BuildStage = "after_script"
	BuildStageArchiveOnSuccessCache    BuildStage = "archive_cache"
	BuildStageArchiveOnFailureCache    BuildStage = "archive_cache_on_failure"
	BuildStageUploadOnSuccessArtifacts BuildStage = "upload_artifacts_on_success"
	BuildStageUploadOnFailureArtifacts BuildStage = "upload_artifacts_on_failure"
	// We only renamed the variable name here as a first step to renaming the stage.
	// a separate issue will address changing the variable value, since it affects the
	// contract with the custom executor: https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28152.
	BuildStageCleanup BuildStage = "cleanup_file_variables"
)

// staticBuildStages is a list of BuildStages which are executed on every build
// and are not dynamically generated from steps.
var staticBuildStages = []BuildStage{
	BuildStagePrepare,
	BuildStageGetSources,
	BuildStageRestoreCache,
	BuildStageDownloadArtifacts,
	BuildStageAfterScript,
	BuildStageArchiveOnSuccessCache,
	BuildStageArchiveOnFailureCache,
	BuildStageUploadOnSuccessArtifacts,
	BuildStageUploadOnFailureArtifacts,
	BuildStageCleanup,
}

const (
	ExecutorJobSectionAttempts = "EXECUTOR_JOB_SECTION_ATTEMPTS"
)

// ErrSkipBuildStage is returned when there's nothing to be executed for the
// build stage.
var ErrSkipBuildStage = errors.New("skip build stage")

type Build struct {
	JobResponse `yaml:",inline"`

	SystemInterrupt  chan os.Signal `json:"-" yaml:"-"`
	RootDir          string         `json:"-" yaml:"-"`
	BuildDir         string         `json:"-" yaml:"-"`
	CacheDir         string         `json:"-" yaml:"-"`
	Hostname         string         `json:"-" yaml:"-"`
	Runner           *RunnerConfig  `json:"runner"`
	ExecutorData     ExecutorData
	ExecutorFeatures FeaturesInfo `json:"-" yaml:"-"`

	SafeDirectoryCheckout bool `json:"-" yaml:"-"`

	// Unique ID for all running builds on this runner
	RunnerID int `json:"runner_id"`

	// Unique ID for all running builds on this runner and this project
	ProjectRunnerID int `json:"project_runner_id"`

	// CurrentStage(), CurrentState() and CurrentExecutorStage() are called
	// from the metrics go routine whilst a build is in-flight, so access
	// to these variables requires a lock.
	statusLock            sync.Mutex
	currentStage          BuildStage
	currentState          BuildRuntimeState
	executorStageResolver func() ExecutorStage

	secretsResolver func(l logger, registry SecretResolverRegistry, featureFlagOn func(string) bool) (SecretsResolver, error)

	Session *session.Session

	logger buildlogger.Logger

	allVariables     JobVariables
	secretsVariables JobVariables
	buildSettings    *BuildSettings

	createdAt time.Time

	Referees         []referees.Referee
	ArtifactUploader func(config JobCredentials, reader io.ReadCloser, options ArtifactsOptions) (UploadState, string)
}

func (b *Build) setCurrentStage(stage BuildStage) {
	b.statusLock.Lock()
	defer b.statusLock.Unlock()

	b.currentStage = stage
}

func (b *Build) CurrentStage() BuildStage {
	b.statusLock.Lock()
	defer b.statusLock.Unlock()

	return b.currentStage
}

func (b *Build) setCurrentState(state BuildRuntimeState) {
	b.statusLock.Lock()
	defer b.statusLock.Unlock()

	b.currentState = state
}

func (b *Build) CurrentState() BuildRuntimeState {
	b.statusLock.Lock()
	defer b.statusLock.Unlock()

	return b.currentState
}

func (b *Build) Log() *logrus.Entry {
	return b.Runner.Log().WithField("job", b.ID).WithField("project", b.JobInfo.ProjectID)
}

func (b *Build) ProjectUniqueName() string {
	projectUniqueName := fmt.Sprintf(
		"runner-%s-project-%d-concurrent-%d",
		b.Runner.ShortDescription(),
		b.JobInfo.ProjectID,
		b.ProjectRunnerID,
	)

	return dns.MakeRFC1123Compatible(projectUniqueName)
}

func (b *Build) ProjectSlug() (string, error) {
	url, err := url.Parse(b.GitInfo.RepoURL)
	if err != nil {
		return "", err
	}
	if url.Host == "" {
		return "", errors.New("only URI reference supported")
	}

	slug := url.Path
	slug = strings.TrimSuffix(slug, ".git")
	slug = path.Clean(slug)
	if slug == "." {
		return "", errors.New("invalid path")
	}
	if strings.Contains(slug, "..") {
		return "", errors.New("it doesn't look like a valid path")
	}
	return slug, nil
}

func (b *Build) ProjectUniqueDir(sharedDir bool) string {
	dir, err := b.ProjectSlug()
	if err != nil {
		dir = fmt.Sprintf("project-%d", b.JobInfo.ProjectID)
	}

	// for shared dirs path is constructed like this:
	// <some-path>/runner-short-id/concurrent-id/group-name/project-name/
	// ex.<some-path>/01234567/0/group/repo/
	if sharedDir {
		dir = path.Join(
			b.Runner.ShortDescription(),
			fmt.Sprintf("%d", b.ProjectRunnerID),
			dir,
		)
	}
	return dir
}

func (b *Build) FullProjectDir() string {
	return helpers.ToSlash(b.BuildDir)
}

func (b *Build) TmpProjectDir() string {
	return helpers.ToSlash(b.BuildDir) + ".tmp"
}

// BuildStages returns a list of all BuildStages which will be executed.
// Not in the order of execution.
func (b *Build) BuildStages() []BuildStage {
	stages := make([]BuildStage, len(staticBuildStages))
	copy(stages, staticBuildStages)

	for _, s := range b.Steps {
		if s.Name == StepNameAfterScript {
			continue
		}

		stages = append(stages, StepToBuildStage(s))
	}

	return stages
}

func (b *Build) getCustomBuildDir(rootDir, dir string, customBuildDirEnabled, sharedDir bool) (string, error) {
	if dir == "" {
		return path.Join(rootDir, b.ProjectUniqueDir(sharedDir)), nil
	}

	if !customBuildDirEnabled {
		return "", MakeBuildError("setting GIT_CLONE_PATH is not allowed, enable `custom_build_dir` feature")
	}

	// See: https://gitlab.com/gitlab-org/gitlab-runner/-/issues/25913
	relDir, err := filepath.Rel(helpers.ToSlash(rootDir), helpers.ToSlash(dir))
	if err != nil {
		return "", &BuildError{Inner: err}
	}
	if strings.HasPrefix(relDir, "..") {
		return "", MakeBuildError("the GIT_CLONE_PATH=%q has to be within %q", dir, rootDir)
	}

	return path.Clean(dir), nil
}

func (b *Build) StartBuild(
	rootDir, cacheDir string,
	customBuildDirEnabled, sharedDir, safeDirectoryCheckout bool,
) error {
	if rootDir == "" {
		return MakeBuildError("the builds_dir is not configured")
	}

	if cacheDir == "" {
		return MakeBuildError("the cache_dir is not configured")
	}

	b.SafeDirectoryCheckout = safeDirectoryCheckout

	// We set RootDir and invalidate variables
	// to be able to use CI_BUILDS_DIR
	b.RootDir = rootDir
	b.CacheDir = path.Join(cacheDir, b.ProjectUniqueDir(false))
	b.RefreshAllVariables()

	var err error
	b.BuildDir, err = b.getCustomBuildDir(b.RootDir, b.Settings().GitClonePath, customBuildDirEnabled, sharedDir)
	if err != nil {
		return err
	}

	// We invalidate variables to be able to use
	// CI_CACHE_DIR and CI_PROJECT_DIR
	b.RefreshAllVariables()
	return nil
}

//nolint:funlen
func (b *Build) executeStage(ctx context.Context, buildStage BuildStage, executor Executor) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	b.setCurrentStage(buildStage)
	b.Log().WithField("build_stage", buildStage).Debug("Executing build stage")

	defer func() {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			b.logger.Warningln(
				string(buildStage) + " could not run to completion because the timeout was exceeded. " +
					"For more control over job and script timeouts see: " +
					"https://docs.gitlab.com/ee/ci/runners/configure_runners.html#set-script-and-after_script-timeouts")
		}
	}()

	shell := executor.Shell()
	if shell == nil {
		return errors.New("no shell defined")
	}

	script, err := GenerateShellScript(ctx, buildStage, *shell)
	if errors.Is(err, ErrSkipBuildStage) {
		if b.IsFeatureFlagOn(featureflags.SkipNoOpBuildStages) {
			b.Log().WithField("build_stage", buildStage).Debug("Skipping stage (nothing to do)")
			return nil
		}

		err = nil
	}

	if err != nil {
		return err
	}

	// Nothing to execute
	if script == "" {
		return nil
	}

	cmd := ExecutorCommand{
		Context:    ctx,
		Script:     script,
		Stage:      buildStage,
		Predefined: getPredefinedEnv(buildStage),
	}

	section := helpers.BuildSection{
		Name:        string(buildStage),
		SkipMetrics: !b.JobResponse.Features.TraceSections,
		Run: func() error {
			msg := fmt.Sprintf(
				"%s%s%s",
				helpers.ANSI_BOLD_CYAN,
				GetStageDescription(buildStage),
				helpers.ANSI_RESET,
			)
			b.logger.Println(msg)
			return executor.Run(cmd)
		},
	}

	return section.Execute(&b.logger)
}

// getPredefinedEnv returns whether a stage should be executed on
// the predefined environment that GitLab Runner provided.
func getPredefinedEnv(buildStage BuildStage) bool {
	env := map[BuildStage]bool{
		BuildStagePrepare:                  true,
		BuildStageGetSources:               true,
		BuildStageClearWorktree:            true,
		BuildStageRestoreCache:             true,
		BuildStageDownloadArtifacts:        true,
		BuildStageAfterScript:              false,
		BuildStageArchiveOnSuccessCache:    true,
		BuildStageArchiveOnFailureCache:    true,
		BuildStageUploadOnFailureArtifacts: true,
		BuildStageUploadOnSuccessArtifacts: true,
		BuildStageCleanup:                  true,
	}

	predefined, ok := env[buildStage]
	if !ok {
		return false
	}

	return predefined
}

func GetStageDescription(stage BuildStage) string {
	descriptions := map[BuildStage]string{
		BuildStagePrepare:                  "Preparing environment",
		BuildStageGetSources:               "Getting source from Git repository",
		BuildStageClearWorktree:            "Deleting all tracked and untracked files due to source fetch failure",
		BuildStageRestoreCache:             "Restoring cache",
		BuildStageDownloadArtifacts:        "Downloading artifacts",
		BuildStageAfterScript:              "Running after_script",
		BuildStageArchiveOnSuccessCache:    "Saving cache for successful job",
		BuildStageArchiveOnFailureCache:    "Saving cache for failed job",
		BuildStageUploadOnFailureArtifacts: "Uploading artifacts for failed job",
		BuildStageUploadOnSuccessArtifacts: "Uploading artifacts for successful job",
		BuildStageCleanup:                  "Cleaning up project directory and file based variables",
	}

	description, ok := descriptions[stage]
	if !ok {
		return fmt.Sprintf("Executing %q stage of the job script", stage)
	}

	return description
}

func (b *Build) executeUploadArtifacts(ctx context.Context, state error, executor Executor) (err error) {
	if state == nil {
		return b.executeStage(ctx, BuildStageUploadOnSuccessArtifacts, executor)
	}

	return b.executeStage(ctx, BuildStageUploadOnFailureArtifacts, executor)
}

func (b *Build) executeArchiveCache(ctx context.Context, state error, executor Executor) (err error) {
	if state == nil {
		return b.executeStage(ctx, BuildStageArchiveOnSuccessCache, executor)
	}

	return b.executeStage(ctx, BuildStageArchiveOnFailureCache, executor)
}

//nolint:funlen,gocognit
func (b *Build) executeScript(ctx context.Context, trace JobTrace, executor Executor) error {
	// track job start and create referees
	startTime := time.Now()
	b.createReferees(executor)

	// Prepare stage
	err := b.executeStage(ctx, BuildStagePrepare, executor)
	if err != nil {
		return fmt.Errorf(
			"prepare environment: %w. "+
				"Check https://docs.gitlab.com/runner/shells/index.html#shell-profile-loading for more information",
			err,
		)
	}

	err = b.attemptGetSourcesStage(ctx, executor, b.GetGetSourcesAttempts())

	if err == nil {
		err = b.attemptExecuteStage(ctx, BuildStageRestoreCache, executor, b.GetRestoreCacheAttempts())
	}
	if err == nil {
		err = b.attemptExecuteStage(ctx, BuildStageDownloadArtifacts, executor, b.GetDownloadArtifactsAttempts())
	}

	//nolint:nestif
	if err == nil {
		timeouts := b.getStageTimeoutContexts(ctx,
			stageTimeout{"RUNNER_SCRIPT_TIMEOUT", 0},
			stageTimeout{"RUNNER_AFTER_SCRIPT_TIMEOUT", AfterScriptTimeout})

		scriptCtx, cancel := timeouts["RUNNER_SCRIPT_TIMEOUT"]()
		defer cancel()

		// update trace's cancel function so that the main script can be cancelled,
		// with after_script and later stages to still complete.
		trace.SetCancelFunc(cancel)

		for _, s := range b.Steps {
			// after_script has a separate BuildStage. See common.BuildStageAfterScript
			if s.Name == StepNameAfterScript {
				continue
			}
			err = b.executeStage(scriptCtx, StepToBuildStage(s), executor)
			if err != nil {
				break
			}
		}

		// if parent context is fine but script context was cancelled we ensure the build error
		// failure reason is "canceled".
		if ctx.Err() == nil && errors.Is(scriptCtx.Err(), context.Canceled) {
			err = &BuildError{
				Inner:         errors.New("canceled"),
				FailureReason: JobCanceled,
			}

			b.logger.Warningln("script canceled externally (UI, API)")
		}

		afterScriptCtx, cancel := timeouts["RUNNER_AFTER_SCRIPT_TIMEOUT"]()
		defer cancel()

		if afterScriptErr := b.executeAfterScript(afterScriptCtx, err, executor); afterScriptErr != nil {
			if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
				// the parent deadline being exceeded is reported at a later stage, so we
				// only focus on errors specific to after_script here
				b.logger.Warningln("after_script failed, but job will continue unaffected:", afterScriptErr)
			}
		}

		// If the parent context reached deadline, don't do anything different than usual.
		// If the script context reached deadline, return the deadline error.
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) && errors.Is(scriptCtx.Err(), context.DeadlineExceeded) {
			err = &BuildError{
				Inner:         fmt.Errorf("script timeout context: %w", scriptCtx.Err()),
				FailureReason: JobExecutionTimeout,
			}
		}
	}

	archiveCacheErr := b.executeArchiveCache(ctx, err, executor)

	artifactUploadErr := b.executeUploadArtifacts(ctx, err, executor)

	// track job end and execute referees
	endTime := time.Now()
	b.executeUploadReferees(ctx, startTime, endTime)

	b.removeFileBasedVariables(ctx, executor)

	return b.pickPriorityError(err, archiveCacheErr, artifactUploadErr)
}

func (b *Build) pickPriorityError(jobErr error, archiveCacheErr error, artifactUploadErr error) error {
	// Use job's errors which came before upload errors as most important to surface
	if jobErr != nil {
		return jobErr
	}

	// Otherwise, use uploading errors
	if archiveCacheErr != nil {
		return archiveCacheErr
	}

	return artifactUploadErr
}

func (b *Build) executeAfterScript(ctx context.Context, err error, executor Executor) error {
	state, _ := b.runtimeStateAndError(err)
	b.GetAllVariables().OverwriteKey("CI_JOB_STATUS", JobVariable{
		Key:   "CI_JOB_STATUS",
		Value: string(state),
	})

	return b.executeStage(ctx, BuildStageAfterScript, executor)
}

// StepToBuildStage returns the BuildStage corresponding to a step.
func StepToBuildStage(s Step) BuildStage {
	return BuildStage(fmt.Sprintf("step_%s", strings.ToLower(string(s.Name))))
}

func (b *Build) createReferees(executor Executor) {
	b.Referees = referees.CreateReferees(executor, b.Runner.Referees, b.Log())
}

func (b *Build) removeFileBasedVariables(ctx context.Context, executor Executor) {
	err := b.executeStage(ctx, BuildStageCleanup, executor)
	if err != nil {
		b.Log().WithError(err).Warning("Error while executing file based variables removal script")
	}
}

func (b *Build) executeUploadReferees(ctx context.Context, startTime, endTime time.Time) {
	if b.Referees == nil || b.ArtifactUploader == nil {
		b.Log().Debug("Skipping referees execution")
		return
	}

	jobCredentials := JobCredentials{
		ID:    b.JobResponse.ID,
		Token: b.JobResponse.Token,
		URL:   b.Runner.RunnerCredentials.URL,
	}

	// execute and upload the results of each referee
	for _, referee := range b.Referees {
		if referee == nil {
			continue
		}

		reader, err := referee.Execute(ctx, startTime, endTime)
		// keep moving even if a subset of the referees have failed
		if err != nil {
			continue
		}

		// referee ran successfully, upload its results to GitLab as an artifact
		b.ArtifactUploader(jobCredentials, io.NopCloser(reader), ArtifactsOptions{
			BaseName: referee.ArtifactBaseName(),
			Type:     referee.ArtifactType(),
			Format:   ArtifactFormat(referee.ArtifactFormat()),
		})
	}
}

func (b *Build) attemptGetSourcesStage(
	ctx context.Context,
	executor Executor,
	attempts int,
) (err error) {
	if attempts < 1 || attempts > 10 {
		return fmt.Errorf("number of attempts out of the range [1, 10] for stage: %s", BuildStageGetSources)
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt == 1 {
			// If GetSources fails we delete all tracked and untracked files. This is
			// because Git's submodule support has various bugs that cause fetches to
			// fail if submodules have changed.
			if err = b.executeStage(ctx, BuildStageClearWorktree, executor); err != nil {
				continue
			}
		}
		if err = b.executeStage(ctx, BuildStageGetSources, executor); err == nil {
			return
		}
	}
	return
}

func (b *Build) attemptExecuteStage(
	ctx context.Context,
	buildStage BuildStage,
	executor Executor,
	attempts int,
) (err error) {
	if attempts < 1 || attempts > 10 {
		return fmt.Errorf("number of attempts out of the range [1, 10] for stage: %s", buildStage)
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if err = b.executeStage(ctx, buildStage, executor); err == nil {
			return
		}
	}
	return
}

func (b *Build) GetBuildTimeout() time.Duration {
	buildTimeout := b.RunnerInfo.Timeout
	if buildTimeout <= 0 {
		buildTimeout = DefaultTimeout
	}
	return time.Duration(buildTimeout) * time.Second
}

func (b *Build) handleError(err error) error {
	state, err := b.runtimeStateAndError(err)
	b.setCurrentState(state)

	return err
}

func (b *Build) runtimeStateAndError(err error) (BuildRuntimeState, error) {
	switch {
	case errors.Is(err, context.Canceled):
		return BuildRunRuntimeCanceled, &BuildError{
			Inner:         errors.New("canceled"),
			FailureReason: JobCanceled,
		}

	case errors.Is(err, context.DeadlineExceeded):
		return BuildRunRuntimeTimedout, &BuildError{
			Inner:         fmt.Errorf("execution took longer than %v seconds", b.GetBuildTimeout()),
			FailureReason: JobExecutionTimeout,
		}

	case err == nil:
		return BuildRunRuntimeSuccess, nil

	default:
		return BuildRunRuntimeFailed, err
	}
}

//nolint:funlen
func (b *Build) run(ctx context.Context, trace JobTrace, executor Executor) (err error) {
	b.setCurrentState(BuildRunRuntimeRunning)

	buildFinish := make(chan error, 1)
	buildPanic := make(chan error, 1)

	runContext, runCancel := context.WithCancel(ctx)
	defer runCancel()

	if term, ok := executor.(terminal.InteractiveTerminal); b.Session != nil && ok {
		b.Session.SetInteractiveTerminal(term)
	}

	if proxyPooler, ok := executor.(proxy.Pooler); b.Session != nil && ok {
		b.Session.SetProxyPool(proxyPooler)
	}

	// Run build script
	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := &BuildError{FailureReason: RunnerSystemFailure, Inner: fmt.Errorf("panic: %s", r)}

				b.Log().WithError(err).Error(string(debug.Stack()))
				buildPanic <- err
			}
		}()

		buildFinish <- b.executeScript(runContext, trace, executor)
	}()

	// Wait for signals: cancel, timeout, abort or finish
	b.Log().Debugln("Waiting for signals...")
	select {
	case <-ctx.Done():
		err = b.handleError(context.Cause(ctx))

	case signal := <-b.SystemInterrupt:
		err = &BuildError{
			Inner:         fmt.Errorf("aborted: %v", signal),
			FailureReason: RunnerSystemFailure,
		}
		b.setCurrentState(BuildRunRuntimeTerminated)

	case err = <-buildFinish:
		// It's possible that the parent context being cancelled will
		// terminate the build early, bringing us here, and although we handle
		// `ctx.Done()` above, select statements are not ordered.
		// We handle this the same as if we received ctx.Done(), but
		// return early because we're no longer waiting for the build
		// to finish.
		if ctx.Err() != nil {
			return b.handleError(context.Cause(ctx))
		}

		if err != nil {
			b.setCurrentState(BuildRunRuntimeFailed)
		} else {
			b.setCurrentState(BuildRunRuntimeSuccess)
		}
		return err

	case err = <-buildPanic:
		b.setCurrentState(BuildRunRuntimeTerminated)
		return err
	}

	b.Log().WithError(err).Debugln("Waiting for build to finish...")

	// Wait till we receive that build did finish
	runCancel()
	b.waitForBuildFinish(buildFinish, WaitForBuildFinishTimeout)

	return err
}

// waitForBuildFinish will wait for the build to finish or timeout, whichever
// comes first. This is to prevent issues where something in the build can't be
// killed or processed and results into the Job running until the GitLab Runner
// process exists.
func (b *Build) waitForBuildFinish(buildFinish <-chan error, timeout time.Duration) {
	select {
	case <-buildFinish:
		return
	case <-time.After(timeout):
		b.logger.Warningln("Timed out waiting for the build to finish")
		return
	}
}

func (b *Build) retryCreateExecutor(
	options ExecutorPrepareOptions,
	provider ExecutorProvider,
	logger buildlogger.Logger,
) (Executor, error) {
	var err error

	for tries := 0; tries < PreparationRetries; tries++ {
		executor := provider.Create()
		if executor == nil {
			return nil, errors.New("failed to create executor")
		}

		b.setExecutorStageResolver(executor.GetCurrentStage)

		err = executor.Prepare(options)
		if err == nil {
			return executor, nil
		}
		executor.Cleanup()
		var buildErr *BuildError
		if errors.As(err, &buildErr) {
			return nil, err
		} else if options.Context.Err() != nil {
			return nil, b.handleError(context.Cause(options.Context))
		}

		logger.SoftErrorln("Preparation failed:", err)
		logger.Infoln("Will be retried in", PreparationRetryInterval, "...")
		time.Sleep(PreparationRetryInterval)
	}

	return nil, err
}

func (b *Build) waitForTerminal(ctx context.Context, timeout time.Duration) error {
	if b.Session == nil || !b.Session.Connected() {
		return nil
	}

	timeout = b.getTerminalTimeout(ctx, timeout)

	b.logger.Infoln(
		fmt.Sprintf(
			"Terminal is connected, will time out in %s...",
			timeout.Round(time.Second),
		),
	)

	select {
	case <-ctx.Done():
		err := b.Session.Kill()
		if err != nil {
			b.Log().WithError(err).Warn("Failed to kill session")
		}
		return errors.New("build cancelled, killing session")
	case <-time.After(timeout):
		err := fmt.Errorf(
			"terminal session timed out (maximum time allowed - %s)",
			timeout.Round(time.Second),
		)
		b.logger.Infoln(err.Error())
		b.Session.TimeoutCh <- err
		return err
	case err := <-b.Session.DisconnectCh:
		b.logger.Infoln("Terminal disconnected")
		return fmt.Errorf("terminal disconnected: %w", err)
	case signal := <-b.SystemInterrupt:
		b.logger.Infoln("Terminal disconnected")
		err := b.Session.Kill()
		if err != nil {
			b.Log().WithError(err).Warn("Failed to kill session")
		}
		return fmt.Errorf("terminal disconnected by system signal: %v", signal)
	}
}

// getTerminalTimeout checks if the the job timeout comes before the
// configured terminal timeout.
func (b *Build) getTerminalTimeout(ctx context.Context, timeout time.Duration) time.Duration {
	expiryTime, _ := ctx.Deadline()

	if expiryTime.Before(time.Now().Add(timeout)) {
		timeout = time.Until(expiryTime)
	}

	return timeout
}

// setTraceStatus sets the final status of a job. If the err
// is nil, the job is successful.
//
// What we send back to GitLab for a failure reason when the err
// is not nil depends:
//
// If the error can be unwrapped to `BuildError`, the BuildError's
// failure reason is given. If the failure reason is not supported
// by GitLab, it's converted to an `UnknownFailure`. If the failure
// reason is not specified, `ScriptFailure` is used.
//
// If an error cannot be unwrapped to `BuildError`, `SystemFailure`
// is used as the failure reason.
func (b *Build) setTraceStatus(trace JobTrace, err error) {
	logger := b.logger.WithFields(logrus.Fields{
		"duration_s": b.Duration().Seconds(),
	})

	if err == nil {
		logger.Infoln("Job succeeded")
		trace.Success()

		return
	}

	var buildError *BuildError
	if errors.As(err, &buildError) {
		msg := fmt.Sprintln("Job failed:", err)
		if buildError.FailureReason == RunnerSystemFailure {
			msg = fmt.Sprintln("Job failed (system failure):", err)
		}

		logger.SoftErrorln(msg)

		trace.SetSupportedFailureReasonMapper(newFailureReasonMapper(b.Features.FailureReasons))
		trace.Fail(err, JobFailureData{
			Reason:   buildError.FailureReason,
			ExitCode: buildError.ExitCode,
		})

		return
	}

	logger.Errorln("Job failed (system failure):", err)
	trace.Fail(err, JobFailureData{Reason: RunnerSystemFailure})
}

func (b *Build) setExecutorStageResolver(resolver func() ExecutorStage) {
	b.statusLock.Lock()
	defer b.statusLock.Unlock()

	b.executorStageResolver = resolver
}

func (b *Build) CurrentExecutorStage() ExecutorStage {
	b.statusLock.Lock()
	defer b.statusLock.Unlock()

	if b.executorStageResolver == nil {
		return ExecutorStage("")
	}

	return b.executorStageResolver()
}

//nolint:funlen
func (b *Build) Run(globalConfig *Config, trace JobTrace) (err error) {
	b.logUsedImages()

	b.logger = buildlogger.New(trace, b.Log())
	b.printRunningWithHeader()
	b.setCurrentState(BuildRunStatePending)

	// These defers are ordered because runBuild could panic and the recover needs to handle that panic.
	// setTraceStatus needs to be last since it needs a correct error value to report the job's status
	defer func() { b.setTraceStatus(trace, err) }()

	defer func() {
		if r := recover(); r != nil {
			err = &BuildError{FailureReason: RunnerSystemFailure, Inner: fmt.Errorf("panic: %s", r)}

			b.Log().WithError(err).Error(string(debug.Stack()))
		}
	}()

	err = b.resolveSecrets()
	if err != nil {
		return err
	}

	b.expandContainerOptions()

	ctx, cancel := context.WithTimeout(context.Background(), b.GetBuildTimeout())
	defer cancel()

	b.configureTrace(trace, cancel)

	b.printSettingErrors()

	options := b.createExecutorPrepareOptions(ctx, globalConfig)
	provider := GetExecutorProvider(b.Runner.Executor)
	if provider == nil {
		return errors.New("executor not found")
	}

	err = provider.GetFeatures(&b.ExecutorFeatures)
	if err != nil {
		return fmt.Errorf("retrieving executor features: %w", err)
	}

	executor, err := b.executeBuildSection(options, provider)
	if err != nil {
		return err
	}
	defer executor.Cleanup()

	// override context that can be canceled by the executor if supported
	if withContext, ok := b.ExecutorData.(WithContext); ok {
		ctx, cancel = withContext.WithContext(ctx)
		defer cancel()
	}

	err = b.run(ctx, trace, executor)
	if errWait := b.waitForTerminal(ctx, globalConfig.SessionServer.GetSessionTimeout()); errWait != nil {
		b.Log().WithError(errWait).Debug("Stopped waiting for terminal")
	}
	executor.Finish(err)

	return err
}

func (b *Build) logUsedImages() {
	if !b.IsFeatureFlagOn(featureflags.LogImagesConfiguredForJob) {
		return
	}

	imageFields := b.JobResponse.Image.LogFields()
	if imageFields != nil {
		b.Log().WithFields(imageFields).Info("Image configured for job")
	}

	for _, service := range b.JobResponse.Services {
		b.Log().WithFields(service.LogFields()).Info("Service image configured for job")
	}
}

func (b *Build) configureTrace(trace JobTrace, cancel context.CancelFunc) {
	trace.SetCancelFunc(cancel)
	trace.SetAbortFunc(cancel)
	trace.SetMasked(MaskOptions{
		Phrases:       b.GetAllVariables().Masked(),
		TokenPrefixes: b.JobResponse.Features.TokenMaskPrefixes,
	})
}

func (b *Build) createExecutorPrepareOptions(ctx context.Context, globalConfig *Config) ExecutorPrepareOptions {
	return ExecutorPrepareOptions{
		Config:      b.Runner,
		Build:       b,
		BuildLogger: b.logger,
		User:        globalConfig.User,
		Context:     ctx,
	}
}

func (b *Build) resolveSecrets() error {
	if b.Secrets == nil {
		return nil
	}

	b.Secrets.expandVariables(b.GetAllVariables())

	section := helpers.BuildSection{
		Name:        string(BuildStageResolveSecrets),
		SkipMetrics: !b.JobResponse.Features.TraceSections,
		Run: func() error {
			resolver, err := b.secretsResolver(&b.logger, GetSecretResolverRegistry(), b.IsFeatureFlagOn)
			if err != nil {
				return fmt.Errorf("creating secrets resolver: %w", err)
			}

			variables, err := resolver.Resolve(b.Secrets)
			if err != nil {
				return fmt.Errorf("resolving secrets: %w", err)
			}

			b.secretsVariables = variables
			b.RefreshAllVariables()

			return nil
		},
	}

	return section.Execute(&b.logger)
}

func (b *Build) executeBuildSection(options ExecutorPrepareOptions, provider ExecutorProvider) (Executor, error) {
	var executor Executor
	var err error
	section := helpers.BuildSection{
		Name:        string(BuildStagePrepareExecutor),
		SkipMetrics: !b.JobResponse.Features.TraceSections,
		Run: func() error {
			msg := fmt.Sprintf(
				"%sPreparing the %q executor%s",
				helpers.ANSI_BOLD_CYAN,
				b.Runner.Executor,
				helpers.ANSI_RESET,
			)
			b.logger.Println(msg)
			executor, err = b.retryCreateExecutor(options, provider, b.logger)
			return err
		},
	}
	err = section.Execute(&b.logger)
	return executor, err
}

func (b *Build) String() string {
	return helpers.ToYAML(b)
}

func (b *Build) platformAppropriatePath(s string) string {
	// Check if we're dealing with a Windows path on a Windows platform
	// filepath.VolumeName will return empty otherwise
	if filepath.VolumeName(s) != "" {
		return filepath.FromSlash(s)
	}
	return s
}

func (b *Build) GetDefaultVariables() JobVariables {
	return JobVariables{
		{
			Key:      "CI_BUILDS_DIR",
			Value:    b.platformAppropriatePath(b.RootDir),
			Public:   true,
			Internal: true,
			File:     false,
		},
		{
			Key:      "CI_PROJECT_DIR",
			Value:    b.platformAppropriatePath(b.FullProjectDir()),
			Public:   true,
			Internal: true,
			File:     false,
		},
		{
			Key:      "CI_CONCURRENT_ID",
			Value:    strconv.Itoa(b.RunnerID),
			Public:   true,
			Internal: true,
			File:     false,
		},
		{
			Key:      "CI_CONCURRENT_PROJECT_ID",
			Value:    strconv.Itoa(b.ProjectRunnerID),
			Public:   true,
			Internal: true,
			File:     false,
		},
		{
			Key:      "CI_SERVER",
			Value:    "yes",
			Public:   true,
			Internal: true,
			File:     false,
		},
		{
			Key:      "CI_JOB_STATUS",
			Value:    string(BuildRunRuntimeRunning),
			Public:   true,
			Internal: true,
		},
		{
			Key:      "CI_JOB_TIMEOUT",
			Value:    strconv.FormatInt(int64(b.GetBuildTimeout().Seconds()), 10),
			Public:   true,
			Internal: true,
			File:     false,
		},
	}
}

func (b *Build) GetDefaultFeatureFlagsVariables() JobVariables {
	variables := make(JobVariables, 0)
	for _, featureFlag := range featureflags.GetAll() {
		variables = append(variables, JobVariable{
			Key:      featureFlag.Name,
			Value:    strconv.FormatBool(featureFlag.DefaultValue),
			Public:   true,
			Internal: true,
			File:     false,
		})
	}

	return variables
}

func (b *Build) GetSharedEnvVariable() JobVariable {
	env := JobVariable{Value: "true", Public: true, Internal: true, File: false}
	if b.IsSharedEnv() {
		env.Key = "CI_SHARED_ENVIRONMENT"
	} else {
		env.Key = "CI_DISPOSABLE_ENVIRONMENT"
	}

	return env
}

func (b *Build) GetCITLSVariables() JobVariables {
	variables := JobVariables{}

	if b.TLSCAChain != "" {
		variables = append(variables, JobVariable{
			Key:      tls.VariableCAFile,
			Value:    b.TLSCAChain,
			Public:   true,
			Internal: true,
			File:     true,
		})
	}

	if b.TLSAuthCert != "" && b.TLSAuthKey != "" {
		variables = append(
			variables,
			JobVariable{
				Key:      tls.VariableCertFile,
				Value:    b.TLSAuthCert,
				Public:   true,
				Internal: true,
				File:     true,
			},
			JobVariable{
				Key:      tls.VariableKeyFile,
				Value:    b.TLSAuthKey,
				Internal: true,
				File:     true,
			},
		)
	}

	return variables
}

func (b *Build) IsSharedEnv() bool {
	return b.ExecutorFeatures.Shared
}

// RefreshAllVariables forces the next time all variables are retrieved to discard
// any cached results and reconstruct/expand all job variables.
func (b *Build) RefreshAllVariables() {
	b.allVariables = nil
	b.buildSettings = nil
}

func (b *Build) GetAllVariables() JobVariables {
	if b.allVariables != nil {
		return b.allVariables
	}

	variables := make(JobVariables, 0)
	variables = append(variables, b.GetDefaultFeatureFlagsVariables()...)
	if b.Image.Name != "" {
		variables = append(
			variables,
			JobVariable{Key: "CI_JOB_IMAGE", Value: b.Image.Name, Public: true, Internal: true, File: false},
		)
	}
	if b.Runner != nil {
		variables = append(variables, b.Runner.GetVariables()...)
	}
	variables = append(variables, b.GetDefaultVariables()...)
	variables = append(variables, b.GetCITLSVariables()...)
	variables = append(variables, b.Variables...)
	variables = append(variables, b.GetSharedEnvVariable())
	variables = append(variables, AppVersion.Variables()...)
	variables = append(variables, b.secretsVariables...)

	variables = append(variables, JobVariable{
		Key: tempProjectDirVariableKey, Value: b.TmpProjectDir(), Public: true, Internal: true,
	})

	b.allVariables = variables.Expand()

	return b.allVariables
}

// Users might specify image and service-image name and aliases as Variables, so we must expand them before they are
// used.
func (b *Build) expandContainerOptions() {
	allVars := b.GetAllVariables()
	b.Image.Name = allVars.ExpandValue(b.Image.Name)
	b.Image.Alias = allVars.ExpandValue(b.Image.Alias)
	for i := range b.Services {
		b.Services[i].Name = allVars.ExpandValue(b.Services[i].Name)
		b.Services[i].Alias = allVars.ExpandValue(b.Services[i].Alias)
	}
}

func (b *Build) getURLWithAuth(URL string) string {
	u, _ := url.Parse(URL)

	if u.Scheme == "ssh" {
		if u.User == nil {
			u.User = url.User("git")
		}
	} else {
		u.User = url.UserPassword("gitlab-ci-token", b.Token)
	}

	return u.String()
}

// GetRemoteURL checks if the default clone URL is overwritten by the runner
// configuration option: 'CloneURL'. If it is, we use that to create the clone
// URL.
func (b *Build) GetRemoteURL() string {
	u, _ := url.Parse(b.Runner.CloneURL)

	if u == nil || u.Scheme == "" {
		return b.GitInfo.RepoURL
	}

	projectPath := b.GetAllVariables().Value("CI_PROJECT_PATH") + ".git"
	u.Path = path.Join(u.Path, projectPath)

	return b.getURLWithAuth(u.String())
}

func (b *Build) getBaseURL() string {
	u, _ := url.Parse(b.Runner.CloneURL)

	if u == nil || u.Scheme == "" {
		return b.Runner.RunnerCredentials.URL
	}

	return u.String()
}

func (b *Build) getURLInsteadOf(target string, source string) []string {
	return []string{"-c", fmt.Sprintf("url.%s.insteadOf=%s", target, source)}
}

// GetURLInsteadOfArgs rewrites a plain HTTPS base URL and the most commonly used SSH/Git
// protocol URLs (including custom SSH ports) into an HTTPS URL with injected job token
// auth, and returns an array of strings to pass as options to git commands.
func (b *Build) GetURLInsteadOfArgs() []string {
	baseURL := strings.TrimRight(b.getBaseURL(), "/")
	if !strings.HasPrefix(baseURL, "http") {
		return []string{}
	}

	baseURLWithAuth := b.getURLWithAuth(baseURL)

	// https://example.com/ 		-> https://gitlab-ci-token:abc123@example.com/
	args := b.getURLInsteadOf(baseURLWithAuth, baseURL)

	if b.Settings().GitSubmoduleForceHTTPS {
		ciServerPort := b.GetAllVariables().Value("CI_SERVER_SHELL_SSH_PORT")
		ciServerHost := b.GetAllVariables().Value("CI_SERVER_SHELL_SSH_HOST")
		if ciServerHost == "" {
			ciServerHost = b.GetAllVariables().Value("CI_SERVER_HOST")
		}

		if ciServerPort == "" || ciServerPort == "22" {
			// git@example.com: 		-> https://gitlab-ci-token:abc123@example.com/
			baseGitURL := fmt.Sprintf("git@%s:", ciServerHost)

			args = append(args, b.getURLInsteadOf(baseURLWithAuth+"/", baseGitURL)...)
			// ssh://git@example.com/ 	-> https://gitlab-ci-token:abc123@example.com/
			baseSSHGitURL := fmt.Sprintf("ssh://git@%s", ciServerHost)
			args = append(args, b.getURLInsteadOf(baseURLWithAuth, baseSSHGitURL)...)
		} else {
			// ssh://git@example.com:8022/ 	-> https://gitlab-ci-token:abc123@example.com/
			baseSSHGitURLWithPort := fmt.Sprintf("ssh://git@%s:%s", ciServerHost, ciServerPort)
			args = append(args, b.getURLInsteadOf(baseURLWithAuth, baseSSHGitURLWithPort)...)
		}
	}
	return args
}

type stageTimeout struct {
	configName     string
	defaultTimeout time.Duration
}

func (b *Build) getStageTimeoutContexts(parent context.Context, timeouts ...stageTimeout) map[string]func() (context.Context, func()) {
	stack := make([]time.Duration, len(timeouts))

	deadline, hasDeadline := parent.Deadline()
	jobTimeout := time.Until(deadline)
	for idx, timeout := range timeouts {
		stack[idx] = timeout.defaultTimeout

		rawTimeout := b.GetAllVariables().Value(timeout.configName)
		duration, parseErr := time.ParseDuration(rawTimeout)

		switch {
		case strings.TrimSpace(rawTimeout) == "":
			// no-op

		case parseErr != nil:
			b.logger.Warningln(fmt.Sprintf("Ignoring malformed %s timeout: %v", timeout.configName, rawTimeout))

		case duration < 0:
			// no relative durations for now...
			b.logger.Warningln(fmt.Sprintf("Ignoring relative %s timeout: %v", timeout.configName, rawTimeout))

		case hasDeadline && duration > jobTimeout:
			// clamping timeouts to the job timeout happens automatically in `context.WithParent()`, mention it here
			b.logger.Warningln(fmt.Sprintf("%s timeout: %v is longer than job timeout. Setting to job timeout", timeout.configName, rawTimeout))

		case duration != 0:
			stack[idx] = duration
		}
	}

	results := make(map[string]func() (context.Context, func()))
	for idx, timeout := range timeouts {
		switch {
		case stack[idx] == 0:
			results[timeout.configName] = func() (context.Context, func()) {
				// no timeout
				return context.WithCancel(parent)
			}

		case stack[idx] > 0:
			duration := stack[idx]
			results[timeout.configName] = func() (context.Context, func()) {
				// absolute timeout
				return context.WithTimeout(parent, duration)
			}
		}
	}

	return results
}

func (b *Build) GetGitStrategy() GitStrategy {
	return b.Settings().GitStrategy
}

func (b *Build) GetRepositoryObjectFormat() string {
	if b.GitInfo.RepoObjectFormat == "" {
		return DefaultObjectFormat
	}

	return b.GitInfo.RepoObjectFormat
}

func (b *Build) GetGitCheckout() bool {
	if b.GetGitStrategy() == GitNone {
		return false
	}

	return b.Settings().GitCheckout
}

func (b *Build) GetSubmoduleStrategy() SubmoduleStrategy {
	if b.GetGitStrategy() == GitNone {
		return SubmoduleNone
	}

	return b.Settings().GitSubmoduleStrategy
}

// GetSubmodulePaths https://git-scm.com/docs/git-submodule#Documentation/git-submodule.txt-ltpathgt82308203
func (b *Build) GetSubmodulePaths() ([]string, error) {
	toks := b.Settings().GitSubmodulePaths
	for _, tok := range toks {
		if tok == ":(exclude)" {
			return nil, fmt.Errorf("GIT_SUBMODULE_PATHS: invalid submodule pathspec %q", toks)
		}
	}
	return toks, nil
}

func (b *Build) GetSubmoduleDepth() int {
	return b.Settings().GitSubmoduleDepth
}

func (b *Build) GetGitCleanFlags() []string {
	return b.Settings().GitCleanFlags
}

func (b *Build) GetGitFetchFlags() []string {
	return b.Settings().GitFetchExtraFlags
}

func (b *Build) GetGitSubmoduleUpdateFlags() []string {
	return b.Settings().GitSubmoduleUpdateFlags
}

func (b *Build) IsDebugTraceEnabled() bool {
	return b.Settings().CIDebugTrace
}

func (b *Build) GetDockerAuthConfig() string {
	return b.Settings().DockerAuthConfig
}

func (b *Build) GetGetSourcesAttempts() int {
	return b.Settings().GetSourcesAttempts
}

func (b *Build) GetDownloadArtifactsAttempts() int {
	return b.Settings().ArtifactDownloadAttempts
}

func (b *Build) GetRestoreCacheAttempts() int {
	return b.Settings().RestoreCacheAttempts
}

func (b *Build) GetCacheRequestTimeout() int {
	return b.Settings().CacheRequestTimeout
}

func (b *Build) GetExecutorJobSectionAttempts() int {
	return b.Settings().ExecutorJobSectionAttempts
}

func (b *Build) StartedAt() time.Time {
	return b.createdAt
}

func (b *Build) Duration() time.Duration {
	return time.Since(b.createdAt)
}

func NewBuild(
	jobData JobResponse,
	runnerConfig *RunnerConfig,
	systemInterrupt chan os.Signal,
	executorData ExecutorData,
) (*Build, error) {
	// Attempt to perform a deep copy of the RunnerConfig
	runnerConfigCopy, err := runnerConfig.DeepCopy()
	if err != nil {
		return nil, fmt.Errorf("deep copy of runner config failed: %w", err)
	}

	return &Build{
		JobResponse:     jobData,
		Runner:          runnerConfigCopy,
		SystemInterrupt: systemInterrupt,
		ExecutorData:    executorData,
		createdAt:       time.Now(),
		secretsResolver: newSecretsResolver,
	}, nil
}

func (b *Build) IsFeatureFlagOn(name string) bool {
	val, ok := b.Settings().FeatureFlags[name]
	return ok && val
}

// getFeatureFlagInfo returns the status of feature flags that differ
// from their default status.
func (b *Build) getFeatureFlagInfo() string {
	var statuses []string
	for _, ff := range featureflags.GetAll() {
		isOn := b.IsFeatureFlagOn(ff.Name)

		if isOn != ff.DefaultValue {
			statuses = append(statuses, fmt.Sprintf("%s:%t", ff.Name, isOn))
		}
	}

	return strings.Join(statuses, ", ")
}

func (b *Build) printRunningWithHeader() {
	b.logger.Println("Running with", AppVersion.Line())
	if b.Runner != nil && b.Runner.ShortDescription() != "" {
		b.logger.Println(fmt.Sprintf(
			"  on %s %s, system ID: %s",
			b.Runner.Name,
			b.Runner.ShortDescription(),
			b.Runner.SystemIDState.GetSystemID(),
		))
	}
	if featureInfo := b.getFeatureFlagInfo(); featureInfo != "" {
		b.logger.Println("  feature flags:", featureInfo)
	}
}

func (b *Build) printSettingErrors() {
	if len(b.Settings().Errors) > 0 {
		b.logger.Warningln(errors.Join(b.Settings().Errors...))
	}
}

func (b *Build) IsLFSSmudgeDisabled() bool {
	return b.Settings().GitLFSSkipSmudge
}

func (b *Build) IsCIDebugServiceEnabled() bool {
	return b.Settings().CIDebugServices
}

func (b *Build) IsDebugModeEnabled() bool {
	return b.IsDebugTraceEnabled() || b.IsCIDebugServiceEnabled()
}

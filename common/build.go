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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/dns"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls"
	"gitlab.com/gitlab-org/gitlab-runner/referees"
	"gitlab.com/gitlab-org/gitlab-runner/session"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

type GitStrategy int

const (
	GitClone GitStrategy = iota
	GitFetch
	GitNone
)

const (
	gitCleanFlagsDefault = "-ffdx"
	gitCleanFlagsNone    = "none"
)

const (
	gitFetchFlagsDefault = "--prune --quiet"
	gitFetchFlagsNone    = "none"
)

type SubmoduleStrategy int

const (
	SubmoduleInvalid SubmoduleStrategy = iota
	SubmoduleNone
	SubmoduleNormal
	SubmoduleRecursive
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

const (
	BuildStageResolveSecrets           BuildStage = "resolve_secrets"
	BuildStagePrepareExecutor          BuildStage = "prepare_executor"
	BuildStagePrepare                  BuildStage = "prepare_script"
	BuildStageGetSources               BuildStage = "get_sources"
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

type invalidAttemptError struct {
	key string
}

func (i *invalidAttemptError) Error() string {
	return fmt.Sprintf("number of attempts out of the range [1, 10] for variable: %s", i.key)
}

func (i *invalidAttemptError) Is(err error) bool {
	_, ok := err.(*invalidAttemptError)
	return ok
}

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
	ExecutorName     func() string

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

	secretsResolver func(l logger, registry SecretResolverRegistry) (SecretsResolver, error)

	Session *session.Session

	logger BuildLogger

	allVariables     JobVariables
	secretsVariables JobVariables

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

func (b *Build) getCustomBuildDir(rootDir, overrideKey string, customBuildDirEnabled, sharedDir bool) (string, error) {
	dir := b.GetAllVariables().Get(overrideKey)
	if dir == "" {
		return path.Join(rootDir, b.ProjectUniqueDir(sharedDir)), nil
	}

	if !customBuildDirEnabled {
		return "", MakeBuildError("setting %s is not allowed, enable `custom_build_dir` feature", overrideKey)
	}

	relDir, err := filepath.Rel(rootDir, dir)
	if err != nil {
		return "", &BuildError{Inner: err}
	}
	if strings.HasPrefix(relDir, "..") {
		return "", MakeBuildError("the %s=%q has to be within %q", overrideKey, dir, rootDir)
	}

	return path.Clean(dir), nil
}

func (b *Build) StartBuild(rootDir, cacheDir string, customBuildDirEnabled, sharedDir bool) error {
	if rootDir == "" {
		return MakeBuildError("the builds_dir is not configured")
	}

	if cacheDir == "" {
		return MakeBuildError("the cache_dir is not configured")
	}

	// We set RootDir and invalidate variables
	// to be able to use CI_BUILDS_DIR
	b.RootDir = rootDir
	b.CacheDir = path.Join(cacheDir, b.ProjectUniqueDir(false))
	b.refreshAllVariables()

	var err error
	b.BuildDir, err = b.getCustomBuildDir(b.RootDir, "GIT_CLONE_PATH", customBuildDirEnabled, sharedDir)
	if err != nil {
		return err
	}

	// We invalidate variables to be able to use
	// CI_CACHE_DIR and CI_PROJECT_DIR
	b.refreshAllVariables()
	return nil
}

func (b *Build) executeStage(ctx context.Context, buildStage BuildStage, executor Executor) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	b.setCurrentStage(buildStage)
	b.Log().WithField("build_stage", buildStage).Debug("Executing build stage")

	shell := executor.Shell()
	if shell == nil {
		return errors.New("no shell defined")
	}

	script, err := GenerateShellScript(buildStage, *shell)
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

func (b *Build) executeScript(ctx context.Context, executor Executor) error {
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

	err = b.attemptExecuteStage(ctx, BuildStageGetSources, executor, b.GetGetSourcesAttempts())

	if err == nil {
		err = b.attemptExecuteStage(ctx, BuildStageRestoreCache, executor, b.GetRestoreCacheAttempts())
	}
	if err == nil {
		err = b.attemptExecuteStage(ctx, BuildStageDownloadArtifacts, executor, b.GetDownloadArtifactsAttempts())
	}

	if err == nil {
		for _, s := range b.Steps {
			// after_script has a separate BuildStage. See common.BuildStageAfterScript
			if s.Name == StepNameAfterScript {
				continue
			}
			err = b.executeStage(ctx, StepToBuildStage(s), executor)
			if err != nil {
				break
			}
		}

		b.executeAfterScript(ctx, err, executor)
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

func (b *Build) executeAfterScript(ctx context.Context, err error, executor Executor) {
	state, _ := b.runtimeStateAndError(err)
	b.GetAllVariables().OverwriteKey("CI_JOB_STATUS", JobVariable{
		Key:   "CI_JOB_STATUS",
		Value: string(state),
	})

	ctx, cancel := context.WithTimeout(ctx, AfterScriptTimeout)
	defer cancel()

	_ = b.executeStage(ctx, BuildStageAfterScript, executor)
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
	switch err {
	case context.Canceled:
		return BuildRunRuntimeCanceled, &BuildError{
			Inner:         errors.New("canceled"),
			FailureReason: JobCanceled,
		}

	case context.DeadlineExceeded:
		return BuildRunRuntimeTimedout, &BuildError{
			Inner:         fmt.Errorf("execution took longer than %v seconds", b.GetBuildTimeout()),
			FailureReason: JobExecutionTimeout,
		}

	case nil:
		return BuildRunRuntimeSuccess, nil

	default:
		return BuildRunRuntimeFailed, err
	}
}

func (b *Build) run(ctx context.Context, executor Executor) (err error) {
	b.setCurrentState(BuildRunRuntimeRunning)

	buildFinish := make(chan error, 1)
	buildPanic := make(chan error, 1)

	runContext, runCancel := context.WithCancel(context.Background())
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
				buildPanic <- &BuildError{FailureReason: RunnerSystemFailure, Inner: fmt.Errorf("panic: %s", r)}
			}
		}()

		buildFinish <- b.executeScript(runContext, executor)
	}()

	// Wait for signals: cancel, timeout, abort or finish
	b.Log().Debugln("Waiting for signals...")
	select {
	case <-ctx.Done():
		err = b.handleError(ctx.Err())

	case signal := <-b.SystemInterrupt:
		err = &BuildError{
			Inner:         fmt.Errorf("aborted: %v", signal),
			FailureReason: RunnerSystemFailure,
		}
		b.setCurrentState(BuildRunRuntimeTerminated)

	case err = <-buildFinish:
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
	logger BuildLogger,
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
			return nil, b.handleError(options.Context.Err())
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

		trace.Fail(err, JobFailureData{
			Reason:   b.ensureSupportedFailureReason(buildError.FailureReason),
			ExitCode: buildError.ExitCode,
		})

		return
	}

	logger.Errorln("Job failed (system failure):", err)
	trace.Fail(err, JobFailureData{Reason: RunnerSystemFailure})
}

func (b *Build) ensureSupportedFailureReason(reason JobFailureReason) JobFailureReason {
	if reason == "" {
		return ScriptFailure
	}

	// GitLab provides a list of supported failure reasons with the job. Should the list be empty, we use
	// the minmum subset of failure reasons we know all GitLab instances support.
	for _, supported := range append(
		b.Features.FailureReasons,
		ScriptFailure,
		RunnerSystemFailure,
		JobExecutionTimeout,
	) {
		if reason == supported {
			return reason
		}
	}

	return UnknownFailure
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

func (b *Build) Run(globalConfig *Config, trace JobTrace) (err error) {
	var executor Executor

	b.ExecutorName = func() string {
		return executor.Name()
	}

	b.logger = NewBuildLogger(trace, b.Log())
	b.printRunningWithHeader()

	b.setCurrentState(BuildRunStatePending)

	// These defers are ordered because runBuild could panic and the recover needs to handle that panic.
	// setTraceStatus needs to be last since it needs a correct error value to report the job's status
	defer func() { b.setTraceStatus(trace, err) }()

	defer func() {
		if r := recover(); r != nil {
			err = &BuildError{FailureReason: RunnerSystemFailure, Inner: fmt.Errorf("panic: %s", r)}
		}
	}()

	defer func() { b.cleanupBuild(executor) }()

	err = b.resolveSecrets()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), b.GetBuildTimeout())
	defer cancel()

	b.configureTrace(trace, cancel)

	options := b.createExecutorPrepareOptions(ctx, globalConfig, trace)
	provider := GetExecutorProvider(b.Runner.Executor)
	if provider == nil {
		return errors.New("executor not found")
	}

	err = provider.GetFeatures(&b.ExecutorFeatures)
	if err != nil {
		return fmt.Errorf("retrieving executor features: %w", err)
	}

	executor, err = b.executeBuildSection(executor, options, provider)

	if err == nil {
		err = b.run(ctx, executor)
		if errWait := b.waitForTerminal(ctx, globalConfig.SessionServer.GetSessionTimeout()); errWait != nil {
			b.Log().WithError(errWait).Debug("Stopped waiting for terminal")
		}
	}

	if executor != nil {
		executor.Finish(err)
	}

	return err
}

func (b *Build) configureTrace(trace JobTrace, cancel context.CancelFunc) {
	trace.SetCancelFunc(cancel)
	trace.SetAbortFunc(cancel)
	trace.SetMasked(MaskOptions{
		Phrases:       b.GetAllVariables().Masked(),
		TokenPrefixes: b.JobResponse.Features.TokenMaskPrefixes,
	})
}

func (b *Build) createExecutorPrepareOptions(
	ctx context.Context,
	globalConfig *Config,
	trace JobTrace,
) ExecutorPrepareOptions {
	return ExecutorPrepareOptions{
		Config:  b.Runner,
		Build:   b,
		Trace:   trace,
		User:    globalConfig.User,
		Context: ctx,
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
			resolver, err := b.secretsResolver(&b.logger, GetSecretResolverRegistry())
			if err != nil {
				return fmt.Errorf("creating secrets resolver: %w", err)
			}

			variables, err := resolver.Resolve(b.Secrets)
			if err != nil {
				return fmt.Errorf("resolving secrets: %w", err)
			}

			b.secretsVariables = variables
			b.refreshAllVariables()

			return nil
		},
	}

	return section.Execute(&b.logger)
}

func (b *Build) executeBuildSection(
	executor Executor,
	options ExecutorPrepareOptions,
	provider ExecutorProvider,
) (Executor, error) {
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

func (b *Build) cleanupBuild(executor Executor) {
	if executor != nil {
		executor.Cleanup()
	}
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

func (b *Build) GetTLSVariables(caFile, certFile, keyFile string) JobVariables {
	variables := JobVariables{}

	if b.TLSCAChain != "" {
		variables = append(variables, JobVariable{
			Key:      caFile,
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
				Key:      certFile,
				Value:    b.TLSAuthCert,
				Public:   true,
				Internal: true,
				File:     true,
			},
			JobVariable{
				Key:      keyFile,
				Value:    b.TLSAuthKey,
				Internal: true,
				File:     true,
			},
		)
	}

	return variables
}

func (b *Build) GetCITLSVariables() JobVariables {
	return b.GetTLSVariables(tls.VariableCAFile, tls.VariableCertFile, tls.VariableKeyFile)
}

func (b *Build) IsSharedEnv() bool {
	return b.ExecutorFeatures.Shared
}

func (b *Build) refreshAllVariables() {
	b.allVariables = nil
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

// GetRemoteURL checks if the default clone URL is overwritten by the runner
// configuration option: 'CloneURL'. If it is, we use that to create the clone
// URL.
func (b *Build) GetRemoteURL() string {
	u, _ := url.Parse(b.Runner.CloneURL)

	if u == nil || u.Scheme == "" {
		return b.GitInfo.RepoURL
	}

	if u.Scheme == "ssh" {
		if u.User == nil {
			u.User = url.User("git")
		}
	} else {
		u.User = url.UserPassword("gitlab-ci-token", b.Token)
	}

	projectPath := b.GetAllVariables().Value("CI_PROJECT_PATH") + ".git"
	u.Path = path.Join(u.Path, projectPath)

	return u.String()
}

func (b *Build) GetGitStrategy() GitStrategy {
	switch b.GetAllVariables().Value("GIT_STRATEGY") {
	case "clone":
		return GitClone

	case "fetch":
		return GitFetch

	case "none":
		return GitNone

	default:
		if b.AllowGitFetch {
			return GitFetch
		}

		return GitClone
	}
}

func (b *Build) GetGitCheckout() bool {
	if b.GetGitStrategy() == GitNone {
		return false
	}

	strCheckout := b.GetAllVariables().Value("GIT_CHECKOUT")
	if strCheckout == "" {
		return true
	}

	checkout, err := strconv.ParseBool(strCheckout)
	if err != nil {
		return true
	}
	return checkout
}

func (b *Build) GetSubmoduleStrategy() SubmoduleStrategy {
	if b.GetGitStrategy() == GitNone {
		return SubmoduleNone
	}
	switch b.GetAllVariables().Value("GIT_SUBMODULE_STRATEGY") {
	case "normal":
		return SubmoduleNormal

	case "recursive":
		return SubmoduleRecursive

	case "none", "":
		// Default (legacy) behavior is to not update/init submodules
		return SubmoduleNone

	default:
		// Will cause an error in AbstractShell) writeSubmoduleUpdateCmds
		return SubmoduleInvalid
	}
}

// GetSubmodulePaths https://git-scm.com/docs/git-submodule#Documentation/git-submodule.txt-ltpathgt82308203
func (b *Build) GetSubmodulePaths() ([]string, error) {
	paths := b.GetAllVariables().Value("GIT_SUBMODULE_PATHS")
	toks := strings.Fields(paths)
	for _, tok := range toks {
		if tok == ":(exclude)" {
			return nil, fmt.Errorf("invalid submodule pathspec '%s'", paths)
		}
	}
	return toks, nil
}

func (b *Build) GetSubmoduleDepth() int {
	depth, err := strconv.Atoi(b.GetAllVariables().Value("GIT_SUBMODULE_DEPTH"))
	if err != nil {
		return b.GitInfo.Depth
	}
	return depth
}

func (b *Build) GetGitCleanFlags() []string {
	flags := b.GetAllVariables().Value("GIT_CLEAN_FLAGS")
	if flags == "" {
		flags = gitCleanFlagsDefault
	}

	if flags == gitCleanFlagsNone {
		return []string{}
	}

	return strings.Fields(flags)
}

func (b *Build) GetGitFetchFlags() []string {
	flags := b.GetAllVariables().Value("GIT_FETCH_EXTRA_FLAGS")
	if flags == "" {
		flags = gitFetchFlagsDefault
	}

	if flags == gitFetchFlagsNone {
		return []string{}
	}

	return strings.Fields(flags)
}

func (b *Build) GetGitSubmoduleUpdateFlags() []string {
	flags := b.GetAllVariables().Value("GIT_SUBMODULE_UPDATE_FLAGS")
	return strings.Fields(flags)
}

func (b *Build) IsDebugTraceEnabled() bool {
	trace, err := strconv.ParseBool(b.GetAllVariables().Value("CI_DEBUG_TRACE"))
	if err != nil {
		trace = false
	}

	if b.Runner.DebugTraceDisabled {
		if trace {
			b.logger.Warningln("CI_DEBUG_TRACE usage is disabled on this Runner")
		}

		return false
	}

	return trace
}

func (b *Build) GetDockerAuthConfig() string {
	return b.GetAllVariables().Value("DOCKER_AUTH_CONFIG")
}

func (b *Build) GetGetSourcesAttempts() int {
	retries, err := strconv.Atoi(b.GetAllVariables().Value("GET_SOURCES_ATTEMPTS"))
	if err != nil {
		return DefaultGetSourcesAttempts
	}
	return retries
}

func (b *Build) GetDownloadArtifactsAttempts() int {
	retries, err := strconv.Atoi(b.GetAllVariables().Value("ARTIFACT_DOWNLOAD_ATTEMPTS"))
	if err != nil {
		return DefaultArtifactDownloadAttempts
	}
	return retries
}

func (b *Build) GetRestoreCacheAttempts() int {
	retries, err := strconv.Atoi(b.GetAllVariables().Value("RESTORE_CACHE_ATTEMPTS"))
	if err != nil {
		return DefaultRestoreCacheAttempts
	}
	return retries
}

func (b *Build) GetCacheRequestTimeout() int {
	timeout, err := strconv.Atoi(b.GetAllVariables().Value("CACHE_REQUEST_TIMEOUT"))
	if err != nil {
		return DefaultCacheRequestTimeout
	}
	return timeout
}

func (b *Build) GetExecutorJobSectionAttempts() (int, error) {
	attempts, err := strconv.Atoi(b.GetAllVariables().Value(ExecutorJobSectionAttempts))
	if err != nil {
		return DefaultExecutorStageAttempts, nil
	}

	if validAttempts(attempts) {
		return 0, &invalidAttemptError{key: ExecutorJobSectionAttempts}
	}

	return attempts, nil
}

func validAttempts(attempts int) bool {
	return attempts < 1 || attempts > 10
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
	if b.Runner.IsFeatureFlagDefined(name) {
		return b.Runner.IsFeatureFlagOn(name)
	}

	return featureflags.IsOn(
		b.Log().WithField("name", name),
		b.GetAllVariables().Get(name),
	)
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

func (b *Build) IsLFSSmudgeDisabled() bool {
	disabled, err := strconv.ParseBool(b.GetAllVariables().Value("GIT_LFS_SKIP_SMUDGE"))
	if err != nil {
		return false
	}

	return disabled
}

func (b *Build) IsCIDebugServiceEnabled() bool {
	debugServices := b.GetAllVariables().Value("CI_DEBUG_SERVICES")

	if debugServices == "" {
		return false
	}

	enabled, err := strconv.ParseBool(debugServices)
	if err != nil {
		b.logger.Warningln(fmt.Sprintf(
			"failed to parse value '%s' for CI_DEBUG_SERVICES variable: %s",
			debugServices,
			err.Error(),
		))
	}
	return enabled
}

func (b *Build) IsDebugModeEnabled() bool {
	return b.IsDebugTraceEnabled() || b.IsCIDebugServiceEnabled()
}

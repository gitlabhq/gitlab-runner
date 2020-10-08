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
	BuildRunStatePending   BuildRuntimeState = "pending"
	BuildRunRuntimeRunning BuildRuntimeState = "running"
	BuildRunRuntimeSuccess BuildRuntimeState = "success"
	BuildRunRuntimeFailed  BuildRuntimeState = "failed"
	// This runtime state indicates that operation did finish
	// which is not exactly true, canceled means that we can still
	// be canceling job (running after_script)
	BuildRunRuntimeCanceled   BuildRuntimeState = "canceled"
	BuildRunRuntimeAborted    BuildRuntimeState = "aborted"
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
	BuildStageCleanupFileVariables     BuildStage = "cleanup_file_variables"
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
	BuildStageCleanupFileVariables,
}

const (
	ExecutorJobSectionAttempts = "EXECUTOR_JOB_SECTION_ATTEMPTS"
)

// ErrSkipBuildStage is returned when there's nothing to be executed for the
// build stage.
var ErrSkipBuildStage = errors.New("skip build stage")

var (
	errCanceledBuildError = &BuildError{
		Inner:         errors.New("canceled"),
		FailureReason: JobCanceled,
	}

	errAbortedBuildError = &BuildError{
		Inner:         errors.New("aborted"),
		FailureReason: JobAborted,
	}
)

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

	// Unique ID for all running builds on this runner
	RunnerID int `json:"runner_id"`

	// Unique ID for all running builds on this runner and this project
	ProjectRunnerID int `json:"project_runner_id"`

	// statusLock handles access to currentStage, currentState and
	// executorStageResolver. These variables can be accessed via
	// CurrentStage(), CurrentState() and CurrentExecutorStage() from the
	// metrics go routine whilst a build is in-flight.
	statusLock            sync.RWMutex
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
	ArtifactUploader func(config JobCredentials, reader io.Reader, options ArtifactsOptions) UploadState
}

func (b *Build) setCurrentStage(stage BuildStage) {
	b.statusLock.Lock()
	defer b.statusLock.Unlock()

	b.currentStage = stage
}

func (b *Build) CurrentStage() BuildStage {
	b.statusLock.RLock()
	defer b.statusLock.RUnlock()

	return b.currentStage
}

func (b *Build) setCurrentState(state BuildRuntimeState) {
	b.statusLock.Lock()
	defer b.statusLock.Unlock()

	b.currentState = state
}

func (b *Build) CurrentState() BuildRuntimeState {
	b.statusLock.RLock()
	defer b.statusLock.RUnlock()

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

	if !strings.HasPrefix(dir, rootDir) {
		return "", MakeBuildError("the %s=%q has to be within %q", overrideKey, dir, rootDir)
	}

	return dir, nil
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
//  the predefined environment that GitLab Runner provided.
func getPredefinedEnv(buildStage BuildStage) bool {
	env := map[BuildStage]bool{
		BuildStagePrepare:                  true,
		BuildStageGetSources:               true,
		BuildStageRestoreCache:             true,
		BuildStageDownloadArtifacts:        true,
		BuildStageAfterScript:              false,
		BuildStageArchiveOnSuccessCache:    true,
		BuildStageArchiveOnFailureCache:    false,
		BuildStageUploadOnFailureArtifacts: true,
		BuildStageUploadOnSuccessArtifacts: true,
		BuildStageCleanupFileVariables:     true,
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
		BuildStageCleanupFileVariables:     "Cleaning up file based variables",
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

func (b *Build) executeSteps(ctx context.Context, executor Executor) error {
	for _, s := range b.Steps {
		// after_script has a separate BuildStage. See common.BuildStageAfterScript
		if s.Name == StepNameAfterScript {
			continue
		}
		err := b.executeStage(ctx, StepToBuildStage(s), executor)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Build) executeScript(abortCtx context.Context, trace JobTrace, executor Executor) error {
	// track job start and create referees
	startTime := time.Now()
	b.createReferees(executor)

	ctx, cancel := context.WithCancel(abortCtx)
	defer cancel()
	trace.SetCancelFunc(cancel)

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
		err = b.executeSteps(ctx, executor)

		// This does indicate that build got canceled, instead of aborted
		if ctx.Err() != nil && abortCtx.Err() == nil {
			err = errCanceledBuildError
		}

		// After script should be executed always regardless of `ctx` being canceled
		b.executeAfterScript(abortCtx, err, executor)
	}

	archiveCacheErr := b.executeArchiveCache(ctx, err, executor)

	artifactUploadErr := b.executeUploadArtifacts(ctx, err, executor)

	// track job end and execute referees
	b.executeUploadReferees(ctx, startTime, time.Now())

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
	err := b.executeStage(ctx, BuildStageCleanupFileVariables, executor)
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
		b.ArtifactUploader(jobCredentials, reader, ArtifactsOptions{
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
	case errCanceledBuildError:
		return BuildRunRuntimeCanceled, err

	case context.Canceled:
		// This is not obvious:
		// it tries to discover a `abortCtx` being canceled,
		// thus having an abort outcome
		return BuildRunRuntimeAborted, errAbortedBuildError

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

func (b *Build) run(ctx context.Context, trace JobTrace, executor Executor) (err error) {
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

		buildFinish <- b.executeScript(runContext, trace, executor)
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
		if _, ok := err.(*BuildError); ok {
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

func (b *Build) setTraceStatus(trace JobTrace, err error) {
	logger := b.logger.WithFields(logrus.Fields{
		"duration": b.Duration(),
	})

	if err == nil {
		logger.Infoln("Job succeeded")
		trace.Success()

		return
	}

	if buildError, ok := err.(*BuildError); ok {
		logger.SoftErrorln("Job failed:", err)

		failureReason := buildError.FailureReason
		if failureReason == "" {
			failureReason = ScriptFailure
		}

		trace.Fail(err, failureReason)

		return
	}

	logger.Errorln("Job failed (system failure):", err)
	trace.Fail(err, RunnerSystemFailure)
}

func (b *Build) setExecutorStageResolver(resolver func() ExecutorStage) {
	b.statusLock.Lock()
	defer b.statusLock.Unlock()

	b.executorStageResolver = resolver
}

func (b *Build) CurrentExecutorStage() ExecutorStage {
	b.statusLock.RLock()
	defer b.statusLock.RUnlock()

	if b.executorStageResolver == nil {
		return ExecutorStage("")
	}

	return b.executorStageResolver()
}

func (b *Build) Run(globalConfig *Config, trace JobTrace) (err error) {
	var executor Executor

	b.logger = NewBuildLogger(trace, b.Log())
	b.logger.Println("Running with", AppVersion.Line())
	if b.Runner != nil && b.Runner.ShortDescription() != "" {
		b.logger.Println("  on", b.Runner.Name, b.Runner.ShortDescription())
	}

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

	// In early phases of build preparation a user-requested cancel is treated as an abort
	trace.SetCancelFunc(cancel)
	trace.SetAbortFunc(cancel)
	trace.SetMasked(b.GetAllVariables().Masked())

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
		err = b.run(ctx, trace, executor)
		if errWait := b.waitForTerminal(ctx, globalConfig.SessionServer.GetSessionTimeout()); errWait != nil {
			b.Log().WithError(errWait).Debug("Stopped waiting for terminal")
		}
	}

	if executor != nil {
		executor.Finish(err)
	}

	return err
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
	}
}

func (b *Build) GetDefaultFeatureFlagsVariables() JobVariables {
	variables := make(JobVariables, 0)
	for _, featureFlag := range featureflags.GetAll() {
		variables = append(variables, JobVariable{
			Key:      featureFlag.Name,
			Value:    featureFlag.DefaultValue,
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

	b.allVariables = variables.Expand()

	return b.allVariables
}

// GetRemoteURL checks if the default clone URL is overwritten by the runner
// configuration option: 'CloneURL'. If it is, we use that to create the clone
// URL.
func (b *Build) GetRemoteURL() string {
	cloneURL := strings.TrimRight(b.Runner.CloneURL, "/")

	if !strings.HasPrefix(cloneURL, "http") {
		return b.GitInfo.RepoURL
	}

	variables := b.GetAllVariables()
	ciJobToken := variables.Get("CI_JOB_TOKEN")
	ciProjectPath := variables.Get("CI_PROJECT_PATH")

	splits := strings.SplitAfterN(cloneURL, "://", 2)

	return fmt.Sprintf("%sgitlab-ci-token:%s@%s/%s.git", splits[0], ciJobToken, splits[1], ciProjectPath)
}

func (b *Build) GetGitStrategy() GitStrategy {
	switch b.GetAllVariables().Get("GIT_STRATEGY") {
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

	strCheckout := b.GetAllVariables().Get("GIT_CHECKOUT")
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
	switch b.GetAllVariables().Get("GIT_SUBMODULE_STRATEGY") {
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

func (b *Build) GetGitCleanFlags() []string {
	flags := b.GetAllVariables().Get("GIT_CLEAN_FLAGS")
	if flags == "" {
		flags = gitCleanFlagsDefault
	}

	if flags == gitCleanFlagsNone {
		return []string{}
	}

	return strings.Fields(flags)
}

func (b *Build) GetGitFetchFlags() []string {
	flags := b.GetAllVariables().Get("GIT_FETCH_EXTRA_FLAGS")
	if flags == "" {
		flags = gitFetchFlagsDefault
	}

	if flags == gitFetchFlagsNone {
		return []string{}
	}

	return strings.Fields(flags)
}

func (b *Build) IsDebugTraceEnabled() bool {
	trace, err := strconv.ParseBool(b.GetAllVariables().Get("CI_DEBUG_TRACE"))
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
	return b.GetAllVariables().Get("DOCKER_AUTH_CONFIG")
}

func (b *Build) GetGetSourcesAttempts() int {
	retries, err := strconv.Atoi(b.GetAllVariables().Get("GET_SOURCES_ATTEMPTS"))
	if err != nil {
		return DefaultGetSourcesAttempts
	}
	return retries
}

func (b *Build) GetDownloadArtifactsAttempts() int {
	retries, err := strconv.Atoi(b.GetAllVariables().Get("ARTIFACT_DOWNLOAD_ATTEMPTS"))
	if err != nil {
		return DefaultArtifactDownloadAttempts
	}
	return retries
}

func (b *Build) GetRestoreCacheAttempts() int {
	retries, err := strconv.Atoi(b.GetAllVariables().Get("RESTORE_CACHE_ATTEMPTS"))
	if err != nil {
		return DefaultRestoreCacheAttempts
	}
	return retries
}

func (b *Build) GetCacheRequestTimeout() int {
	timeout, err := strconv.Atoi(b.GetAllVariables().Get("CACHE_REQUEST_TIMEOUT"))
	if err != nil {
		return DefaultCacheRequestTimeout
	}
	return timeout
}

func (b *Build) GetExecutorJobSectionAttempts() (int, error) {
	attempts, err := strconv.Atoi(b.GetAllVariables().Get(ExecutorJobSectionAttempts))
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
	value := b.GetAllVariables().Get(name)

	on, err := featureflags.IsOn(value)
	if err != nil {
		logrus.WithError(err).
			WithField("name", name).
			WithField("value", value).
			Error("Error while parsing the value of feature flag")

		return false
	}

	return on
}

func (b *Build) IsLFSSmudgeDisabled() bool {
	disabled, err := strconv.ParseBool(b.GetAllVariables().Get("GIT_LFS_SKIP_SMUDGE"))
	if err != nil {
		return false
	}

	return disabled
}

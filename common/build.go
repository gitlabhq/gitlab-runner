package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	"github.com/jpillora/backoff"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/dns"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls"
	"gitlab.com/gitlab-org/gitlab-runner/referees"
	"gitlab.com/gitlab-org/gitlab-runner/session"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
	"gitlab.com/gitlab-org/gitlab-runner/steps"
	"gitlab.com/gitlab-org/step-runner/pkg/api/client"
	"gitlab.com/gitlab-org/step-runner/schema/v1"
)

type BuildRuntimeState string

func (s BuildRuntimeState) String() string {
	return string(s)
}

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

type OnBuildStageFn func(stage BuildStage)

func (fn OnBuildStageFn) Call(stage BuildStage) {
	if fn != nil {
		fn(stage)
	}
}

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

var (
	ErrJobCanceled      = errors.New("canceled")
	ErrJobScriptTimeout = errors.New("script timeout")
)

const (
	ExecutorJobSectionAttempts = "EXECUTOR_JOB_SECTION_ATTEMPTS"
)

// ErrSkipBuildStage is returned when there's nothing to be executed for the
// build stage.
var ErrSkipBuildStage = errors.New("skip build stage")

type Build struct {
	spec.Job `yaml:",inline" inputs:"expand"`

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

	failureReason spec.JobFailureReason

	secretsResolver func(l logger, registry SecretResolverRegistry, featureFlagOn func(string) bool) (SecretsResolver, error)

	Session *session.Session

	logger buildlogger.Logger

	allVariables     spec.Variables
	secretsVariables spec.Variables
	buildSettings    *BuildSettings

	startedAt  time.Time
	finishedAt time.Time

	Referees         []referees.Referee
	ArtifactUploader func(config JobCredentials, bodyProvider ContentProvider, options ArtifactsOptions) (UploadState, string)

	urlHelper urlHelper

	OnBuildStageStartFn OnBuildStageFn
	OnBuildStageEndFn   OnBuildStageFn
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

func (b *Build) setCurrentStateIf(existingState BuildRuntimeState, newState BuildRuntimeState) {
	b.statusLock.Lock()
	defer b.statusLock.Unlock()

	if b.currentState != existingState {
		return
	}

	b.currentState = newState
}

func (b *Build) CurrentState() BuildRuntimeState {
	b.statusLock.Lock()
	defer b.statusLock.Unlock()

	return b.currentState
}

func (b *Build) FailureReason() spec.JobFailureReason {
	return b.failureReason
}

func (b *Build) Log() *logrus.Entry {
	l := b.Runner.Log().
		WithFields(logrus.Fields{
			"job":               b.ID,
			"project":           b.JobInfo.ProjectID,
			"project_full_path": b.JobInfo.ProjectFullPath,
			"namespace_id":      b.JobInfo.NamespaceID,
			"root_namespace_id": b.JobInfo.RootNamespaceID,
			"organization_id":   b.JobInfo.OrganizationID,
			"gitlab_user_id":    b.JobInfo.UserID,
		})

	if b.JobInfo.ScopedUserID != nil {
		l = l.WithField("gitlab_scoped_user_id", *b.JobInfo.ScopedUserID)
	}

	// this is only set after the prepare stage has run
	if b.Hostname != "" {
		l = l.WithField("name", b.Hostname)
	}

	// executor-specific log fields
	for k, v := range GetExecutorLogFields(b.ExecutorData) {
		l = l.WithField(k, v)
	}

	return l
}

// ProjectUniqueShortName returns a unique name for the current build.
// It is similar to ProjectUniqueName but removes unnecessary string
// and adds the current BuildID as an additional composition to the unique string
func (b *Build) ProjectUniqueShortName() string {
	projectUniqueName := fmt.Sprintf(
		"runner-%s-%d-%d-%d",
		b.Runner.ShortDescription(),
		b.JobInfo.ProjectID,
		b.ProjectRunnerID,
		b.ID,
	)

	return dns.MakeRFC1123Compatible(projectUniqueName)
}

// ProjectUniqueName returns a unique name for a runner && project. It uses the runner's short description, thus uses a
// truncated token in it's human readable form.
func (b *Build) ProjectUniqueName() string {
	projectUniqueName := fmt.Sprintf(
		"runner-%s-project-%d-concurrent-%d",
		b.Runner.ShortDescription(),
		b.JobInfo.ProjectID,
		b.ProjectRunnerID,
	)

	return dns.MakeRFC1123Compatible(projectUniqueName)
}

// ProjectRealUniqueName is similar to its sister methods, and returns a unique name for the runner && project.
// It uses the following parts to generate a truncated¹ sha256 sum:
//   - the runner's full token
//   - the runner's system ID
//   - the project ID
//   - the project runner ID
//
// With that the name is not susceptible to name clashes, when tokens are similar enough and therefore are the same
// after getting the runner's short description (i.e. after the token has been truncated)
//
// ¹ we truncate the resulting sum from original 32 bytes to 16 bytes, to give us and users a shorter name, thus shorter
// volume names when used in the docker volume manager. Truncating to 16 bytes (32 chars when hex encoded, the same
// length as an hex encoded md5sum) is cryptographically sound, it's still strong against collisions.
func (b *Build) ProjectRealUniqueName() string {
	const byteLen = 16

	data := fmt.Sprintf("%s-%s-%d-%d",
		b.Runner.GetToken(),
		b.Runner.GetSystemID(),
		b.JobInfo.ProjectID,
		b.ProjectRunnerID,
	)

	sum := sha256.Sum256([]byte(data))
	return "runner-" + hex.EncodeToString(sum[:byteLen])
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
	// <some-path>/runner-short-id/concurrent-project-id/group-name/project-name/
	// ex.<some-path>/01234567/0/group/repo/
	if sharedDir {
		dir = path.Join(
			b.Runner.ShortDescription(),
			fmt.Sprintf("%d", b.ProjectRunnerID),
			dir,
		)
	}

	if b.GetGitStrategy() == GitEmpty {
		dir += "-empty"
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
		if s.Name == spec.StepNameAfterScript {
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

//nolint:gocognit
func (b *Build) executeStepStage(ctx context.Context, connector steps.Connector, buildStage BuildStage, req []schema.Step) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	b.OnBuildStageStartFn.Call(buildStage)
	defer b.OnBuildStageEndFn.Call(buildStage)

	b.setCurrentStage(buildStage)
	b.Log().WithField("build_stage", buildStage).Debug("Executing build stage")

	section := helpers.BuildSection{
		Name:        string(buildStage),
		SkipMetrics: !b.Job.Features.TraceSections,
		Run: func() error {
			msg := fmt.Sprintf(
				"%s%s%s",
				helpers.ANSI_BOLD_CYAN,
				GetStageDescription(buildStage),
				helpers.ANSI_RESET,
			)
			b.logger.Println(msg)

			// todo: step-runner should eventually:
			// - format its own logs to the Runner log spec
			// - provides its own timestamps and mask its own secrets
			// for now though, we wrap its logs providing this, and treat everything as stdout
			stdout := b.logger.Stream(buildlogger.StreamWorkLevel, buildlogger.Stdout)

			info := steps.JobInfo{
				ID:         b.ID,
				ProjectDir: b.FullProjectDir(),
				Variables:  b.GetAllVariables(),
			}

			return wrapStepStageErr(steps.Execute(ctx, connector, info, req, stdout))
		},
	}

	return section.Execute(&b.logger)
}

func wrapStepStageErr(err error) error {
	if err == nil {
		return nil
	}

	berr := &BuildError{Inner: err}

	var cserr *steps.ClientStatusError
	if errors.As(err, &cserr) {
		switch cserr.Status.State {
		case client.StateUnspecified:
			berr.FailureReason = UnknownFailure
		case client.StateFailure:
			berr.FailureReason = ScriptFailure
		}
	}

	return berr
}

//nolint:gocognit
func (b *Build) executeStage(ctx context.Context, buildStage BuildStage, executor Executor) error {
	if b.UseNativeSteps() {
		connector, ok := executor.(steps.Connector)
		if ok {
			if handled, steps := stepDispatch(b, executor, buildStage); handled {
				return b.executeStepStage(ctx, connector, buildStage, steps)
			}
		}
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	b.OnBuildStageStartFn.Call(buildStage)
	defer b.OnBuildStageEndFn.Call(buildStage)

	b.setCurrentStage(buildStage)
	b.Log().WithField("build_stage", buildStage).Debug("Executing build stage")

	defer func() {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			b.logger.Warningln(
				string(buildStage) + " could not run to completion because the timeout was exceeded. " +
					"For more control over job and script timeouts see: " +
					"https://docs.gitlab.com/ci/runners/configure_runners/#set-script-and-after_script-timeouts")
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
		SkipMetrics: !b.Job.Features.TraceSections,
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

func (b *Build) executeScript(ctx context.Context, trace JobTrace, executor Executor) error {
	// track job start and create referees
	startTime := time.Now()
	b.createReferees(executor)

	// execute prepare scripts
	err, cont := b.executePrepareScripts(ctx, executor)
	if !cont {
		return err
	}

	// execute user provided scripts
	//nolint:nestif
	if err == nil {
		if b.UseNativeSteps() && len(b.Job.Run) > 0 {
			if _, ok := executor.(steps.Connector); !ok {
				return ExecutorStepRunnerConnectNotSupported
			}
			err = b.executeStage(ctx, stepRunBuildStage, executor)
		} else {
			err = b.executeUserScripts(ctx, trace, executor)
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

func (b *Build) executePrepareScripts(ctx context.Context, executor Executor) (error, bool) {
	// Prepare stage
	err := b.executeStage(ctx, BuildStagePrepare, executor)
	if err != nil {
		return fmt.Errorf(
			"prepare environment: %w. "+
				"Check https://docs.gitlab.com/runner/shells/#shell-profile-loading for more information",
			err,
		), false
	}

	err = b.attemptExecuteStage(ctx, BuildStageGetSources, executor, b.GetGetSourcesAttempts(), func(attempt int) error {
		if attempt == 1 {
			// If GetSources fails we delete all tracked and untracked files. This is
			// because Git's submodule support has various bugs that cause fetches to
			// fail if submodules have changed.
			return b.executeStage(ctx, BuildStageClearWorktree, executor)
		}

		return nil
	})

	if err == nil {
		err = b.attemptExecuteStage(ctx, BuildStageRestoreCache, executor, b.GetRestoreCacheAttempts(), nil)
	}
	if err == nil {
		err = b.attemptExecuteStage(ctx, BuildStageDownloadArtifacts, executor, b.GetDownloadArtifactsAttempts(), nil)
	}

	return err, true
}

func (b *Build) executeUserScripts(ctx context.Context, trace JobTrace, executor Executor) error {
	var err error

	timeouts := b.getStageTimeoutContexts(ctx,
		stageTimeout{"RUNNER_SCRIPT_TIMEOUT", 0},
		stageTimeout{"RUNNER_AFTER_SCRIPT_TIMEOUT", AfterScriptTimeout})

	scriptCtx, cancel := timeouts["RUNNER_SCRIPT_TIMEOUT"]()
	defer cancel()

	// update trace's cancel function so that the main script can be cancelled,
	// with after_script and later stages to still complete.
	trace.SetCancelFunc(cancel)

	b.printPolicyOptions()

	for _, s := range b.Steps {
		// after_script has a separate BuildStage. See common.BuildStageAfterScript
		if s.Name == spec.StepNameAfterScript {
			continue
		}
		err = b.executeStage(scriptCtx, StepToBuildStage(s), executor)
		if err != nil {
			break
		}
	}

	switch {
	// if parent context is fine but script context was cancelled we ensure the build error
	// failure reason is "canceled".
	case ctx.Err() == nil && errors.Is(scriptCtx.Err(), context.Canceled):
		err = &BuildError{
			Inner:         ErrJobCanceled,
			FailureReason: JobCanceled,
		}

		b.logger.Warningln("script canceled externally (UI, API)")

	// If the parent context reached deadline, don't do anything different than usual.
	// If the script context reached deadline, return the deadline error.
	case !errors.Is(ctx.Err(), context.DeadlineExceeded) && errors.Is(scriptCtx.Err(), context.DeadlineExceeded):
		err = &BuildError{
			Inner:         fmt.Errorf("%w: %w", ErrJobScriptTimeout, scriptCtx.Err()),
			FailureReason: JobExecutionTimeout,
		}
	}

	afterScriptCtx, cancel := timeouts["RUNNER_AFTER_SCRIPT_TIMEOUT"]()
	defer cancel()

	if afterScriptErr := b.executeAfterScript(afterScriptCtx, err, executor); afterScriptErr != nil {
		// the parent deadline being exceeded is reported at a later stage, so we
		// only focus on errors specific to after_script here.
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			// By default after-script ignores errors, but this can
			// be disabled via the AFTER_SCRIPT_IGNORE_ERRORS variable.

			if b.Settings().AfterScriptIgnoreErrors {
				b.logger.Warningln("after_script failed, but job will continue unaffected:", afterScriptErr)
			} else if err == nil {
				// If there's an existing error don't overwrite it with
				// the after-script error.
				err = afterScriptErr
			}
		}
	}

	return err
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
	b.GetAllVariables().OverwriteKey("CI_JOB_STATUS", spec.Variable{
		Key:   "CI_JOB_STATUS",
		Value: string(state),
	})

	return b.executeStage(ctx, BuildStageAfterScript, executor)
}

// StepToBuildStage returns the BuildStage corresponding to a step.
func StepToBuildStage(s spec.Step) BuildStage {
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
		ID:    b.Job.ID,
		Token: b.Job.Token,
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

		bodyProvider := StreamProvider{
			ReaderFactory: func() (io.ReadCloser, error) {
				return io.NopCloser(reader), nil
			},
		}

		// referee ran successfully, upload its results to GitLab as an artifact
		b.ArtifactUploader(jobCredentials, bodyProvider, ArtifactsOptions{
			BaseName: referee.ArtifactBaseName(),
			Type:     referee.ArtifactType(),
			Format:   spec.ArtifactFormat(referee.ArtifactFormat()),
		})
	}
}

func (b *Build) attemptExecuteStage(
	ctx context.Context,
	buildStage BuildStage,
	executor Executor,
	attempts int,
	retryCallback func(attempt int) error,
) error {
	if attempts < 1 || attempts > 10 {
		return fmt.Errorf("number of attempts out of the range [1, 10] for stage: %s", buildStage)
	}

	retry := backoff.Backoff{
		Min:    5 * time.Second,
		Max:    5 * time.Minute,
		Jitter: true,
		Factor: 1.5,
	}

	var err error
	for attempt := range attempts {
		if retryCallback != nil {
			if err = retryCallback(attempt); err != nil {
				continue
			}
		}

		if err = b.executeStage(ctx, buildStage, executor); err == nil {
			return nil
		}

		if attempt == attempts-1 {
			break
		}

		if b.IsFeatureFlagOn(featureflags.UseExponentialBackoffStageRetry) {
			duration := retry.Duration()
			b.logger.Infoln(fmt.Sprintf("Retrying in %v", duration))
			time.Sleep(duration)
		}
	}

	return err
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
	case errors.Is(err, context.Canceled), errors.Is(err, ErrJobCanceled):
		return BuildRunRuntimeCanceled, &BuildError{
			Inner:         ErrJobCanceled,
			FailureReason: JobCanceled,
		}

	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, ErrJobScriptTimeout):
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
	logger := b.Log().WithFields(logrus.Fields{
		"duration_s": b.FinalDuration().Seconds(),
	})

	buildLogger := b.getNewLogger(trace, logger, true)
	defer buildLogger.Close()

	if err == nil {
		logger.WithFields(logrus.Fields{"job-status": "success"}).Infoln("Job succeeded")
		buildLogger.Infoln("Job succeeded")
		logTerminationError(buildLogger, "Success", trace.Success())

		return
	}

	b.setCurrentStateIf(BuildRunStatePending, BuildRunRuntimeFailed)

	var buildError *BuildError
	if errors.As(err, &buildError) {
		b.failureReason = buildError.FailureReason

		msg := fmt.Sprint("Job failed: ", err)
		if buildError.FailureReason == RunnerSystemFailure {
			msg = fmt.Sprint("Job failed (system failure): ", err)
		}

		logger.
			WithFields(logrus.Fields{
				"job-status":     "failed",
				"error":          err,
				"failure_reason": buildError.FailureReason,
				"exit_code":      buildError.ExitCode,
			}).
			Warningln(msg)
		buildLogger.SoftErrorln(msg)

		trace.SetSupportedFailureReasonMapper(newFailureReasonMapper(b.Features.FailureReasons))
		err = trace.Fail(err, JobFailureData{
			Reason:   buildError.FailureReason,
			ExitCode: buildError.ExitCode,
		})
		logTerminationError(buildLogger, "Fail", err)

		return
	}

	logger.
		WithFields(logrus.Fields{
			"job-status":     "failed",
			"error":          err,
			"failure_reason": RunnerSystemFailure,
		}).
		Errorln("Job failed (system failure):", err)
	buildLogger.Errorln("Job failed (system failure):", err)
	logTerminationError(buildLogger, "Fail", trace.Fail(err, JobFailureData{Reason: RunnerSystemFailure}))
}

func logTerminationError(logger buildlogger.Logger, name string, err error) {
	if err == nil {
		return
	}

	logger.WithFields(logrus.Fields{"error": err}).Errorln(fmt.Sprintf("Job trace termination %q failed", name))
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
	b.setCurrentState(BuildRunStatePending)

	// These defers are ordered because runBuild could panic and the recover needs to handle that panic.
	// setTraceStatus needs to be last since it needs a correct error value to report the job's status
	defer func() {
		b.ensureFinishedAt()
		b.setTraceStatus(trace, err)
	}()

	defer func() {
		if r := recover(); r != nil {
			err = &BuildError{FailureReason: RunnerSystemFailure, Inner: fmt.Errorf("panic: %s", r)}

			b.Log().WithError(err).Error(string(debug.Stack()))
		}
	}()

	err = b.expandInputs()
	if err != nil {
		return &BuildError{FailureReason: ConfigurationError, Inner: err}
	}

	b.logUsedImages()
	b.printRunningWithHeader(trace)

	err = b.resolveSecrets(trace)
	if err != nil {
		return err
	}

	b.expandContainerOptions()

	b.logger = b.getNewLogger(trace, b.Log(), false)
	defer b.logger.Close()

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

// expandInputs expands inputs in various build configuration settings.
//
// TODO: we want to expand inputs as early as possible to optimize the feedback loop.
// However, that may lead to problems where certain expansion context is only available later on.
// This might not be a problem for Inputs itself, but for functions (like `now()`) or
// when we allow other context in the expression, like access to environment variables,
// or other job-runtime dependent features.
// For a good middle ground we could parse the scripts as moa expressions and cache them
// and only later on evaluate given the necessary context.
func (b *Build) expandInputs() error {
	if !b.IsFeatureFlagOn(featureflags.EnableJobInputsInterpolation) {
		return nil
	}

	return spec.ExpandInputs(&b.Inputs, b)
}

func (b *Build) getNewLogger(trace JobTrace, log *logrus.Entry, teeOnly bool) buildlogger.Logger {
	return buildlogger.New(
		trace,
		log,
		buildlogger.Options{
			MaskPhrases:          b.GetAllVariables().Masked(),
			MaskTokenPrefixes:    b.Job.Features.TokenMaskPrefixes,
			Timestamping:         b.IsFeatureFlagOn(featureflags.UseTimestamps),
			MaskAllDefaultTokens: b.IsFeatureFlagOn(featureflags.MaskAllDefaultTokens),
			TeeOnly:              teeOnly,
		},
	)
}

func (b *Build) logUsedImages() {
	if !b.IsFeatureFlagOn(featureflags.LogImagesConfiguredForJob) {
		return
	}

	fields := func(i spec.Image) logrus.Fields {
		if i.Name == "" {
			return nil
		}

		fields := logrus.Fields{
			"image_name": i.Name,
		}
		if i.ExecutorOptions.Docker.Platform != "" {
			fields["image_platform"] = i.ExecutorOptions.Docker.Platform
		}

		return fields
	}

	imageFields := fields(b.Job.Image)
	if imageFields != nil {
		b.Log().WithFields(imageFields).Info("Image configured for job")
	}

	for _, service := range b.Job.Services {
		b.Log().WithFields(fields(service)).Info("Service image configured for job")
	}
}

func (b *Build) configureTrace(trace JobTrace, cancel context.CancelFunc) {
	trace.SetCancelFunc(cancel)
	trace.SetAbortFunc(cancel)
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

func (b *Build) resolveSecrets(trace JobTrace) error {
	if b.Secrets == nil {
		return nil
	}

	b.Secrets.ExpandVariables(b.GetAllVariables())

	b.OnBuildStageStartFn.Call(BuildStageResolveSecrets)
	defer b.OnBuildStageEndFn.Call(BuildStageResolveSecrets)

	section := helpers.BuildSection{
		Name:        string(BuildStageResolveSecrets),
		SkipMetrics: !b.Job.Features.TraceSections,
		Run: func() error {
			logger := b.getNewLogger(trace, b.Log(), false)
			defer logger.Close()

			resolver, err := b.secretsResolver(&logger, GetSecretResolverRegistry(), b.IsFeatureFlagOn)
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

	b.OnBuildStageStartFn.Call(BuildStagePrepareExecutor)
	defer b.OnBuildStageEndFn.Call(BuildStagePrepareExecutor)

	section := helpers.BuildSection{
		Name:        string(BuildStagePrepareExecutor),
		SkipMetrics: !b.Job.Features.TraceSections,
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

func (b *Build) GetDefaultVariables() spec.Variables {
	return spec.Variables{
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

func (b *Build) GetDefaultFeatureFlagsVariables() spec.Variables {
	variables := make(spec.Variables, 0)
	for _, featureFlag := range featureflags.GetAll() {
		variables = append(variables, spec.Variable{
			Key:      featureFlag.Name,
			Value:    strconv.FormatBool(featureFlag.DefaultValue),
			Public:   true,
			Internal: true,
			File:     false,
		})
	}

	return variables
}

func (b *Build) GetSharedEnvVariable() spec.Variable {
	env := spec.Variable{Value: "true", Public: true, Internal: true, File: false}
	if b.IsSharedEnv() {
		env.Key = "CI_SHARED_ENVIRONMENT"
	} else {
		env.Key = "CI_DISPOSABLE_ENVIRONMENT"
	}

	return env
}

func (b *Build) GetCITLSVariables() spec.Variables {
	variables := spec.Variables{}

	if b.TLSData.CAChain != "" {
		variables = append(variables, spec.Variable{
			Key:      tls.VariableCAFile,
			Value:    b.TLSData.CAChain,
			Public:   true,
			Internal: true,
			File:     true,
		})
	}

	if b.TLSData.AuthCert != "" && b.TLSData.AuthKey != "" {
		variables = append(
			variables,
			spec.Variable{
				Key:      tls.VariableCertFile,
				Value:    b.TLSData.AuthCert,
				Public:   true,
				Internal: true,
				File:     true,
			},
			spec.Variable{
				Key:      tls.VariableKeyFile,
				Value:    b.TLSData.AuthKey,
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

// getBaseVariablesBeforeJob returns the base variables that come before job variables.
func (b *Build) getBaseVariablesBeforeJob() spec.Variables {
	variables := make(spec.Variables, 0)

	if b.Image.Name != "" {
		variables = append(
			variables,
			spec.Variable{Key: "CI_JOB_IMAGE", Value: b.Image.Name, Public: true, Internal: true, File: false},
		)
	}
	if b.Runner != nil {
		variables = append(variables, b.Runner.GetVariables()...)
	}
	variables = append(variables, b.GetDefaultVariables()...)
	variables = append(variables, b.GetCITLSVariables()...)

	return variables
}

// getBaseVariablesAfterJob returns the base variables that come after job variables.
func (b *Build) getBaseVariablesAfterJob() spec.Variables {
	variables := make(spec.Variables, 0)

	variables = append(variables, b.GetSharedEnvVariable())
	variables = append(variables, AppVersion.Variables()...)
	variables = append(variables, b.secretsVariables...)

	variables = append(variables, spec.Variable{
		Key: spec.TempProjectDirVariableKey, Value: b.TmpProjectDir(), Public: true, Internal: true,
	})

	return variables
}

// getVariablesForFeatureFlagResolution returns an initial set of variables that will be used
// to resolve feature flag settings. This is used only during initSettings.
func (b *Build) getVariablesForFeatureFlagResolution() spec.Variables {
	variables := make(spec.Variables, 0)

	variables = append(variables, b.GetDefaultFeatureFlagsVariables()...)
	variables = append(variables, b.getBaseVariablesBeforeJob()...)
	variables = append(variables, b.Variables...)
	variables = append(variables, b.getBaseVariablesAfterJob()...)

	return variables.Expand()
}

// getResolvedFeatureFlags returns resolved feature flags with TOML precedence.
// This assumes build settings have been initialized. This is
// part of the two-phase feature flag resolution process that ensures
// TOML settings take precedence over job variables.
func (b *Build) getResolvedFeatureFlags() spec.Variables {
	variables := make(spec.Variables, 0)

	if b.buildSettings == nil {
		logrus.Warn("build settings are not initialized")
		return variables
	}

	for _, featureFlag := range featureflags.GetAll() {
		resolvedValue := b.buildSettings.FeatureFlags[featureFlag.Name]
		variables = append(variables, spec.Variable{
			Key:      featureFlag.Name,
			Value:    strconv.FormatBool(resolvedValue),
			Public:   true,
			Internal: true,
			File:     false,
		})
	}

	return variables
}

// getNonFeatureFlagJobVariables gets job variables, excluding feature flags to prevent double inclusion
// and to maintain the precedence of TOML-configured feature flags over job variables.
func (b *Build) getNonFeatureFlagJobVariables() spec.Variables {
	featureFlagNames := make(map[string]bool)
	for _, ff := range featureflags.GetAll() {
		featureFlagNames[ff.Name] = true
	}

	filtered := make(spec.Variables, 0, len(b.Variables))
	for _, variable := range b.Variables {
		if !featureFlagNames[variable.Key] {
			filtered = append(filtered, variable)
		}
	}

	return filtered
}

// GetAllVariables() returns final variables with a consistent precedence order:
// 1. Resolved feature flags (TOML takes precedence over job variables)
// 2. Base variables that come before job variables
// 3. Job variables (excluding feature flags to prevent overriding resolved values)
// 4. Base variables that come after job variables
func (b *Build) GetAllVariables() spec.Variables {
	if b.allVariables != nil {
		return b.allVariables
	}

	// Phase 1: Ensure feature flags have been resolved.
	if b.buildSettings == nil {
		b.Settings()
	}

	variables := make(spec.Variables, 0)

	// Phase 2: Add resolved feature flags first (maintains original precedence order)
	variables = append(variables, b.getResolvedFeatureFlags()...)
	variables = append(variables, b.getBaseVariablesBeforeJob()...)
	variables = append(variables, b.getNonFeatureFlagJobVariables()...)
	variables = append(variables, b.getBaseVariablesAfterJob()...)

	b.allVariables = variables.Expand()

	return b.allVariables
}

// IsProtected states if the git ref this build is for is protected.
// GitLab 18.3+ provides the `protected` property in GitInfo to check if a branch is protected.
// For older GitLab versions, we fall back to the CI_COMMIT_REF_PROTECTED predefined variable.
func (b *Build) IsProtected() bool {
	if p := b.GitInfo.Protected; p != nil {
		return *p
	}

	// we dedup the vars here, keeping the original, so that we don't consider an override by the user.
	return b.GetAllVariables().Dedup(true).Bool("CI_COMMIT_REF_PROTECTED")
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

// withUrlHelper lazyly sets up the correct url helper, stores it for the rest of the lifetime of the build, and returns
// the appropriate url helper.
func (b *Build) withUrlHelper() urlHelper {
	if b.urlHelper != nil {
		return b.urlHelper
	}

	urlHelperConfig := urlHelperConfig{
		CloneURL:               b.Runner.CloneURL,
		CredentialsURL:         b.Runner.RunnerCredentials.URL,
		RepoURL:                b.GitInfo.RepoURL,
		GitSubmoduleForceHTTPS: b.Settings().GitSubmoduleForceHTTPS,
		Token:                  b.Token,
		CiProjectPath:          b.GetAllVariables().Value("CI_PROJECT_PATH"),
		CiServerShellSshPort:   b.GetAllVariables().Value("CI_SERVER_SHELL_SSH_PORT"),
		CiServerShellSshHost:   b.GetAllVariables().Value("CI_SERVER_SHELL_SSH_HOST"),
		CiServerHost:           b.GetAllVariables().Value("CI_SERVER_HOST"),
	}

	urlHelper := &authenticatedURLHelper{&urlHelperConfig}
	b.urlHelper = urlHelper

	if b.IsFeatureFlagOn(featureflags.GitURLsWithoutTokens) {
		b.urlHelper = &unauthenticatedURLHelper{urlHelper}
	}

	return b.urlHelper
}

// GetRemoteURL uses the urlHelper to get the remote URL used for fetching the repo.
func (b *Build) GetRemoteURL() (*url.URL, error) {
	return b.withUrlHelper().GetRemoteURL()
}

// GetInsteadOfs uses the urlHelper to generate insteadOf URLs to pass on to git.
func (b *Build) GetInsteadOfs() ([][2]string, error) {
	return b.withUrlHelper().GetInsteadOfs()
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
	if b.GetGitStrategy() == GitNone || b.GetGitStrategy() == GitEmpty {
		return false
	}

	return b.Settings().GitCheckout
}

func (b *Build) GetSubmoduleStrategy() SubmoduleStrategy {
	if b.GetGitStrategy() == GitNone || b.GetGitStrategy() == GitEmpty {
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

func (b *Build) GetGitCloneFlags() []string {
	return b.Settings().GitCloneExtraFlags
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
	return b.startedAt
}

func (b *Build) FinishedAt() time.Time {
	return b.finishedAt
}

// CurrentDuration presents the duration since when the job was started
// to the moment when CurrentDuration was called. To be used in cases,
// when we want to check the duration of the job while it's still being
// executed
func (b *Build) CurrentDuration() time.Duration {
	return time.Since(b.startedAt)
}

// FinalDuration presents the total duration of the job since when it was
// started to when it was finished. To be used when reporting the final
// duration through logs or metrics, for example for billing purposes.
func (b *Build) FinalDuration() time.Duration {
	if b.finishedAt.IsZero() {
		return time.Duration(0)
	}

	return b.finishedAt.Sub(b.startedAt)
}

func (b *Build) ensureFinishedAt() {
	b.finishedAt = time.Now()
}

type urlHelper interface {
	GetRemoteURL() (*url.URL, error)
	GetInsteadOfs() ([][2]string, error)
}

func NewBuild(
	jobData spec.Job,
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
		Job:             jobData,
		Runner:          runnerConfigCopy,
		SystemInterrupt: systemInterrupt,
		ExecutorData:    executorData,
		startedAt:       time.Now(),
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

func (b *Build) printRunningWithHeader(trace JobTrace) {
	logger := b.getNewLogger(trace, b.Log(), false)
	defer logger.Close()

	logger.Println("Running with", AppVersion.Line())
	if b.Runner != nil && b.Runner.ShortDescription() != "" {
		logger.Println(fmt.Sprintf(
			"  on %s %s, system ID: %s",
			b.Runner.Name,
			b.Runner.ShortDescription(),
			b.Runner.SystemID,
		))
	}
	if featureInfo := b.getFeatureFlagInfo(); featureInfo != "" {
		logger.Println("  feature flags:", featureInfo)
	}
}

func (b *Build) printSettingErrors() {
	if len(b.Settings().Errors) > 0 {
		b.logger.Warningln(errors.Join(b.Settings().Errors...))
	}
}

func (b *Build) printPolicyOptions() {
	if !b.Job.PolicyOptions.PolicyJob {
		return
	}

	b.logger.Infoln(fmt.Sprintf(`Job triggered by policy "%s".`, b.Job.PolicyOptions.Name))

	// VariableOverrideAllowed is optional.
	// If not set, YAML variables defined in the policy are enforced with the highest precedence.
	if b.Job.PolicyOptions.VariableOverrideAllowed == nil {
		b.logger.Infoln("Variables defined in the policy take precedence over matching user-defined CI/CD variables for this job.")
		return
	}

	var message = "User-defined CI/CD variables are "
	if *b.Job.PolicyOptions.VariableOverrideAllowed {
		message += "allowed in this job"
	} else {
		message += "ignored in this job"
	}
	// VariableOverrideExceptions acts as an allowlist when VariableOverrideExceptions is false
	// and a denylist when it's true.
	if b.Job.PolicyOptions.VariableOverrideExceptions != nil {
		message += fmt.Sprintf(" (except for %s)", strings.Join(b.Job.PolicyOptions.VariableOverrideExceptions, ", "))
	}
	message += " according to the pipeline execution policy."
	b.logger.Infoln(message)
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

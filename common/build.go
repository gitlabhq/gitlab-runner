package common

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls"
	"gitlab.com/gitlab-org/gitlab-runner/session"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

type GitStrategy int

const (
	GitClone GitStrategy = iota
	GitFetch
	GitNone
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
	BuildRunRuntimeFinished   BuildRuntimeState = "finished"
	BuildRunRuntimeCanceled   BuildRuntimeState = "canceled"
	BuildRunRuntimeTerminated BuildRuntimeState = "terminated"
	BuildRunRuntimeTimedout   BuildRuntimeState = "timedout"
)

type BuildStage string

const (
	BuildStagePrepare                  BuildStage = "prepare_script"
	BuildStageGetSources               BuildStage = "get_sources"
	BuildStageRestoreCache             BuildStage = "restore_cache"
	BuildStageDownloadArtifacts        BuildStage = "download_artifacts"
	BuildStageUserScript               BuildStage = "build_script"
	BuildStageAfterScript              BuildStage = "after_script"
	BuildStageArchiveCache             BuildStage = "archive_cache"
	BuildStageUploadOnSuccessArtifacts BuildStage = "upload_artifacts_on_success"
	BuildStageUploadOnFailureArtifacts BuildStage = "upload_artifacts_on_failure"
)

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

	CurrentStage BuildStage
	CurrentState BuildRuntimeState

	Session *session.Session

	executorStageResolver func() ExecutorStage
	logger                BuildLogger
	allVariables          JobVariables

	createdAt time.Time
}

func (b *Build) Log() *logrus.Entry {
	return b.Runner.Log().WithField("job", b.ID).WithField("project", b.JobInfo.ProjectID)
}

func (b *Build) ProjectUniqueName() string {
	return fmt.Sprintf("runner-%s-project-%d-concurrent-%d",
		b.Runner.ShortDescription(), b.JobInfo.ProjectID, b.ProjectRunnerID)
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
			fmt.Sprintf("%s", b.Runner.ShortDescription()),
			fmt.Sprintf("%d", b.ProjectRunnerID),
			dir,
		)
	}
	return dir
}

func (b *Build) FullProjectDir() string {
	return helpers.ToSlash(b.BuildDir)
}

func (b *Build) StartBuild(rootDir, cacheDir string, sharedDir bool) {
	b.RootDir = rootDir
	b.BuildDir = path.Join(rootDir, b.ProjectUniqueDir(sharedDir))
	b.CacheDir = path.Join(cacheDir, b.ProjectUniqueDir(false))
}

func (b *Build) executeStage(ctx context.Context, buildStage BuildStage, executor Executor) error {
	b.CurrentStage = buildStage

	b.Log().WithField("build_stage", buildStage).Debug("Executing build stage")

	shell := executor.Shell()
	if shell == nil {
		return errors.New("No shell defined")
	}

	script, err := GenerateShellScript(buildStage, *shell)
	if err != nil {
		return err
	}

	// Nothing to execute
	if script == "" {
		return nil
	}

	cmd := ExecutorCommand{
		Context: ctx,
		Script:  script,
		Stage:   buildStage,
	}

	switch buildStage {
	case BuildStageUserScript, BuildStageAfterScript: // use custom build environment
		cmd.Predefined = false
	default: // all other stages use a predefined build environment
		cmd.Predefined = true
	}

	section := helpers.BuildSection{
		Name:        string(buildStage),
		SkipMetrics: !b.JobResponse.Features.TraceSections,
		Run:         func() error { return executor.Run(cmd) },
	}
	return section.Execute(&b.logger)
}

func (b *Build) executeUploadArtifacts(ctx context.Context, state error, executor Executor) (err error) {
	if state == nil {
		return b.executeStage(ctx, BuildStageUploadOnSuccessArtifacts, executor)
	}

	return b.executeStage(ctx, BuildStageUploadOnFailureArtifacts, executor)
}

func (b *Build) executeScript(ctx context.Context, executor Executor) error {
	// Prepare stage
	err := b.executeStage(ctx, BuildStagePrepare, executor)

	if err == nil {
		err = b.attemptExecuteStage(ctx, BuildStageGetSources, executor, b.GetGetSourcesAttempts())
	}
	if err == nil {
		err = b.attemptExecuteStage(ctx, BuildStageRestoreCache, executor, b.GetRestoreCacheAttempts())
	}
	if err == nil {
		err = b.attemptExecuteStage(ctx, BuildStageDownloadArtifacts, executor, b.GetDownloadArtifactsAttempts())
	}

	if err == nil {
		// Execute user build script (before_script + script)
		err = b.executeStage(ctx, BuildStageUserScript, executor)

		// Execute after script (after_script)
		timeoutContext, timeoutCancel := context.WithTimeout(ctx, AfterScriptTimeout)
		defer timeoutCancel()

		b.executeStage(timeoutContext, BuildStageAfterScript, executor)
	}

	// Execute post script (cache store, artifacts upload)
	if err == nil {
		err = b.executeStage(ctx, BuildStageArchiveCache, executor)
	}

	uploadError := b.executeUploadArtifacts(ctx, err, executor)

	// Use job's error as most important
	if err != nil {
		return err
	}

	// Otherwise, use uploadError
	return uploadError
}

func (b *Build) attemptExecuteStage(ctx context.Context, buildStage BuildStage, executor Executor, attempts int) (err error) {
	if attempts < 1 || attempts > 10 {
		return fmt.Errorf("Number of attempts out of the range [1, 10] for stage: %s", buildStage)
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
	switch err {
	case context.Canceled:
		b.CurrentState = BuildRunRuntimeCanceled
		return &BuildError{Inner: errors.New("canceled")}

	case context.DeadlineExceeded:
		b.CurrentState = BuildRunRuntimeTimedout
		return &BuildError{
			Inner:         fmt.Errorf("execution took longer than %v seconds", b.GetBuildTimeout()),
			FailureReason: JobExecutionTimeout,
		}

	default:
		b.CurrentState = BuildRunRuntimeFinished
		return err
	}
}

func (b *Build) run(ctx context.Context, executor Executor) (err error) {
	b.CurrentState = BuildRunRuntimeRunning

	buildFinish := make(chan error, 1)

	runContext, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	if term, ok := executor.(terminal.InteractiveTerminal); b.Session != nil && ok {
		b.Session.SetInteractiveTerminal(term)
	}

	// Run build script
	go func() {
		buildFinish <- b.executeScript(runContext, executor)
	}()

	// Wait for signals: cancel, timeout, abort or finish
	b.Log().Debugln("Waiting for signals...")
	select {
	case <-ctx.Done():
		err = b.handleError(ctx.Err())

	case signal := <-b.SystemInterrupt:
		err = fmt.Errorf("aborted: %v", signal)
		b.CurrentState = BuildRunRuntimeTerminated

	case err = <-buildFinish:
		b.CurrentState = BuildRunRuntimeFinished
		return err
	}

	b.Log().WithError(err).Debugln("Waiting for build to finish...")

	// Wait till we receive that build did finish
	runCancel()
	<-buildFinish
	return err
}

func (b *Build) retryCreateExecutor(options ExecutorPrepareOptions, provider ExecutorProvider, logger BuildLogger) (executor Executor, err error) {
	for tries := 0; tries < PreparationRetries; tries++ {
		executor = provider.Create()
		if executor == nil {
			err = errors.New("failed to create executor")
			return
		}

		b.executorStageResolver = executor.GetCurrentStage

		err = executor.Prepare(options)
		if err == nil {
			break
		}
		if executor != nil {
			executor.Cleanup()
			executor = nil
		}
		if _, ok := err.(*BuildError); ok {
			break
		} else if options.Context.Err() != nil {
			return nil, b.handleError(options.Context.Err())
		}

		logger.SoftErrorln("Preparation failed:", err)
		logger.Infoln("Will be retried in", PreparationRetryInterval, "...")
		time.Sleep(PreparationRetryInterval)
	}
	return
}

func (b *Build) waitForTerminal(ctx context.Context, timeout time.Duration) error {
	if b.Session == nil || !b.Session.Connected() {
		return nil
	}

	timeout = b.getTerminalTimeout(ctx, timeout)

	b.logger.Infoln(
		fmt.Sprintf(
			"Terminal is connected, will time out in %s...",
			// TODO: switch to timeout.Round(time.Second) after upgrading to Go 1.9+
			roundDuration(timeout, time.Second),
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
			"Terminal session timed out (maximum time allowed - %s)",
			// TODO: switch to timeout.Round(time.Second) after upgrading to Go 1.9+
			roundDuration(timeout, time.Second),
		)
		b.logger.Infoln(err.Error())
		b.Session.TimeoutCh <- err
		return err
	case err := <-b.Session.DisconnectCh:
		b.logger.Infoln("Terminal disconnected")
		return fmt.Errorf("terminal disconnected: %v", err)
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
		timeout = expiryTime.Sub(time.Now())
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

func (b *Build) CurrentExecutorStage() ExecutorStage {
	if b.executorStageResolver == nil {
		b.executorStageResolver = func() ExecutorStage {
			return ExecutorStage("")
		}
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

	b.CurrentState = BuildRunStatePending

	defer func() {
		b.setTraceStatus(trace, err)

		if executor != nil {
			executor.Cleanup()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), b.GetBuildTimeout())
	defer cancel()

	trace.SetCancelFunc(cancel)

	options := ExecutorPrepareOptions{
		Config:  b.Runner,
		Build:   b,
		Trace:   trace,
		User:    globalConfig.User,
		Context: ctx,
	}

	provider := GetExecutor(b.Runner.Executor)
	if provider == nil {
		return errors.New("executor not found")
	}

	provider.GetFeatures(&b.ExecutorFeatures)

	executor, err = b.retryCreateExecutor(options, provider, b.logger)
	if err == nil {
		err = b.run(ctx, executor)
		if err := b.waitForTerminal(ctx, globalConfig.SessionServer.GetSessionTimeout()); err != nil {
			b.Log().WithError(err).Debug("Stopped waiting for terminal")
		}
	}

	if executor != nil {
		executor.Finish(err)
	}

	return err
}

func (b *Build) String() string {
	return helpers.ToYAML(b)
}

func (b *Build) GetDefaultVariables() JobVariables {
	return JobVariables{
		{Key: "CI_PROJECT_DIR", Value: filepath.FromSlash(b.FullProjectDir()), Public: true, Internal: true, File: false},
		{Key: "CI_SERVER", Value: "yes", Public: true, Internal: true, File: false},
	}
}

func (b *Build) GetDefaultFeatureFlagsVariables() JobVariables {
	return JobVariables{
		{Key: "FF_K8S_USE_ENTRYPOINT_OVER_COMMAND", Value: "true", Public: true, Internal: true, File: false}, // TODO: Remove in 12.0
	}
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
		variables = append(variables, JobVariable{tls.VariableCAFile, b.TLSCAChain, true, true, true})
	}
	if b.TLSAuthCert != "" && b.TLSAuthKey != "" {
		variables = append(variables, JobVariable{tls.VariableCertFile, b.TLSAuthCert, true, true, true})
		variables = append(variables, JobVariable{tls.VariableKeyFile, b.TLSAuthKey, true, true, true})
	}
	return variables
}

func (b *Build) GetGitTLSVariables() JobVariables {
	variables := JobVariables{}
	if b.TLSCAChain != "" {
		variables = append(variables, JobVariable{"GIT_SSL_CAINFO", b.TLSCAChain, true, true, true})
	}
	if b.TLSAuthCert != "" && b.TLSAuthKey != "" {
		variables = append(variables, JobVariable{"GIT_SSL_CERT", b.TLSAuthCert, true, true, true})
		variables = append(variables, JobVariable{"GIT_SSL_KEY", b.TLSAuthKey, true, true, true})
	}
	return variables
}

func (b *Build) IsSharedEnv() bool {
	return b.ExecutorFeatures.Shared
}

func (b *Build) GetAllVariables() JobVariables {
	if b.allVariables != nil {
		return b.allVariables
	}

	variables := make(JobVariables, 0)
	if b.Runner != nil {
		variables = append(variables, b.Runner.GetVariables()...)
	}
	variables = append(variables, b.GetDefaultVariables()...)
	variables = append(variables, b.GetDefaultFeatureFlagsVariables()...)
	variables = append(variables, b.GetCITLSVariables()...)
	variables = append(variables, b.Variables...)
	variables = append(variables, b.GetSharedEnvVariable())
	variables = append(variables, AppVersion.Variables()...)

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

func (b *Build) GetGitDepth() string {
	return b.GetAllVariables().Get("GIT_DEPTH")
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
	if len(strCheckout) == 0 {
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

func (b *Build) IsDebugTraceEnabled() bool {
	trace, err := strconv.ParseBool(b.GetAllVariables().Get("CI_DEBUG_TRACE"))
	if err != nil {
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

func (b *Build) Duration() time.Duration {
	return time.Since(b.createdAt)
}

func NewBuild(jobData JobResponse, runnerConfig *RunnerConfig, systemInterrupt chan os.Signal, executorData ExecutorData) (*Build, error) {
	// Attempt to perform a deep copy of the RunnerConfig
	runnerConfigCopy, err := runnerConfig.DeepCopy()
	if err != nil {
		return nil, fmt.Errorf("deep copy of runner config failed: %v", err)
	}

	return &Build{
		JobResponse:     jobData,
		Runner:          runnerConfigCopy,
		SystemInterrupt: systemInterrupt,
		ExecutorData:    executorData,
		createdAt:       time.Now(),
	}, nil
}

func (b *Build) IsFeatureFlagOn(name string) bool {
	ffValue := b.GetAllVariables().Get(name)

	if ffValue == "" {
		return false
	}

	on, err := strconv.ParseBool(ffValue)
	if err != nil {
		logrus.WithError(err).
			WithField("ffName", name).
			WithField("ffValue", ffValue).
			Error("Error while parsing the value of feature flag")

		return false
	}

	return on
}

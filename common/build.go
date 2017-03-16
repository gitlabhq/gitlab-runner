package common

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers"
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
	BuildRunRuntimeRunning                      = "running"
	BuildRunRuntimeFinished                     = "finished"
	BuildRunRuntimeCanceled                     = "canceled"
	BuildRunRuntimeTerminated                   = "terminated"
	BuildRunRuntimeTimedout                     = "timedout"
)

type BuildStage string

const (
	BuildStagePrepare           BuildStage = "prepare_script"
	BuildStageGetSources                   = "get_sources"
	BuildStageRestoreCache                 = "restore_cache"
	BuildStageDownloadArtifacts            = "download_artifacts"
	BuildStageUserScript                   = "build_script"
	BuildStageAfterScript                  = "after_script"
	BuildStageArchiveCache                 = "archive_cache"
	BuildStageUploadArtifacts              = "upload_artifacts"
)

type Build struct {
	JobResponse `yaml:",inline"`

	Trace           JobTrace
	SystemInterrupt chan os.Signal `json:"-" yaml:"-"`
	RootDir         string         `json:"-" yaml:"-"`
	BuildDir        string         `json:"-" yaml:"-"`
	CacheDir        string         `json:"-" yaml:"-"`
	Hostname        string         `json:"-" yaml:"-"`
	Runner          *RunnerConfig  `json:"runner"`
	ExecutorData    ExecutorData

	// Unique ID for all running builds on this runner
	RunnerID int `json:"runner_id"`

	// Unique ID for all running builds on this runner and this project
	ProjectRunnerID int `json:"project_runner_id"`

	CurrentStage BuildStage
	CurrentState BuildRuntimeState
}

func (b *Build) Log() *logrus.Entry {
	return b.Runner.Log().WithField("job", b.ID).WithField("project", b.JobInfo.ProjectID)
}

func (b *Build) ProjectUniqueName() string {
	return fmt.Sprintf("runner-%s-project-%d-concurrent-%d",
		b.Runner.ShortDescription(), b.JobInfo.ProjectID, b.ProjectRunnerID)
}

func (b *Build) ProjectSlug() (string, error) {
	url, err := url.Parse(b.RepoURL)
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

func (b *Build) executeStage(buildStage BuildStage, executor Executor, abort chan interface{}) error {
	b.CurrentStage = buildStage

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
		Script: script,
		Abort:  abort,
	}

	switch buildStage {
	case BuildStageUserScript, BuildStageAfterScript: // use custom build environment
		cmd.Predefined = false
	default: // all other stages use a predefined build environment
		cmd.Predefined = true
	}

	return executor.Run(cmd)
}

func (b *Build) executeUploadArtifacts(state error, executor Executor, abort chan interface{}) (err error) {
	when, _ := b.Options.GetString("artifacts", "when")

	if state == nil {
		// Previous stages were successful
		if when == "" || when == "on_success" || when == "always" {
			err = b.executeStage(BuildStageUploadArtifacts, executor, abort)
		}
	} else {
		// Previous stage did fail
		if when == "on_failure" || when == "always" {
			err = b.executeStage(BuildStageUploadArtifacts, executor, abort)
		}
	}

	// Use previous error if set
	if state != nil {
		err = state
	}
	return
}

func (b *Build) executeScript(executor Executor, abort chan interface{}) error {
	// Prepare stage
	err := b.executeStage(BuildStagePrepare, executor, abort)

	if err == nil {
		err = b.attemptExecuteStage(BuildStageGetSources, executor, abort, b.GetGetSourcesAttempts())
	}
	if err == nil {
		err = b.attemptExecuteStage(BuildStageDownloadArtifacts, executor, abort, b.GetDownloadArtifactsAttempts())
	}
	if err == nil {
		err = b.attemptExecuteStage(BuildStageRestoreCache, executor, abort, b.GetRestoreCacheAttempts())
	}

	if err == nil {
		// Execute user build script (before_script + script)
		err = b.executeStage(BuildStageUserScript, executor, abort)

		// Execute after script (after_script)
		timeoutCh := make(chan interface{}, 1)
		timeout := time.AfterFunc(time.Minute*5, func() {
			close(timeoutCh)
		})
		b.executeStage(BuildStageAfterScript, executor, timeoutCh)
		timeout.Stop()
	}

	// Execute post script (cache store, artifacts upload)
	if err == nil {
		err = b.executeStage(BuildStageArchiveCache, executor, abort)
	}
	err = b.executeUploadArtifacts(err, executor, abort)
	return err
}

func (b *Build) attemptExecuteStage(buildStage BuildStage, executor Executor, abort chan interface{}, attempts int) (err error) {
	if attempts < 1 || attempts > 10 {
		return fmt.Errorf("Number of attempts out of the range [1, 10] for stage: %s", buildStage)
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if err = b.executeStage(buildStage, executor, abort); err == nil {
			return
		}
	}
	return
}

func (b *Build) run(executor Executor) (err error) {
	b.CurrentState = BuildRunRuntimeRunning

	buildTimeout := b.Timeout
	if buildTimeout <= 0 {
		buildTimeout = DefaultTimeout
	}

	buildFinish := make(chan error, 1)
	buildAbort := make(chan interface{})

	// Run build script
	go func() {
		buildFinish <- b.executeScript(executor, buildAbort)
	}()

	// Wait for signals: cancel, timeout, abort or finish
	b.Log().Debugln("Waiting for signals...")
	select {
	case <-b.Trace.Aborted():
		err = &BuildError{Inner: errors.New("canceled")}
		b.CurrentStage = BuildRunRuntimeCanceled

	case <-time.After(time.Duration(buildTimeout) * time.Second):
		err = &BuildError{Inner: fmt.Errorf("execution took longer than %v seconds", buildTimeout)}
		b.CurrentStage = BuildRunRuntimeTimedout

	case signal := <-b.SystemInterrupt:
		err = fmt.Errorf("aborted: %v", signal)
		b.CurrentStage = BuildRunRuntimeTerminated

	case err = <-buildFinish:
		b.CurrentState = BuildRunRuntimeFinished
		return err
	}

	b.Log().WithError(err).Debugln("Waiting for build to finish...")

	// Wait till we receive that build did finish
	for {
		select {
		case buildAbort <- true:
		case <-buildFinish:
			return err
		}
	}
}

func (b *Build) retryCreateExecutor(globalConfig *Config, provider ExecutorProvider, logger BuildLogger) (executor Executor, err error) {
	for tries := 0; tries < PreparationRetries; tries++ {
		executor = provider.Create()
		if executor == nil {
			err = errors.New("failed to create executor")
			return
		}

		err = executor.Prepare(globalConfig, b.Runner, b)
		if err == nil {
			break
		}
		if executor != nil {
			executor.Cleanup()
			executor = nil
		}
		if _, ok := err.(*BuildError); ok {
			break
		}

		logger.SoftErrorln("Preparation failed:", err)
		logger.Infoln("Will be retried in", PreparationRetryInterval, "...")
		time.Sleep(PreparationRetryInterval)
	}
	return
}

func (b *Build) Run(globalConfig *Config, trace JobTrace) (err error) {
	var executor Executor

	logger := NewBuildLogger(trace, b.Log())
	logger.Println(fmt.Sprintf("Running with %s\n  on %s (%s)", AppVersion.Line(), b.Runner.Name, b.Runner.ShortDescription()))

	b.CurrentState = BuildRunStatePending

	defer func() {
		if _, ok := err.(*BuildError); ok {
			logger.SoftErrorln("Job failed:", err)
			trace.Fail(err)
		} else if err != nil {
			logger.Errorln("Job failed (system failure):", err)
			trace.Fail(err)
		} else {
			logger.Infoln("Job succeeded")
			trace.Success()
		}
		if executor != nil {
			executor.Cleanup()
		}
	}()

	b.Trace = trace

	provider := GetExecutor(b.Runner.Executor)
	if provider == nil {
		return errors.New("executor not found")
	}

	executor, err = b.retryCreateExecutor(globalConfig, provider, logger)
	if err == nil {
		err = b.run(executor)
	}
	if executor != nil {
		executor.Finish(err)
	}
	return err
}

func (b *Build) String() string {
	return helpers.ToYAML(b)
}

func (b *Build) GetDefaultVariables() BuildVariables {
	return BuildVariables{
		{"CI", "true", true, true, false},
		{"CI_DEBUG_TRACE", "false", true, true, false},
		{"CI_BUILD_REF", b.Sha, true, true, false},
		{"CI_BUILD_BEFORE_SHA", b.BeforeSha, true, true, false},
		{"CI_BUILD_REF_NAME", b.RefName, true, true, false},
		{"CI_BUILD_ID", strconv.Itoa(b.ID), true, true, false},
		{"CI_BUILD_REPO", b.RepoURL, true, true, false},
		{"CI_BUILD_TOKEN", b.Token, true, true, false},
		{"CI_PROJECT_ID", strconv.Itoa(b.JobInfo.ProjectID), true, true, false},
		{"CI_PROJECT_DIR", b.FullProjectDir(), true, true, false},
		{"CI_SERVER", "yes", true, true, false},
		{"CI_SERVER_NAME", "GitLab CI", true, true, false},
		{"CI_SERVER_VERSION", "", true, true, false},
		{"CI_SERVER_REVISION", "", true, true, false},
		{"GITLAB_CI", "true", true, true, false},
	}
}

func (b *Build) GetAllVariables() (variables BuildVariables) {
	if b.Runner != nil {
		variables = append(variables, b.Runner.GetVariables()...)
	}
	variables = append(variables, b.GetDefaultVariables()...)
	variables = append(variables, b.Variables...)
	return variables.Expand()
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

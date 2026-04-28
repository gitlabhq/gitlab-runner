package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/stages"
	"gitlab.com/gitlab-org/step-runner/pkg/runner"
)

const afterScriptStepName = "after_script"

var (
	ErrJobScriptTimeout = errors.New("job script timeout exceeded")
	ErrJobCanceled      = errors.New("job canceled")
)

type ExitError struct {
	Inner    error
	ExitCode int
}

func (e *ExitError) Error() string { return e.Inner.Error() }
func (e *ExitError) Unwrap() error { return e.Inner }

type Runner struct {
	config *Config
	env    *env.Env

	mu           sync.Mutex
	scriptCancel context.CancelFunc
}

type Option func(*Runner) error

type Config struct {
	CacheDir                string        `json:"cache_dir,omitempty"`
	ArchiverStagingDir      string        `json:"archiver_staging_dir,omitempty"`
	Shell                   string        `json:"shell,omitempty"`
	LoginShell              bool          `json:"login_shell,omitempty"`
	Timeout                 time.Duration `json:"timeout,omitempty"`
	ScriptTimeout           time.Duration `json:"script_timeout,omitempty"`
	AfterScriptTimeout      time.Duration `json:"after_script_timeout,omitempty"`
	AfterScriptIgnoreErrors bool          `json:"after_script_ignore_errors,omitempty"`
	ID                      int64         `json:"id,omitempty"`
	Token                   string        `json:"token,omitempty"`
	BaseURL                 string        `json:"base_url,omitempty"`

	GetSources       stages.GetSources         `json:"get_sources,omitempty"`
	CacheExtract     []stages.CacheExtract     `json:"cache_extract,omitempty"`
	ArtifactExtract  []stages.ArtifactDownload `json:"artifact_extract,omitempty"`
	Steps            []stages.Step             `json:"steps,omitempty"`
	CacheArchive     []stages.CacheArchive     `json:"cache_archive,omitempty"`
	ArtifactsArchive []stages.ArtifactUpload   `json:"artifacts_archive,omitempty"`
	Cleanup          stages.Cleanup            `json:"cleanup,omitempty"`
}

func New(config Config, builtinCtx runner.BuiltinContext, options ...Option) (*Runner, error) {
	stepEnv := builtinCtx.GetEnvs()
	for key, value := range builtinCtx.GetJobVars() {
		stepEnv[key] = value.GetStringValue()
	}

	stdout, stderr := builtinCtx.Pipe()

	e := &env.Env{
		ID:                config.ID,
		Token:             config.Token,
		BaseURL:           config.BaseURL,
		WorkingDir:        builtinCtx.WorkDir(),
		CacheDir:          config.CacheDir,
		StagingDir:        config.ArchiverStagingDir,
		Shell:             config.Shell,
		Timeout:           config.Timeout,
		LoginShell:        config.LoginShell,
		GracefulExitDelay: builtinCtx.GracefulExitDelay(),
		GitLabEnv:         make(map[string]string),
		Env:               stepEnv,
		Stdout:            stdout,
		Stderr:            stderr,
	}

	e.Env["CI_JOB_STATUS"] = string(env.Running)
	r := &Runner{config: &config, env: e}

	for _, opt := range options {
		if err := opt(r); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// Cancel cancels the currently running script phase. During prepare or
// user scripts, the relevant context is canceled. During after_script,
// Cancel is a no-op, ensuring that it and the remaining build stages
// continue.
func (r *Runner) Cancel() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.scriptCancel != nil {
		r.scriptCancel()
	}
}

// Run executes the full job lifecycle.
func (r *Runner) Run(ctx context.Context) error {
	jobCtx, jobCancel := r.withTimeout(ctx, r.config.Timeout)
	defer jobCancel()
	defer r.cleanup()

	// Before user scripts, Cancel() cancels the entire job.
	r.setCancel(jobCancel)

	if err := r.setupGitlabEnv(); err != nil {
		return fmt.Errorf("setting up GITLAB_ENV: %w", err)
	}

	if err := r.prepare(jobCtx); err != nil {
		return err
	}

	scriptErr := r.executeSteps(jobCtx)
	cacheErr, artifactErr := r.finalize(jobCtx)

	return pickPriorityError(scriptErr, cacheErr, artifactErr)
}

func (r *Runner) setCancel(cancel context.CancelFunc) {
	r.mu.Lock()
	r.scriptCancel = cancel
	r.mu.Unlock()
}

//nolint:gocognit
func (r *Runner) prepare(ctx context.Context) error {
	if err := r.section(ctx, "get_sources", r.config.GetSources.Run); err != nil {
		return fmt.Errorf("fetching sources: %w", err)
	}

	if hasCacheSources(r.config.CacheExtract) {
		_ = r.section(ctx, "restore_cache", func(ctx context.Context, e *env.Env) error {
			for _, cache := range r.config.CacheExtract {
				if len(cache.Sources) == 0 {
					continue
				}
				if err := cache.Run(ctx, e); err != nil {
					r.logWarningf("Failed to restore cache %q: %v", cache.Sources[0].Key, err)
				}
			}
			return nil
		})
	}

	if len(r.config.ArtifactExtract) > 0 {
		_ = r.section(ctx, "download_artifacts", func(ctx context.Context, e *env.Env) error {
			for _, artifact := range r.config.ArtifactExtract {
				if err := artifact.Run(ctx, e); err != nil {
					r.logWarningf("Failed to download artifact %q: %v", artifact.ArtifactName, err)
				}
			}
			return nil
		})
	}

	return nil
}

func hasCacheSources(extracts []stages.CacheExtract) bool {
	for _, c := range extracts {
		if len(c.Sources) > 0 {
			return true
		}
	}
	return false
}

// statusFromError mirrors build.go's runtimeStateAndError classification,
// mapping the script error into the appropriate CI_JOB_STATUS value.
func statusFromError(err error) env.JobStatus {
	switch {
	case err == nil:
		return env.Success
	case errors.Is(err, ErrJobScriptTimeout), errors.Is(err, context.DeadlineExceeded):
		return env.Timedout
	case errors.Is(err, ErrJobCanceled), errors.Is(err, context.Canceled):
		return env.Canceled
	default:
		return env.Failed
	}
}

// executeSteps runs all steps, switching from the script timeout to the
// after-script timeout at the after_script boundary.
func (r *Runner) executeSteps(jobCtx context.Context) error {
	scriptSteps, afterSteps := r.config.Steps, []stages.Step(nil)
	for i, step := range r.config.Steps {
		if step.Step == afterScriptStepName {
			scriptSteps, afterSteps = r.config.Steps[:i], r.config.Steps[i:]
			break
		}
	}

	err := r.runScriptSteps(jobCtx, scriptSteps)
	r.env.SetStatus(statusFromError(err))

	return r.runAfterScriptSteps(jobCtx, afterSteps, err)
}

func (r *Runner) runScriptSteps(jobCtx context.Context, steps []stages.Step) error {
	scriptCtx, cancel := r.withTimeout(jobCtx, r.config.ScriptTimeout)
	defer cancel()

	r.setCancel(cancel)

	var firstErr error
	for _, step := range steps {
		r.loadGitlabEnv()

		if err := r.section(scriptCtx, "step_"+step.Step, step.Run); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			break
		}
	}

	return r.classifyScriptContextError(jobCtx, scriptCtx, firstErr)
}

// runAfterScriptSteps executes after_script steps under their own timeout.
// It may promote an after-script error into *scriptErr when appropriate.
func (r *Runner) runAfterScriptSteps(jobCtx context.Context, steps []stages.Step, err error) error {
	if len(steps) == 0 {
		r.setCancel(nil)
		return err
	}

	afterCtx, cancel := r.withTimeout(jobCtx, r.config.AfterScriptTimeout)
	defer cancel()

	// Cancel() is a no-op during after_script, matching build.go behavior:
	// the trace cancel func is only set to the script context's cancel and
	// is never updated for after_script.
	r.setCancel(nil)

	_ = r.section(afterCtx, "after_script", func(ctx context.Context, e *env.Env) error {
		for _, step := range steps {
			r.loadGitlabEnv()

			afterErr := step.Run(ctx, e)
			if afterErr == nil {
				continue
			}

			// If the overall job deadline expired, stop immediately.
			if errors.Is(jobCtx.Err(), context.DeadlineExceeded) {
				break
			}

			if r.config.AfterScriptIgnoreErrors {
				r.logWarningf("after_script failed, but job will continue unaffected: %v", afterErr)
			} else if err == nil {
				err = afterErr
			}
		}
		return nil
	})

	return err
}

//nolint:gocognit
func (r *Runner) finalize(ctx context.Context) (cacheErr, artifactErr error) {
	r.loadGitlabEnv()

	cacheSection := "archive_cache"
	uploadSection := "upload_artifacts_on_success"
	if !r.env.IsSuccessful() {
		cacheSection = "archive_cache_on_failure"
		uploadSection = "upload_artifacts_on_failure"
	}

	if len(r.config.CacheArchive) > 0 {
		_ = r.section(ctx, cacheSection, func(ctx context.Context, e *env.Env) error {
			for _, cache := range r.config.CacheArchive {
				if err := cache.Run(ctx, e); err != nil {
					r.logWarningf("Failed to archive cache %q: %v", cache.Key, err)
					if cacheErr == nil {
						cacheErr = err
					}
				}
			}
			return nil
		})
	}

	// Mirror abstract's writeUploadArtifacts ErrSkipBuildStage guard: with
	// no server URL there is nowhere to upload to, so skip the section
	// entirely rather than invoking artifacts-uploader with --url "".
	if len(r.config.ArtifactsArchive) > 0 && r.env.BaseURL != "" {
		_ = r.section(ctx, uploadSection, func(ctx context.Context, e *env.Env) error {
			for _, artifact := range r.config.ArtifactsArchive {
				if err := artifact.Run(ctx, e); err != nil {
					r.logWarningf("Failed to upload artifact %q: %v", artifact.ArtifactName, err)
					if artifactErr == nil {
						artifactErr = err
					}
				}
			}
			return nil
		})
	}

	return cacheErr, artifactErr
}

// classifyScriptContextError checks whether a cancellation or script-level
// timeout occurred, and wraps the error accordingly. It snapshots both
// context errors atomically to avoid TOCTOU races.
func (r *Runner) classifyScriptContextError(jobCtx, scriptCtx context.Context, err error) error {
	jobErr := jobCtx.Err()
	scriptCtxErr := scriptCtx.Err()

	switch {
	case jobErr == nil && errors.Is(scriptCtxErr, context.Canceled):
		r.logWarningf("Script canceled externally (UI, API)")
		return &ExitError{Inner: ErrJobCanceled, ExitCode: 1}

	case !errors.Is(jobErr, context.DeadlineExceeded) &&
		errors.Is(scriptCtxErr, context.DeadlineExceeded):
		return &ExitError{
			Inner:    fmt.Errorf("%w: %w", ErrJobScriptTimeout, scriptCtxErr),
			ExitCode: 1,
		}
	}

	return err
}

func pickPriorityError(scriptErr, cacheErr, artifactErr error) error {
	switch {
	case scriptErr != nil:
		return scriptErr
	case cacheErr != nil:
		return cacheErr
	default:
		return artifactErr
	}
}

// withTimeout returns a derived context with a deadline when d > 0,
// or a plain cancelable context when d is zero.
func (r *Runner) withTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d > 0 {
		return context.WithTimeout(parent, d)
	}
	return context.WithCancel(parent)
}

func (r *Runner) section(ctx context.Context, name string, fn func(context.Context, *env.Env) error) error {
	fmt.Fprintf(r.env.Stdout, "section_start:%d:%s\r\033[0K", time.Now().Unix(), name)
	defer fmt.Fprintf(r.env.Stdout, "section_end:%d:%s\r\033[0K", time.Now().Unix(), name)

	return fn(ctx, r.env)
}

func (r *Runner) logWarningf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(r.env.Stderr, " %s\033[0m\n", msg)
}

// setupGitlabEnv creates the GITLAB_ENV file so user scripts can append
// KEY=VALUE lines to define dynamic variables for subsequent steps.
func (r *Runner) setupGitlabEnv() error {
	tmpDir := r.env.WorkingDir + ".tmp"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return err
	}

	envFile := filepath.Join(tmpDir, "gitlab_runner_env")
	if err := os.WriteFile(envFile, nil, 0o600); err != nil {
		return err
	}

	r.env.GitLabEnvFile = envFile
	r.env.GitLabEnv["GITLAB_ENV"] = envFile

	return nil
}

// loadGitlabEnv rebuilds the dynamic variable overlay from the GITLAB_ENV
// file. The file is the sole source of truth, so the overlay is recreated
// from scratch each time.
func (r *Runner) loadGitlabEnv() {
	if r.env.GitLabEnvFile == "" {
		return
	}

	overlay := map[string]string{
		"GITLAB_ENV": r.env.GitLabEnvFile,
	}

	data, err := os.ReadFile(r.env.GitLabEnvFile)
	if err != nil {
		r.logWarningf("Failed to read GITLAB_ENV file: %v", err)
		return
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			overlay[k] = v
		}
	}

	r.env.GitLabEnv = overlay
}

func (r *Runner) cleanup() {
	if err := r.config.Cleanup.Run(context.Background(), r.env); err != nil {
		r.logWarningf("Cleanup failed: %v", err)
	}

	if r.env.GitLabEnvFile != "" {
		os.Remove(r.env.GitLabEnvFile)
	}
}

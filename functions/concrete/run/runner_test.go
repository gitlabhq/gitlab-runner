//go:build !integration

package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/stages"
)

func testRunner(t *testing.T, cfg *Config) *Runner {
	t.Helper()

	if cfg == nil {
		cfg = &Config{}
	}

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	require.NoError(t, os.MkdirAll(workDir, 0o755))

	shell := "bash"
	if runtime.GOOS == "windows" {
		shell = "powershell"
	}

	e := &env.Env{
		WorkingDir: workDir,
		Shell:      shell,
		Env: map[string]string{
			"CI_JOB_STATUS": string(env.Running),
		},
		GitLabEnv: map[string]string{},
		Stdout:    &bytes.Buffer{},
		Stderr:    &bytes.Buffer{},
	}

	return &Runner{config: cfg, env: e}
}

func runnerStdout(r *Runner) string {
	return r.env.Stdout.(*bytes.Buffer).String()
}

func runnerStderr(r *Runner) string {
	return r.env.Stderr.(*bytes.Buffer).String()
}

func TestPickPriorityError(t *testing.T) {
	errScript := errors.New("script")
	errCache := errors.New("cache")
	errArtifact := errors.New("artifact")

	tests := []struct {
		name       string
		script     error
		cache      error
		artifact   error
		wantErr    error
		wantNilErr bool
	}{
		{
			name:     "script wins over all",
			script:   errScript,
			cache:    errCache,
			artifact: errArtifact,
			wantErr:  errScript,
		},
		{
			name:     "cache wins when no script error",
			cache:    errCache,
			artifact: errArtifact,
			wantErr:  errCache,
		},
		{
			name:     "artifact wins when no script or cache error",
			artifact: errArtifact,
			wantErr:  errArtifact,
		},
		{
			name:       "all nil returns nil",
			wantNilErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickPriorityError(tt.script, tt.cache, tt.artifact)
			if tt.wantNilErr {
				assert.NoError(t, got)
			} else {
				assert.Equal(t, tt.wantErr, got)
			}
		})
	}
}

func TestClassifyScriptContextError(t *testing.T) {
	tests := []struct {
		name      string
		jobCtx    func() (context.Context, context.CancelFunc)
		scriptCtx func() (context.Context, context.CancelFunc)
		inputErr  error
		wantInner error
		wantNil   bool
		wantPass  bool // expect inputErr returned as-is
	}{
		{
			name:   "external cancel: job alive, script canceled",
			jobCtx: func() (context.Context, context.CancelFunc) { return context.WithCancel(t.Context()) },
			scriptCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx, cancel
			},
			wantInner: ErrJobCanceled,
		},
		{
			name:   "script deadline: job alive, script deadline exceeded",
			jobCtx: func() (context.Context, context.CancelFunc) { return context.WithCancel(t.Context()) },
			scriptCtx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
			},
			wantInner: ErrJobScriptTimeout,
		},
		{
			name:   "passthrough: both contexts alive",
			jobCtx: func() (context.Context, context.CancelFunc) { return context.WithCancel(t.Context()) },
			scriptCtx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(t.Context())
			},
			inputErr: errors.New("original"),
			wantPass: true,
		},
		{
			name:   "nil everything",
			jobCtx: func() (context.Context, context.CancelFunc) { return context.WithCancel(t.Context()) },
			scriptCtx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(t.Context())
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testRunner(t, nil)

			jobCtx, jobCancel := tt.jobCtx()
			defer jobCancel()
			scriptCtx, scriptCancel := tt.scriptCtx()
			defer scriptCancel()

			got := r.classifyScriptContextError(jobCtx, scriptCtx, tt.inputErr)

			switch {
			case tt.wantNil:
				assert.NoError(t, got)
			case tt.wantPass:
				assert.Equal(t, tt.inputErr, got)
			default:
				var exitErr *ExitError
				require.True(t, errors.As(got, &exitErr))
				assert.ErrorIs(t, exitErr.Inner, tt.wantInner)
			}
		})
	}
}

func TestWithTimeout(t *testing.T) {
	tests := []struct {
		name        string
		duration    time.Duration
		hasDeadline bool
	}{
		{
			name:        "zero duration gives cancellable context without deadline",
			duration:    0,
			hasDeadline: false,
		},
		{
			name:        "positive duration gives context with deadline",
			duration:    time.Hour,
			hasDeadline: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testRunner(t, nil)

			ctx, cancel := r.withTimeout(t.Context(), tt.duration)
			defer cancel()

			_, has := ctx.Deadline()
			assert.Equal(t, tt.hasDeadline, has)

			// Both cases should be cancellable.
			cancel()
			assert.Error(t, ctx.Err())
		})
	}
}

func TestGitlabEnv_Setup(t *testing.T) {
	r := testRunner(t, &Config{})
	require.NoError(t, r.setupGitlabEnv())

	assert.NotEmpty(t, r.env.GitLabEnvFile)
	assert.FileExists(t, r.env.GitLabEnvFile)
	assert.Equal(t, r.env.GitLabEnvFile, r.env.GitLabEnv["GITLAB_ENV"])
}

func TestGitlabEnv_Load(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantVars map[string]string
		wantGone []string
	}{
		{
			name:    "single variable",
			content: "MY_VAR=hello\n",
			wantVars: map[string]string{
				"MY_VAR": "hello",
			},
		},
		{
			name:    "overwrite on reload",
			content: "FOO=baz\n",
			wantVars: map[string]string{
				"FOO": "baz",
			},
		},
		{
			name:     "cleared when removed from file",
			content:  "",
			wantGone: []string{"FOO", "MY_VAR"},
		},
		{
			name:    "multiple variables",
			content: "A=1\nB=2\nC=3\n",
			wantVars: map[string]string{
				"A": "1",
				"B": "2",
				"C": "3",
			},
		},
		{
			name:    "value containing equals sign",
			content: "DSN=host=localhost port=5432\n",
			wantVars: map[string]string{
				"DSN": "host=localhost port=5432",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testRunner(t, &Config{})
			require.NoError(t, r.setupGitlabEnv())

			require.NoError(t, os.WriteFile(r.env.GitLabEnvFile, []byte(tt.content), 0o600))
			r.loadGitlabEnv()

			for k, v := range tt.wantVars {
				assert.Equal(t, v, r.env.GitLabEnv[k], "expected %s=%s", k, v)
			}
			for _, k := range tt.wantGone {
				_, exists := r.env.GitLabEnv[k]
				assert.False(t, exists, "%s should not exist", k)
			}

			// GITLAB_ENV path should always be preserved.
			assert.Equal(t, r.env.GitLabEnvFile, r.env.GitLabEnv["GITLAB_ENV"])
		})
	}
}

func TestGitlabEnv_LoadMissingFile(t *testing.T) {
	r := testRunner(t, &Config{})
	require.NoError(t, r.setupGitlabEnv())

	os.Remove(r.env.GitLabEnvFile)
	r.loadGitlabEnv()

	assert.Contains(t, runnerStderr(r), "Failed to read GITLAB_ENV file")
}

func TestGitlabEnv_LoadNoopWithoutSetup(t *testing.T) {
	r := testRunner(t, &Config{})

	// loadGitlabEnv with no file set should be a no-op, not panic.
	assert.NotPanics(t, func() { r.loadGitlabEnv() })
}

func TestGitlabEnv_Cleanup(t *testing.T) {
	r := testRunner(t, &Config{})
	require.NoError(t, r.setupGitlabEnv())

	envFile := r.env.GitLabEnvFile
	assert.FileExists(t, envFile)

	r.cleanup()
	assert.NoFileExists(t, envFile)
}

func TestAfterScript_ErrorHandling(t *testing.T) {
	tests := []struct {
		name            string
		ignoreErrors    bool
		existingErr     error
		wantErrNil      bool
		wantOriginalErr bool
		wantStderr      string
	}{
		{
			name:         "ignore errors: after_script failure is logged and suppressed",
			ignoreErrors: true,
			wantErrNil:   true,
			wantStderr:   "after_script failed, but job will continue unaffected",
		},
		{
			name:       "do not ignore: after_script error promoted when no script error",
			wantErrNil: false,
		},
		{
			name:            "do not ignore: original script error preserved",
			existingErr:     errors.New("original script error"),
			wantOriginalErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testRunner(t, &Config{
				AfterScriptIgnoreErrors: tt.ignoreErrors,
				Steps: []stages.Step{
					{Step: afterScriptStepName, Script: []string{"exit 1"}, OnSuccess: true, OnFailure: true},
				},
			})

			err := r.runAfterScriptSteps(t.Context(), r.config.Steps, tt.existingErr)

			switch {
			case tt.wantErrNil:
				assert.NoError(t, err)
			case tt.wantOriginalErr:
				assert.Equal(t, tt.existingErr, err)
			default:
				assert.Error(t, err)
			}

			if tt.wantStderr != "" {
				assert.Contains(t, runnerStderr(r), tt.wantStderr)
			}
		})
	}
}

func TestAfterScript_SetsScriptCancelNil(t *testing.T) {
	tests := []struct {
		name  string
		steps []stages.Step
	}{
		{
			name:  "with steps",
			steps: []stages.Step{{Step: afterScriptStepName, Script: []string{}, OnSuccess: true, OnFailure: true}},
		},
		{
			name:  "empty steps",
			steps: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testRunner(t, &Config{})

			_ = r.runAfterScriptSteps(t.Context(), tt.steps, nil)

			r.mu.Lock()
			assert.Nil(t, r.scriptCancel)
			r.mu.Unlock()
		})
	}
}

func TestCancel_NilScriptCancel_DoesNotPanic(t *testing.T) {
	r := testRunner(t, &Config{})

	r.mu.Lock()
	r.scriptCancel = nil
	r.mu.Unlock()

	assert.NotPanics(t, func() { r.Cancel() })
}

func TestSection_OutputFormat(t *testing.T) {
	r := testRunner(t, nil)

	err := r.section(t.Context(), "test_section", func(_ context.Context, _ *env.Env) error {
		return nil
	})
	require.NoError(t, err)

	out := runnerStdout(r)
	assert.Contains(t, out, "section_start:")
	assert.Contains(t, out, "test_section")
	assert.Contains(t, out, "section_end:")
}

// TestSectionNames_MatchAbstractShell verifies the runner emits section
// names matching the abstract shell's BuildStage values (see
// common/build.go's BuildStage constants and StepToBuildStage), so UI and
// log tooling that keys off section names continues to work after the
// script-to-step migration. Each section should appear exactly once,
// regardless of how many cache/artifact items the loop processes.
func TestSectionNames_MatchAbstractShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script execution; skip on windows")
	}

	r := testRunner(t, &Config{
		GetSources: stages.GetSources{GitStrategy: "none", MaxAttempts: 1},
		CacheExtract: []stages.CacheExtract{
			{Sources: []stages.CacheSource{{Key: "k1", Name: "k1"}}},
			{Sources: []stages.CacheSource{{Key: "k2", Name: "k2"}}},
		},
		ArtifactExtract: []stages.ArtifactDownload{
			{ArtifactName: "a1", Filename: "a1.zip", DownloadAttempts: 1},
		},
		Steps: []stages.Step{
			{Step: "script", Script: []string{"true"}, OnSuccess: true},
			{Step: afterScriptStepName, Script: []string{"true"}, OnSuccess: true, OnFailure: true},
		},
		CacheArchive: []stages.CacheArchive{
			{Key: "k1", Paths: []string{"x"}, OnSuccess: true},
		},
		ArtifactsArchive: []stages.ArtifactUpload{
			{ArtifactName: "a1", Paths: []string{"x"}, OnSuccess: true},
		},
	})
	r.env.BaseURL = "https://gitlab.example.com"

	_ = r.Run(t.Context())
	out := runnerStdout(r)

	// Each section_start/end line is "section_<start|end>:<unix>:<name>\r...";
	// expect exactly two occurrences of ":<name>\r" (start + end), regardless
	// of how many cache/artifact items the loop processed.
	wantOnce := []string{
		"get_sources",
		"restore_cache",
		"download_artifacts",
		"step_script",
		"after_script",
		"archive_cache",
		"upload_artifacts_on_success",
	}
	for _, name := range wantOnce {
		assert.Equal(t, 2, strings.Count(out, ":"+name+"\r"),
			"section %q should appear exactly once (start+end markers)", name)
	}

	mustNotAppear := []string{
		"restore_cache_0", "download_artifacts_0",
		"step_0_script", "after_script_0",
		"archive_cache_0", "upload_artifacts_0",
		"archive_cache_on_failure", "upload_artifacts_on_failure",
	}
	for _, name := range mustNotAppear {
		assert.NotContains(t, out, ":"+name+"\r",
			"legacy or wrong-state section %q should not be emitted", name)
	}
}

func TestFinalize_FailurePathSectionNames(t *testing.T) {
	r := testRunner(t, &Config{
		CacheArchive: []stages.CacheArchive{
			{Key: "k1", Paths: []string{"x"}, OnFailure: true},
		},
		ArtifactsArchive: []stages.ArtifactUpload{
			{ArtifactName: "a1", Paths: []string{"x"}, OnFailure: true},
		},
	})
	r.env.BaseURL = "http://test"
	r.env.SetStatus(env.Failed)

	_, _ = r.finalize(t.Context())
	out := runnerStdout(r)

	assert.Contains(t, out, ":archive_cache_on_failure\r")
	assert.Contains(t, out, ":upload_artifacts_on_failure\r")
	assert.NotContains(t, out, ":archive_cache\r",
		"must not emit success-path cache section name on failure path")
	assert.NotContains(t, out, ":upload_artifacts_on_success\r",
		"must not emit success-path upload section name on failure path")
}

// TestFinalize_EmptyBaseURLSkipsArtifactUpload mirrors abstract.go's
// writeUploadArtifacts ErrSkipBuildStage guard: when there is no server
// URL to upload to, the upload section must not be emitted at all rather
// than invoking artifacts-uploader with --url "". The cache-archive
// section is independent of BaseURL and should still emit.
func TestFinalize_EmptyBaseURLSkipsArtifactUpload(t *testing.T) {
	r := testRunner(t, &Config{
		CacheArchive: []stages.CacheArchive{
			{Key: "k1", Paths: []string{"x"}, OnSuccess: true},
		},
		ArtifactsArchive: []stages.ArtifactUpload{
			{ArtifactName: "a1", Paths: []string{"x"}, OnSuccess: true},
		},
	})
	// BaseURL deliberately left empty.

	_, _ = r.finalize(t.Context())
	out := runnerStdout(r)

	assert.NotContains(t, out, ":upload_artifacts_on_success\r",
		"upload section must be skipped when BaseURL is empty")
	assert.NotContains(t, out, ":upload_artifacts_on_failure\r")
	assert.Contains(t, out, ":archive_cache\r",
		"cache archive should still emit independent of BaseURL")
}

func TestSection_PropagatesError(t *testing.T) {
	r := testRunner(t, nil)
	want := errors.New("section failed")

	got := r.section(t.Context(), "failing", func(_ context.Context, _ *env.Env) error {
		return want
	})
	assert.Equal(t, want, got)
}

func TestExecuteSteps_SuccessFlow(t *testing.T) {
	r := testRunner(t, &Config{
		Steps: []stages.Step{
			{Step: "build", Script: []string{}, OnSuccess: true},
			{Step: afterScriptStepName, Script: []string{}, OnSuccess: true, OnFailure: true},
		},
	})

	err := r.executeSteps(t.Context())
	assert.NoError(t, err)
	assert.True(t, r.env.IsSuccessful())
}

func TestExecuteSteps_ScriptFailureSetsSuccessFalse(t *testing.T) {
	r := testRunner(t, &Config{
		Steps: []stages.Step{
			{Step: "build", Script: []string{"exit 1"}, OnSuccess: true},
			{Step: afterScriptStepName, Script: []string{}, OnSuccess: true, OnFailure: true},
		},
	})

	err := r.executeSteps(t.Context())
	assert.Error(t, err)
	assert.False(t, r.env.IsSuccessful())
}

func TestExecuteSteps_AfterScriptRunsOnFailure(t *testing.T) {
	r := testRunner(t, &Config{
		Steps: []stages.Step{
			{Step: "build", Script: []string{"exit 1"}, OnSuccess: true},
			{Step: afterScriptStepName, Script: []string{"echo after"}, OnSuccess: true, OnFailure: true},
		},
	})

	_ = r.executeSteps(t.Context())
	assert.Contains(t, runnerStdout(r), "after_script")
}

func TestScriptTimeout(t *testing.T) {
	r := testRunner(t, &Config{
		ScriptTimeout: 100 * time.Millisecond,
		Steps: []stages.Step{
			{Step: "script", Script: []string{"sleep 10"}, OnSuccess: true},
		},
	})

	err := r.executeSteps(t.Context())
	require.Error(t, err)

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		assert.ErrorIs(t, exitErr.Inner, ErrJobScriptTimeout)
	}
}

func TestAfterScriptTimeout_IndependentOfScriptTimeout(t *testing.T) {
	r := testRunner(t, &Config{
		ScriptTimeout:      50 * time.Millisecond,
		AfterScriptTimeout: 500 * time.Millisecond,
		Steps: []stages.Step{
			{Step: "script", Script: []string{"sleep 10"}, OnSuccess: true},
			{Step: afterScriptStepName, Script: []string{"echo after_ran"}, OnSuccess: true, OnFailure: true},
		},
	})

	err := r.executeSteps(t.Context())
	require.Error(t, err)

	// After-script should have run under its own timeout.
	assert.Contains(t, runnerStdout(r), "after_script")
}

func TestJobTimeout(t *testing.T) {
	r := testRunner(t, &Config{
		Timeout: 100 * time.Millisecond,
		GetSources: stages.GetSources{
			GitStrategy: "none",
			MaxAttempts: 1,
		},
		Steps: []stages.Step{
			{Step: "script", Script: []string{"sleep 10"}, OnSuccess: true},
		},
	})

	err := r.Run(t.Context())
	assert.Error(t, err)
}

func TestCancel_DuringScripts(t *testing.T) {
	r := testRunner(t, &Config{
		GetSources: stages.GetSources{GitStrategy: "none", MaxAttempts: 1},
		Steps: []stages.Step{
			{Step: "script", Script: []string{"sleep 60"}, OnSuccess: true},
			{Step: afterScriptStepName, Script: []string{"echo after"}, OnSuccess: true, OnFailure: true},
		},
	})

	done := make(chan error, 1)
	go func() { done <- r.Run(t.Context()) }()

	time.Sleep(100 * time.Millisecond)
	r.Cancel()

	err := <-done
	require.Error(t, err)

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		assert.ErrorIs(t, exitErr.Inner, ErrJobCanceled)
	}
}

func TestCIJobStatus(t *testing.T) {
	tests := []struct {
		name    string
		success bool
		want    string
	}{
		{"set on success", true, "success"},
		{"set on failure", false, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testRunner(t, &Config{})
			assert.Equal(t, "running", r.env.Env["CI_JOB_STATUS"])
			if tt.success {
				r.env.SetStatus(env.Success)
			} else {
				r.env.SetStatus(env.Failed)
			}
			assert.Equal(t, tt.want, r.env.Env["CI_JOB_STATUS"])
		})
	}
}

func TestPrepare_NonFatalFailures(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "cache restore failure",
			cfg: Config{
				GetSources: stages.GetSources{GitStrategy: "none", MaxAttempts: 1},
				CacheExtract: []stages.CacheExtract{
					{Sources: []stages.CacheSource{{Key: "bad"}}},
				},
			},
		},
		{
			name: "artifact download failure",
			cfg: Config{
				GetSources: stages.GetSources{GitStrategy: "none", MaxAttempts: 1},
				ArtifactExtract: []stages.ArtifactDownload{
					{ArtifactName: "bad", Filename: "bad.zip", DownloadAttempts: 1},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testRunner(t, &tt.cfg)
			err := r.prepare(t.Context())
			assert.NoError(t, err)
		})
	}
}

func TestRun_StrategyNone(t *testing.T) {
	tests := []struct {
		name    string
		script  []string
		wantErr bool
	}{
		{
			name:   "empty script succeeds",
			script: []string{},
		},
		{
			name:    "failing script surfaces error",
			script:  []string{"exit 1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testRunner(t, &Config{
				GetSources: stages.GetSources{GitStrategy: "none", MaxAttempts: 1},
				Steps: []stages.Step{
					{Step: "script", Script: tt.script, OnSuccess: true},
				},
			})

			err := r.Run(t.Context())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStatusFromError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want env.JobStatus
	}{
		{
			name: "nil maps to success",
			err:  nil,
			want: env.Success,
		},
		{
			name: "generic error maps to failed",
			err:  errors.New("something broke"),
			want: env.Failed,
		},
		{
			name: "ErrJobScriptTimeout maps to timedout",
			err:  ErrJobScriptTimeout,
			want: env.Timedout,
		},
		{
			name: "wrapped ErrJobScriptTimeout maps to timedout",
			err:  fmt.Errorf("wrapped: %w", ErrJobScriptTimeout),
			want: env.Timedout,
		},
		{
			name: "context.DeadlineExceeded maps to timedout",
			err:  context.DeadlineExceeded,
			want: env.Timedout,
		},
		{
			name: "wrapped context.DeadlineExceeded maps to timedout",
			err:  fmt.Errorf("wrapped: %w", context.DeadlineExceeded),
			want: env.Timedout,
		},
		{
			name: "ErrJobCanceled maps to canceled",
			err:  ErrJobCanceled,
			want: env.Canceled,
		},
		{
			name: "wrapped ErrJobCanceled maps to canceled",
			err:  fmt.Errorf("wrapped: %w", ErrJobCanceled),
			want: env.Canceled,
		},
		{
			name: "context.Canceled maps to canceled",
			err:  context.Canceled,
			want: env.Canceled,
		},
		{
			name: "wrapped context.Canceled maps to canceled",
			err:  fmt.Errorf("wrapped: %w", context.Canceled),
			want: env.Canceled,
		},
		{
			name: "ExitError wrapping ErrJobScriptTimeout maps to timedout",
			err:  &ExitError{Inner: fmt.Errorf("%w: %w", ErrJobScriptTimeout, context.DeadlineExceeded), ExitCode: 1},
			want: env.Timedout,
		},
		{
			name: "ExitError wrapping ErrJobCanceled maps to canceled",
			err:  &ExitError{Inner: ErrJobCanceled, ExitCode: 1},
			want: env.Canceled,
		},
		{
			name: "ExitError wrapping generic error maps to failed",
			err:  &ExitError{Inner: errors.New("exit 1"), ExitCode: 1},
			want: env.Failed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusFromError(tt.err))
		})
	}
}

func TestExecuteSteps_CIJobStatus(t *testing.T) {
	tests := []struct {
		name       string
		cfg        Config
		wantStatus string
	}{
		{
			name: "success sets CI_JOB_STATUS to success",
			cfg: Config{
				Steps: []stages.Step{
					{Step: "build", Script: []string{}, OnSuccess: true},
					{Step: afterScriptStepName, Script: []string{}, OnSuccess: true, OnFailure: true},
				},
			},
			wantStatus: "success",
		},
		{
			name: "script failure sets CI_JOB_STATUS to failed",
			cfg: Config{
				Steps: []stages.Step{
					{Step: "build", Script: []string{"exit 1"}, OnSuccess: true},
					{Step: afterScriptStepName, Script: []string{}, OnSuccess: true, OnFailure: true},
				},
			},
			wantStatus: "failed",
		},
		{
			name: "script timeout sets CI_JOB_STATUS to timedout",
			cfg: Config{
				ScriptTimeout: 100 * time.Millisecond,
				Steps: []stages.Step{
					{Step: "script", Script: []string{"sleep 10"}, OnSuccess: true},
					{Step: afterScriptStepName, Script: []string{}, OnSuccess: true, OnFailure: true},
				},
			},
			wantStatus: "timedout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testRunner(t, &tt.cfg)

			_ = r.executeSteps(t.Context())

			assert.Equal(t, tt.wantStatus, r.env.Env["CI_JOB_STATUS"])
		})
	}
}

// TestStep_LinesShareShellState verifies the contract the builder relies on
// when it folds pre_build_script and post_build_script into each user step:
// every line of a single stages.Step.Script runs inside the same shell
// process, so shell-only state (exports, cd, set options, function
// definitions) defined earlier in the script is visible later. This matches
// the abstract shell's writeUserScript behaviour, where pre_build_script,
// the user script and post_build_script all run as one shell invocation.
func TestStep_LinesShareShellState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash export semantics; skip on windows")
	}

	r := testRunner(t, &Config{
		Steps: []stages.Step{
			{
				Step: "script",
				Script: []string{
					"export PRE_BUILD_VAR=hello",     // pre_build_script line
					`echo "got:[${PRE_BUILD_VAR}]"`,  // user script line
					`echo "post:[${PRE_BUILD_VAR}]"`, // post_build_script line
				},
				OnSuccess: true,
			},
		},
	})

	err := r.executeSteps(t.Context())
	require.NoError(t, err)

	out := runnerStdout(r)
	assert.Contains(t, out, "got:[hello]",
		"pre_build_script exports must be visible to the user script lines that follow")
	assert.Contains(t, out, "post:[hello]",
		"pre_build_script exports must be visible to post_build_script lines that follow")
}

func TestExecuteSteps_CancelSetsCIJobStatusCanceled(t *testing.T) {
	r := testRunner(t, &Config{
		Steps: []stages.Step{
			{Step: "script", Script: []string{"sleep 60"}, OnSuccess: true},
			{Step: afterScriptStepName, Script: []string{}, OnSuccess: true, OnFailure: true},
		},
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = r.executeSteps(t.Context())
	}()

	// Give the step time to start, then cancel.
	time.Sleep(100 * time.Millisecond)
	r.Cancel()

	<-done

	assert.Equal(t, "canceled", r.env.Env["CI_JOB_STATUS"])
}

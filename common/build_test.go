//go:build !integration

package common

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/session"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

func init() {
	s := MockShell{}
	s.On("GetName").Return("script-shell")
	s.On("IsDefault").Return(false)
	s.On("GenerateScript", mock.Anything, mock.Anything, mock.Anything).Return("script", nil)
	RegisterShell(&s)
}

func TestBuildPredefinedVariables(t *testing.T) {
	for _, rootDir := range []string{"/root/dir1", "/root/dir2"} {
		t.Run(rootDir, func(t *testing.T) {
			build := runSuccessfulMockBuild(t, func(options ExecutorPrepareOptions) error {
				return options.Build.StartBuild(rootDir, "/cache/dir", false, false, false)
			})

			projectDir := build.GetAllVariables().Value("CI_PROJECT_DIR")
			assert.NotEmpty(t, projectDir, "should have CI_PROJECT_DIR")
		})
	}
}

func TestBuildTimeoutExposed(t *testing.T) {
	const testTimeout = 180
	tests := map[string]struct {
		forceDefault    bool
		customTimeout   int
		expectedTimeout int
	}{
		"no timeout specified": {
			forceDefault:    true,
			expectedTimeout: DefaultTimeout,
		},
		"timeout with arbitrary value": {
			customTimeout:   testTimeout,
			expectedTimeout: testTimeout,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			build := runSuccessfulMockBuild(t, func(options ExecutorPrepareOptions) error {
				if !tt.forceDefault {
					options.Build.RunnerInfo.Timeout = tt.customTimeout
				}
				return options.Build.StartBuild("/root/dir", "/cache/dir", false, false, false)
			})

			exposedTimeout, err := strconv.Atoi(build.GetAllVariables().Value("CI_JOB_TIMEOUT"))
			require.NoError(t, err)
			assert.Equal(t, exposedTimeout, tt.expectedTimeout)
		})
	}
}

func matchBuildStage(buildStage BuildStage) interface{} {
	return mock.MatchedBy(func(cmd ExecutorCommand) bool {
		return cmd.Stage == buildStage
	})
}

func TestBuildRun(t *testing.T) {
	runSuccessfulMockBuild(t, func(options ExecutorPrepareOptions) error { return nil })
}

func TestBuildPanic(t *testing.T) {
	panicFn := func(mock.Arguments) {
		panic("panic message")
	}

	tests := map[string]struct {
		setupMockExecutor func(*MockExecutor)
	}{
		"prepare": {
			setupMockExecutor: func(executor *MockExecutor) {
				executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
					Run(panicFn).Once()
			},
		},
		"run": {
			setupMockExecutor: func(executor *MockExecutor) {
				executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()
				executor.On("Finish", mock.Anything).Once()
				executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
				executor.On("Run", mock.Anything).Run(panicFn).Once()
				executor.On("Cleanup").Once()
			},
		},
		"cleanup": {
			setupMockExecutor: func(executor *MockExecutor) {
				executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()
				executor.On("Finish", mock.Anything).Once()
				executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
				executor.On("Run", mock.Anything).Once()
				executor.On("Cleanup").Run(panicFn).Once()
			},
		},
		"shell": {
			setupMockExecutor: func(executor *MockExecutor) {
				executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()
				executor.On("Finish", mock.Anything).Once()
				executor.On("Shell").Run(panicFn)
				executor.On("Cleanup").Once()
			},
		},
		"run+cleanup": {
			setupMockExecutor: func(executor *MockExecutor) {
				executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()
				executor.On("Finish", mock.Anything).Once()
				executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
				executor.On("Run", mock.Anything).Run(panicFn).Once()
				executor.On("Cleanup").Run(panicFn).Once()
			},
		},
		"finish": {
			setupMockExecutor: func(executor *MockExecutor) {
				executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()
				executor.On("Finish", mock.Anything).Run(panicFn).Once()
				executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
				executor.On("Run", mock.Anything).Once()
				executor.On("Cleanup").Once()
			},
		},
		"finish+cleanup+shell": {
			setupMockExecutor: func(executor *MockExecutor) {
				executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()
				executor.On("Finish", mock.Anything).Run(panicFn).Once()
				executor.On("Shell").Run(panicFn).Return(&ShellScriptInfo{Shell: "script-shell"})
				executor.On("Cleanup").Run(panicFn).Once()
			},
		},
		"run+finish+cleanup": {
			setupMockExecutor: func(executor *MockExecutor) {
				executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()
				executor.On("Finish", mock.Anything).Run(panicFn).Once()
				executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
				executor.On("Run", mock.Anything).Run(panicFn).Once()
				executor.On("Cleanup").Run(panicFn).Once()
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			executor, provider := setupMockExecutorAndProvider(t)

			tt.setupMockExecutor(executor)

			RegisterExecutorProviderForTest(t, t.Name(), provider)

			res, err := GetSuccessfulBuild()
			require.NoError(t, err)

			cfg := &RunnerConfig{}
			cfg.Executor = t.Name()
			build, err := NewBuild(res, cfg, nil, nil)
			require.NoError(t, err)
			var out bytes.Buffer
			err = build.Run(&Config{}, &Trace{Writer: &out})
			assert.EqualError(t, err, "panic: panic message")
			assert.Contains(t, out.String(), "panic: panic message")
		})
	}
}

func TestJobImageExposed(t *testing.T) {
	tests := map[string]struct {
		image           string
		vars            []JobVariable
		expectVarExists bool
		expectImageName string
	}{
		"normal image exposed": {
			image:           "alpine:3.14",
			expectVarExists: true,
			expectImageName: "alpine:3.14",
		},
		"image with variable expansion": {
			image:           "${IMAGE}:3.14",
			vars:            []JobVariable{{Key: "IMAGE", Value: "alpine", Public: true}},
			expectVarExists: true,
			expectImageName: "alpine:3.14",
		},
		"no image specified": {
			image:           "",
			expectVarExists: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			build := runSuccessfulMockBuild(t, func(options ExecutorPrepareOptions) error {
				options.Build.Image.Name = tt.image
				options.Build.Variables = append(options.Build.Variables, tt.vars...)
				return options.Build.StartBuild("/root/dir", "/cache/dir", false, false, false)
			})

			actualVarExists := false
			for _, v := range build.GetAllVariables() {
				if v.Key == "CI_JOB_IMAGE" {
					actualVarExists = true
					break
				}
			}
			assert.Equal(t, tt.expectVarExists, actualVarExists, "CI_JOB_IMAGE exported?")

			if tt.expectVarExists {
				actualJobImage := build.GetAllVariables().Value("CI_JOB_IMAGE")
				assert.Equal(t, tt.expectImageName, actualJobImage)
			}
		})
	}
}

func TestBuildRunNoModifyConfig(t *testing.T) {
	expectHostAddr := "10.0.0.1"
	p := setupSuccessfulMockExecutor(t, func(options ExecutorPrepareOptions) error {
		options.Config.Docker.Credentials.Host = "10.0.0.2"
		return nil
	})

	rc := &RunnerConfig{
		RunnerSettings: RunnerSettings{
			Docker: &DockerConfig{
				Credentials: docker.Credentials{
					Host: expectHostAddr,
				},
			},
		},
	}
	build := registerExecutorWithSuccessfulBuild(t, p, rc)

	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)
	assert.Equal(t, expectHostAddr, rc.Docker.Credentials.Host)
}

func TestRetryPrepare(t *testing.T) {
	PreparationRetryInterval = 0

	e := NewMockExecutor(t)
	p := NewMockExecutorProvider(t)

	// Create executor
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(e).Times(3)

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("prepare failed")).Twice()
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	e.On("Cleanup").Times(3)

	// Succeed a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", mock.Anything).Return(nil)
	e.On("Finish", nil).Once()

	build := registerExecutorWithSuccessfulBuild(t, p, new(RunnerConfig))
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestPrepareFailure(t *testing.T) {
	PreparationRetryInterval = 0

	e := NewMockExecutor(t)
	p := NewMockExecutorProvider(t)

	// Create executor
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(e).Times(3)

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("prepare failed")).Times(3)
	e.On("Cleanup").Times(3)

	build := registerExecutorWithSuccessfulBuild(t, p, new(RunnerConfig))
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "prepare failed")
}

func TestPrepareFailureOnBuildError(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider(t)
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(&BuildError{}).Once()
	executor.On("Cleanup").Once()

	build := registerExecutorWithSuccessfulBuild(t, provider, new(RunnerConfig))
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})

	expectedErr := new(BuildError)
	assert.ErrorIs(t, err, expectedErr)
}

func TestPrepareEnvironmentFailure(t *testing.T) {
	testErr := errors.New("test-err")

	e := NewMockExecutor(t)
	p := NewMockExecutorProvider(t)

	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()
	p.On("Create").Return(e).Once()

	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	e.On("Cleanup").Once()
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(testErr).Once()
	e.On("Finish", mock.Anything).Once()

	RegisterExecutorProviderForTest(t, "build-run-prepare-environment-failure-on-build-error", p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-prepare-environment-failure-on-build-error",
			},
		},
	}

	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.ErrorIs(t, err, testErr)
}

func TestJobFailure(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider(t)
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	executor.On("Cleanup").Once()

	// Set up a failing a build script
	thrownErr := &BuildError{Inner: errors.New("test error"), ExitCode: 1}
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	executor.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	executor.On("Run", mock.Anything).Return(thrownErr).Times(3)
	executor.On("Run", matchBuildStage(BuildStageCleanup)).Return(nil).Once()
	executor.On("Finish", thrownErr).Once()

	RegisterExecutorProviderForTest(t, "build-run-job-failure", provider)

	failedBuild, err := GetFailedBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: failedBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-job-failure",
			},
		},
	}

	trace := NewMockJobTrace(t)
	trace.On("Write", mock.Anything).Return(0, nil)
	trace.On("IsStdout").Return(true)
	trace.On("SetCancelFunc", mock.Anything).Once()
	trace.On("SetAbortFunc", mock.Anything).Once()
	trace.On("SetSupportedFailureReasonMapper", mock.Anything).Once()
	trace.On("Fail", thrownErr, JobFailureData{Reason: "", ExitCode: 1}).Return(nil).Once()

	err = build.Run(&Config{}, trace)

	expectedErr := new(BuildError)
	assert.ErrorIs(t, err, expectedErr)
}

func TestJobFailureOnExecutionTimeout(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider(t)
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	executor.On("Cleanup").Once()

	// Succeed a build script
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	executor.On("Run", matchBuildStage("step_script")).Run(func(mock.Arguments) {
		time.Sleep(2 * time.Second)
	}).Return(nil)
	executor.On("Run", mock.Anything).Return(nil)
	executor.On("Finish", mock.Anything).Once()

	build := registerExecutorWithSuccessfulBuild(t, provider, new(RunnerConfig))
	build.JobResponse.RunnerInfo.Timeout = 1

	trace := NewMockJobTrace(t)
	trace.On("Write", mock.Anything).Return(0, nil)
	trace.On("IsStdout").Return(true)
	trace.On("SetCancelFunc", mock.Anything).Twice()
	trace.On("SetAbortFunc", mock.Anything).Once()
	trace.On("SetSupportedFailureReasonMapper", mock.Anything).Once()
	trace.On("Fail", mock.Anything, JobFailureData{Reason: JobExecutionTimeout}).Run(func(arguments mock.Arguments) {
		assert.Error(t, arguments.Get(0).(error))
	}).Return(nil).Once()

	err := build.Run(&Config{}, trace)

	expectedErr := &BuildError{FailureReason: JobExecutionTimeout}
	assert.ErrorIs(t, err, expectedErr)
}

func TestRunFailureRunsAfterScriptAndArtifactsOnFailure(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider(t)
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	executor.On("Cleanup").Once()

	// Fail a build script
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	executor.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(nil).Once()
	executor.On("Run", matchBuildStage("step_script")).Return(errors.New("build fail")).Once()
	executor.On("Run", matchBuildStage(BuildStageAfterScript)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageUploadOnFailureArtifacts)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageArchiveOnFailureCache)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageCleanup)).Return(nil).Once()
	executor.On("Finish", errors.New("build fail")).Once()

	RegisterExecutorProviderForTest(t, "build-run-run-failure", provider)

	failedBuild, err := GetFailedBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: failedBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-run-failure",
			},
		},
	}
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "build fail")
}

func TestGetSourcesRunFailure(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider(t)
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	executor.On("Cleanup").Once()

	// Fail a build script
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	executor.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	for attempt := 0; attempt < 10; attempt++ {
		if attempt == 0 {
			executor.On("Run", matchBuildStage(BuildStageClearWorktree)).Return(nil)
		}
		executor.On("Run", matchBuildStage(BuildStageGetSources)).Return(errors.New("build fail"))
	}
	executor.On("Run", matchBuildStage(BuildStageArchiveOnFailureCache)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageUploadOnFailureArtifacts)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageCleanup)).Return(nil).Once()
	executor.On("Finish", errors.New("build fail")).Once()

	build := registerExecutorWithSuccessfulBuild(t, provider, new(RunnerConfig))
	build.Variables = append(build.Variables, JobVariable{Key: "GET_SOURCES_ATTEMPTS", Value: "3"})
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "build fail")
}

func TestArtifactDownloadRunFailure(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider(t)
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	executor.On("Cleanup").Once()

	// Fail a build script
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	executor.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(errors.New("build fail")).Times(3)
	executor.On("Run", matchBuildStage(BuildStageArchiveOnFailureCache)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageUploadOnFailureArtifacts)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageCleanup)).Return(nil).Once()
	executor.On("Finish", errors.New("build fail")).Once()

	build := registerExecutorWithSuccessfulBuild(t, provider, new(RunnerConfig))
	build.Variables = append(build.Variables, JobVariable{Key: "ARTIFACT_DOWNLOAD_ATTEMPTS", Value: "3"})
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "build fail")
}

func TestArtifactUploadRunFailure(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider(t)
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	executor.On("Cleanup").Once()

	// Successful build script
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"}).Times(9)
	executor.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(nil).Once()
	executor.On("Run", matchBuildStage("step_script")).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageAfterScript)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageArchiveOnSuccessCache)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageUploadOnSuccessArtifacts)).Return(errors.New("upload fail")).Once()
	executor.On("Run", matchBuildStage(BuildStageCleanup)).Return(nil).Once()
	executor.On("Finish", errors.New("upload fail")).Once()

	build := registerExecutorWithSuccessfulBuild(t, provider, new(RunnerConfig))
	successfulBuild := build.JobResponse
	successfulBuild.Artifacts = make(Artifacts, 1)
	successfulBuild.Artifacts[0] = Artifact{
		Name:      "my-artifact",
		Untracked: false,
		Paths:     ArtifactPaths{"cached/*"},
		When:      ArtifactWhenAlways,
	}
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "upload fail")
}

func TestArchiveCacheOnScriptFailure(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider(t)
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	executor.On("Cleanup").Once()

	// Fail a build script
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"}).Times(9)
	executor.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(nil).Once()
	executor.On("Run", matchBuildStage("step_script")).Return(errors.New("script failure")).Once()
	executor.On("Run", matchBuildStage(BuildStageAfterScript)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageArchiveOnFailureCache)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageUploadOnFailureArtifacts)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageCleanup)).Return(nil).Once()
	executor.On("Finish", errors.New("script failure")).Once()

	build := registerExecutorWithSuccessfulBuild(t, provider, new(RunnerConfig))
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "script failure")
}

func TestUploadArtifactsOnArchiveCacheFailure(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider(t)
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	executor.On("Cleanup").Once()

	// Successful build script
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"}).Times(9)
	executor.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(nil).Once()
	executor.On("Run", matchBuildStage("step_script")).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageAfterScript)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageArchiveOnSuccessCache)).Return(errors.New("cache failure")).Once()
	executor.On("Run", matchBuildStage(BuildStageUploadOnSuccessArtifacts)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageCleanup)).Return(nil).Once()
	executor.On("Finish", errors.New("cache failure")).Once()

	build := registerExecutorWithSuccessfulBuild(t, provider, new(RunnerConfig))
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "cache failure")
}

func TestRestoreCacheRunFailure(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider(t)
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	executor.On("Cleanup").Once()

	// Fail a build script
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	executor.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(errors.New("build fail")).Times(3)
	executor.On("Run", matchBuildStage(BuildStageArchiveOnFailureCache)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageUploadOnFailureArtifacts)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageCleanup)).Return(nil).Once()
	executor.On("Finish", errors.New("build fail")).Once()

	build := registerExecutorWithSuccessfulBuild(t, provider, new(RunnerConfig))
	build.Variables = append(build.Variables, JobVariable{Key: "RESTORE_CACHE_ATTEMPTS", Value: "3"})
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "build fail")
}

func TestRunWrongAttempts(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider(t)
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	executor.On("Cleanup").Once()

	// Fail a build script
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	executor.On("Run", mock.Anything).Return(nil).Once()
	executor.
		On("Run", mock.Anything).
		Return(errors.New("number of attempts out of the range [1, 10] for stage: get_sources"))
	executor.On(
		"Finish",
		errors.New("number of attempts out of the range [1, 10] for stage: get_sources"),
	)

	build := registerExecutorWithSuccessfulBuild(t, provider, new(RunnerConfig))
	build.Variables = append(build.Variables, JobVariable{Key: "GET_SOURCES_ATTEMPTS", Value: "0"})
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "number of attempts out of the range [1, 10] for stage: get_sources")
}

func TestRunSuccessOnSecondAttempt(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider(t)

	// We run everything once
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	executor.On("Finish", mock.Anything).Once()
	executor.On("Cleanup").Once()

	// Run script successfully
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})

	var getSourcesRunAttempts int
	executor.On("Run", mock.Anything).Return(func(cmd ExecutorCommand) error {
		if cmd.Stage == BuildStageGetSources {
			getSourcesRunAttempts++
			if getSourcesRunAttempts == 1 {
				return errors.New("build fail")
			}
		}
		return nil
	})

	build := registerExecutorWithSuccessfulBuild(t, provider, new(RunnerConfig))
	build.Variables = append(build.Variables, JobVariable{Key: "GET_SOURCES_ATTEMPTS", Value: "3"})
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)
	assert.Equal(t, 2, getSourcesRunAttempts)
}

func TestDebugTrace(t *testing.T) {
	testCases := map[string]struct {
		debugTraceVariableValue   string
		expectedValue             bool
		debugTraceFeatureDisabled bool
		expectedLogOutput         string
	}{
		"variable not set": {
			expectedValue: false,
		},
		"variable set to false": {
			debugTraceVariableValue: "false",
			expectedValue:           false,
		},
		"variable set to true": {
			debugTraceVariableValue: "true",
			expectedValue:           true,
		},
		"variable set to a non-bool value": {
			debugTraceVariableValue: "xyz",
			expectedValue:           false,
		},
		"variable set to true and feature disabled from configuration": {
			debugTraceVariableValue:   "true",
			expectedValue:             false,
			debugTraceFeatureDisabled: true,
			expectedLogOutput:         "CI_DEBUG_TRACE: usage is disabled on this Runner",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			build := &Build{
				JobResponse: JobResponse{
					Variables: JobVariables{},
				},
				Runner: &RunnerConfig{
					RunnerSettings: RunnerSettings{
						DebugTraceDisabled: testCase.debugTraceFeatureDisabled,
					},
				},
			}

			if testCase.debugTraceVariableValue != "" {
				build.Variables = append(
					build.Variables,
					JobVariable{Key: "CI_DEBUG_TRACE", Value: testCase.debugTraceVariableValue, Public: true},
				)
			}

			isTraceEnabled := build.IsDebugTraceEnabled()
			assert.Equal(t, testCase.expectedValue, isTraceEnabled)

			if testCase.expectedLogOutput != "" {
				output := errors.Join(build.Settings().Errors...).Error()
				assert.Contains(t, output, testCase.expectedLogOutput)
			}
		})
	}
}

func TestDefaultEnvVariables(t *testing.T) {
	tests := map[string]struct {
		buildDir      string
		expectedValue string
	}{
		"UNIX-style BuildDir": {
			buildDir:      "/tmp/test-build/dir",
			expectedValue: "CI_PROJECT_DIR=/tmp/test-build/dir",
		},
		// The next four tests' expected value will depend on the platform running the tests
		"Windows UNC-style BuildDir (extended-length path support)": {
			buildDir:      `\\?\C:\tmp\test-build\dir`,
			expectedValue: "CI_PROJECT_DIR=" + filepath.FromSlash("//?/C:/tmp/test-build/dir"),
		},
		"Windows UNC-style BuildDir": {
			buildDir:      `\\host\share\tmp\test-build\dir`,
			expectedValue: "CI_PROJECT_DIR=" + filepath.FromSlash("//host/share/tmp/test-build/dir"),
		},
		"Windows-style BuildDir (PS)": {
			buildDir:      `C:\tmp\test-build\dir`,
			expectedValue: "CI_PROJECT_DIR=" + filepath.FromSlash("C:/tmp/test-build/dir"),
		},
		"Windows-style BuildDir with forward slashes and drive letter": {
			buildDir:      "C:/tmp/test-build/dir",
			expectedValue: "CI_PROJECT_DIR=" + filepath.FromSlash("C:/tmp/test-build/dir"),
		},
		"Windows-style BuildDir in MSYS bash executor and drive letter)": {
			buildDir:      "/c/tmp/test-build/dir",
			expectedValue: "CI_PROJECT_DIR=/c/tmp/test-build/dir",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			build := new(Build)
			build.BuildDir = test.buildDir

			vars := build.GetAllVariables().StringList()

			assert.Contains(t, vars, test.expectedValue)
			assert.Contains(t, vars, "CI_SERVER=yes")
		})
	}
}

func TestSharedEnvVariables(t *testing.T) {
	for _, shared := range [...]bool{true, false} {
		t.Run(fmt.Sprintf("Value:%v", shared), func(t *testing.T) {
			assert := assert.New(t)
			build := Build{
				ExecutorFeatures: FeaturesInfo{Shared: shared},
			}
			vars := build.GetAllVariables().StringList()

			assert.NotNil(vars)

			present := "CI_SHARED_ENVIRONMENT=true"
			absent := "CI_DISPOSABLE_ENVIRONMENT=true"
			if !shared {
				present, absent = absent, present
			}

			assert.Contains(vars, present)
			assert.NotContains(vars, absent)
			// we never expose false
			assert.NotContains(vars, "CI_SHARED_ENVIRONMENT=false")
			assert.NotContains(vars, "CI_DISPOSABLE_ENVIRONMENT=false")
		})
	}
}

func TestGetRemoteURL(t *testing.T) {
	const (
		exampleJobToken    = "job-token"
		exampleRepoURL     = "http://gitlab-ci-token:job-token@test.remote/my/project.git"
		exampleProjectPath = "my/project"

		jobTokenFromEnv = true
		jobTokenInURL   = false
	)

	testCases := map[string]struct {
		runner                    RunnerSettings
		jobTokenVariableOverwrite string
		expectedURL               map[bool]string
	}{
		"using clone_url with http protocol": {
			runner: RunnerSettings{
				CloneURL: "http://test.local/",
			},
			expectedURL: map[bool]string{
				jobTokenFromEnv: "http://test.local/my/project.git",
				jobTokenInURL:   "http://gitlab-ci-token:job-token@test.local/my/project.git",
			},
		},
		"using clone_url with https protocol": {
			runner: RunnerSettings{
				CloneURL: "https://test.local",
			},
			expectedURL: map[bool]string{
				jobTokenFromEnv: "https://test.local/my/project.git",
				jobTokenInURL:   "https://gitlab-ci-token:job-token@test.local/my/project.git",
			},
		},
		"using clone_url with relative URL": {
			runner: RunnerSettings{
				CloneURL: "https://test.local/gitlab",
			},
			expectedURL: map[bool]string{
				jobTokenFromEnv: "https://test.local/gitlab/my/project.git",
				jobTokenInURL:   "https://gitlab-ci-token:job-token@test.local/gitlab/my/project.git",
			},
		},
		"using clone_url with relative URL with trailing slash": {
			runner: RunnerSettings{
				CloneURL: "https://test.local/gitlab/",
			},
			expectedURL: map[bool]string{
				jobTokenFromEnv: "https://test.local/gitlab/my/project.git",
				jobTokenInURL:   "https://gitlab-ci-token:job-token@test.local/gitlab/my/project.git",
			},
		},
		"using clone_url with ssh protocol": {
			runner: RunnerSettings{
				CloneURL: "ssh://git@test.local/",
			},
			expectedURL: map[bool]string{
				jobTokenFromEnv: "ssh://git@test.local/my/project.git",
				jobTokenInURL:   "ssh://git@test.local/my/project.git",
			},
		},
		"using clone_url with ssh protocol and default username": {
			runner: RunnerSettings{
				CloneURL: "ssh://test.local/",
			},
			expectedURL: map[bool]string{
				jobTokenFromEnv: "ssh://git@test.local/my/project.git",
				jobTokenInURL:   "ssh://git@test.local/my/project.git",
			},
		},
		"not using clone_url": {
			runner: RunnerSettings{},
			expectedURL: map[bool]string{
				jobTokenFromEnv: "http://test.remote/my/project.git",
				jobTokenInURL:   exampleRepoURL,
			},
		},
		"overwriting job token with variable and clone_url": {
			runner: RunnerSettings{
				CloneURL: "https://test.local",
			},
			jobTokenVariableOverwrite: "wrong-token",
			expectedURL: map[bool]string{
				jobTokenFromEnv: "https://test.local/my/project.git",
				jobTokenInURL:   "https://gitlab-ci-token:job-token@test.local/my/project.git",
			},
		},
		"overwriting job token with variable and no clone_url": {
			runner:                    RunnerSettings{},
			jobTokenVariableOverwrite: "wrong-token",
			expectedURL: map[bool]string{
				jobTokenFromEnv: "http://test.remote/my/project.git",
				jobTokenInURL:   exampleRepoURL,
			},
		},
	}

	for _, ffState := range []bool{jobTokenFromEnv, jobTokenInURL} {
		ff := featureflags.GitURLsWithoutTokens
		name := fmt.Sprintf("%s:%t", ff, ffState)
		t.Run(name, func(t *testing.T) {
			for tn, tc := range testCases {
				t.Run(tn, func(t *testing.T) {
					build := &Build{
						Runner: &RunnerConfig{
							RunnerSettings: tc.runner,
						},
						JobResponse: JobResponse{
							Token: exampleJobToken,
							GitInfo: GitInfo{
								RepoURL: exampleRepoURL,
							},
						},
					}

					build.Runner.FeatureFlags = map[string]bool{
						ff: ffState,
					}

					variables := JobVariables{
						{Key: "CI_PROJECT_PATH", Value: exampleProjectPath},
					}

					if tc.jobTokenVariableOverwrite != "" {
						variables = append(variables, JobVariable{
							Key:   "CI_JOB_TOKEN",
							Value: tc.jobTokenVariableOverwrite,
						})
					}

					build.JobResponse.Variables = variables

					remoteURL, err := build.GetRemoteURL()
					assert.NoError(t, err, "getting remote URL")
					assert.Equal(t, tc.expectedURL[ffState], remoteURL)
				})
			}
		})
	}
}

func TestGetURLInsteadOfArgs(t *testing.T) {
	const (
		exampleJobToken   = "job-token"
		exampleServerHost = "test.local"
		exampleServerURL  = "https://test.local"

		jobTokenFromEnv = true
		jobTokenInURL   = false
	)

	testCases := map[string]struct {
		cloneURL     string
		serverURL    string
		serverPort   string
		expectedArgs map[bool][]string
		forceHTTPS   bool
	}{
		"with default url": {
			expectedArgs: map[bool][]string{
				jobTokenFromEnv: {},
				jobTokenInURL: {
					"-c", "url.https://gitlab-ci-token:job-token@test.local.insteadOf=https://test.local",
				},
			},
		},
		"with clone_url": {
			cloneURL: "https://custom.local",
			expectedArgs: map[bool][]string{
				jobTokenFromEnv: {},
				jobTokenInURL: {
					"-c", "url.https://gitlab-ci-token:job-token@custom.local.insteadOf=https://custom.local",
				},
			},
		},
		"with http protocol": {
			serverURL: "http://test.local",
			expectedArgs: map[bool][]string{
				jobTokenFromEnv: {},
				jobTokenInURL: {
					"-c", "url.http://gitlab-ci-token:job-token@test.local.insteadOf=http://test.local",
				},
			},
		},
		"with clone_url and http protocol": {
			cloneURL: "http://test.local",
			expectedArgs: map[bool][]string{
				jobTokenFromEnv: {},
				jobTokenInURL: {
					"-c", "url.http://gitlab-ci-token:job-token@test.local.insteadOf=http://test.local",
				},
			},
		},
		"with directory URL": {
			serverURL: "https://test.local/gitlab",
			expectedArgs: map[bool][]string{
				jobTokenFromEnv: {},
				jobTokenInURL: {
					"-c", "url.https://gitlab-ci-token:job-token@test.local/gitlab.insteadOf=https://test.local/gitlab",
				},
			},
		},
		"with directory URL with trailing slash stripped": {
			cloneURL: "https://test.local/gitlab/",
			expectedArgs: map[bool][]string{
				jobTokenFromEnv: {},
				jobTokenInURL: {
					"-c", "url.https://gitlab-ci-token:job-token@test.local/gitlab.insteadOf=https://test.local/gitlab",
				},
			},
		},
		"with clone_url and ssh protocol ignored": {
			cloneURL: "ssh://git@test.local",
			expectedArgs: map[bool][]string{
				jobTokenFromEnv: {},
				jobTokenInURL:   {},
			},
		},
		"with default url and force HTTPS": {
			forceHTTPS: true,
			expectedArgs: map[bool][]string{
				jobTokenFromEnv: {
					"-c",
					"url.https://test.local/.insteadOf=git@test.local:",
					"-c",
					"url.https://test.local.insteadOf=ssh://git@test.local",
				},
				jobTokenInURL: {
					"-c",
					"url.https://gitlab-ci-token:job-token@test.local.insteadOf=https://test.local",
					"-c",
					"url.https://gitlab-ci-token:job-token@test.local/.insteadOf=git@test.local:",
					"-c",
					"url.https://gitlab-ci-token:job-token@test.local.insteadOf=ssh://git@test.local",
				},
			},
		},
		"with default url and custom SSH port and force HTTPS": {
			forceHTTPS: true,
			serverPort: "8022",
			expectedArgs: map[bool][]string{
				jobTokenFromEnv: {
					"-c",
					"url.https://test.local.insteadOf=ssh://git@test.local:8022",
				},
				jobTokenInURL: {
					"-c",
					"url.https://gitlab-ci-token:job-token@test.local.insteadOf=https://test.local",
					"-c",
					"url.https://gitlab-ci-token:job-token@test.local.insteadOf=ssh://git@test.local:8022",
				},
			},
		},
		"with default url and trailing slash stripped and force HTTPS": {
			forceHTTPS: true,
			serverURL:  "https://test.local/",
			expectedArgs: map[bool][]string{
				jobTokenFromEnv: {
					"-c",
					"url.https://test.local/.insteadOf=git@test.local:",
					"-c",
					"url.https://test.local.insteadOf=ssh://git@test.local",
				},
				jobTokenInURL: {
					"-c",
					"url.https://gitlab-ci-token:job-token@test.local.insteadOf=https://test.local",
					"-c",
					"url.https://gitlab-ci-token:job-token@test.local/.insteadOf=git@test.local:",
					"-c",
					"url.https://gitlab-ci-token:job-token@test.local.insteadOf=ssh://git@test.local",
				},
			},
		},
		"with default url and directory URL and force HTTPS": {
			forceHTTPS: true,
			serverURL:  "https://test.local/gitlab",
			expectedArgs: map[bool][]string{
				jobTokenFromEnv: {
					"-c",
					"url.https://test.local/gitlab/.insteadOf=git@test.local:",
					"-c",
					"url.https://test.local/gitlab.insteadOf=ssh://git@test.local",
				},
				jobTokenInURL: {
					"-c",
					"url.https://gitlab-ci-token:job-token@test.local/gitlab.insteadOf=https://test.local/gitlab",
					"-c",
					"url.https://gitlab-ci-token:job-token@test.local/gitlab/.insteadOf=git@test.local:",
					"-c",
					"url.https://gitlab-ci-token:job-token@test.local/gitlab.insteadOf=ssh://git@test.local",
				},
			},
		},
		"with clone_url and ssh protocol and force HTTPS ignored": {
			forceHTTPS: true,
			cloneURL:   "ssh://git@test.local",
			expectedArgs: map[bool][]string{
				jobTokenFromEnv: {},
				jobTokenInURL:   {},
			},
		},
	}

	for _, ffState := range []bool{jobTokenFromEnv, jobTokenInURL} {
		ff := featureflags.GitURLsWithoutTokens
		name := fmt.Sprintf("%s:%t", ff, ffState)
		t.Run(name, func(t *testing.T) {
			for tn, tc := range testCases {
				t.Run(tn, func(t *testing.T) {
					build := &Build{
						Runner: &RunnerConfig{
							RunnerCredentials: RunnerCredentials{
								URL: exampleServerURL,
							},
							RunnerSettings: RunnerSettings{
								CloneURL: tc.cloneURL,
							},
						},
						JobResponse: JobResponse{
							Token: exampleJobToken,
						},
					}

					build.Runner.FeatureFlags = map[string]bool{
						ff: ffState,
					}

					if tc.serverURL != "" {
						build.Runner.RunnerCredentials.URL = tc.serverURL
					}

					variables := JobVariables{
						{Key: "CI_SERVER_SHELL_SSH_HOST", Value: exampleServerHost},
					}

					if tc.forceHTTPS {
						variables = append(variables, JobVariable{
							Key:   "GIT_SUBMODULE_FORCE_HTTPS",
							Value: "true",
						})
					}

					if tc.serverPort != "" {
						variables = append(variables, JobVariable{
							Key:   "CI_SERVER_SHELL_SSH_PORT",
							Value: tc.serverPort,
						})
					}

					build.JobResponse.Variables = variables

					gitURLArgs, err := build.GetURLInsteadOfArgs()
					assert.NoError(t, err, "getting git insteadOf URLs")
					assert.Equal(t, tc.expectedArgs[ffState], gitURLArgs)
				})
			}
		})
	}
}

func TestIsFeatureFlagOn(t *testing.T) {
	const testFF = "FF_TEST_FEATURE"

	tests := map[string]struct {
		featureFlagCfg map[string]bool
		value          string
		expectedStatus bool
	}{
		"no value": {
			value:          "",
			expectedStatus: false,
		},
		"true": {
			value:          "true",
			expectedStatus: true,
		},
		"1": {
			value:          "1",
			expectedStatus: true,
		},
		"false": {
			value:          "false",
			expectedStatus: false,
		},
		"0": {
			value:          "0",
			expectedStatus: false,
		},
		"invalid value": {
			value:          "test",
			expectedStatus: false,
		},
		"feature flag set inside config.toml take precedence": {
			featureFlagCfg: map[string]bool{
				testFF: true,
			},
			value:          "false",
			expectedStatus: true,
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			build := new(Build)
			build.Runner = &RunnerConfig{
				RunnerSettings: RunnerSettings{
					FeatureFlags: testCase.featureFlagCfg,
				},
			}
			build.Variables = JobVariables{
				{Key: testFF, Value: testCase.value},
			}

			status := build.IsFeatureFlagOn(testFF)
			assert.Equal(t, testCase.expectedStatus, status)
		})
	}
}

func TestIsFeatureFlagOn_SetWithRunnerVariables(t *testing.T) {
	tests := map[string]struct {
		variable      string
		expectedValue bool
	}{
		"it has default value of FF": {
			variable:      "",
			expectedValue: false,
		},
		"it enables FF": {
			variable:      "FF_NETWORK_PER_BUILD=true",
			expectedValue: true,
		},
		"it disable FF": {
			variable:      "FF_NETWORK_PER_BUILD=false",
			expectedValue: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			build := new(Build)
			build.Runner = &RunnerConfig{
				RunnerSettings: RunnerSettings{
					Environment: []string{test.variable},
				},
			}

			result := build.IsFeatureFlagOn("FF_NETWORK_PER_BUILD")
			assert.Equal(t, test.expectedValue, result)
		})
	}
}

func TestIsFeatureFlagOn_Precedence(t *testing.T) {
	const testFF = "FF_TEST_FEATURE"

	t.Run("config takes precedence over job variable", func(t *testing.T) {
		b := &Build{
			Runner: &RunnerConfig{
				RunnerSettings: RunnerSettings{
					FeatureFlags: map[string]bool{
						testFF: true,
					},
				},
			},
			JobResponse: JobResponse{
				Variables: JobVariables{
					{Key: testFF, Value: "false"},
				},
			},
		}

		assert.True(t, b.IsFeatureFlagOn(testFF))
	})

	t.Run("config takes precedence over configured environments", func(t *testing.T) {
		b := &Build{
			Runner: &RunnerConfig{
				RunnerSettings: RunnerSettings{
					FeatureFlags: map[string]bool{
						testFF: true,
					},
					Environment: []string{testFF + "=false"},
				},
			},
		}

		assert.True(t, b.IsFeatureFlagOn(testFF))
	})

	t.Run("variable defined at job take precedence over configured environments", func(t *testing.T) {
		b := &Build{
			Runner: &RunnerConfig{
				RunnerSettings: RunnerSettings{
					Environment: []string{testFF + "=false"},
				},
			},
			JobResponse: JobResponse{
				Variables: JobVariables{
					{Key: testFF, Value: "true"},
				},
			},
		}

		assert.True(t, b.IsFeatureFlagOn(testFF))
	})
}

func TestGetAllVariables_FeatureFlagResolution(t *testing.T) {
	testFF := featureflags.UseFastzip

	tests := map[string]struct {
		runnerFeatureFlags map[string]bool
		jobVariables       JobVariables
		expectedFFValue    string
		description        string
	}{
		"TOML feature flag appears in GetAllVariables": {
			runnerFeatureFlags: map[string]bool{
				testFF: true,
			},
			expectedFFValue: "true",
			description:     "TOML-configured feature flag should appear in GetAllVariables",
		},
		"TOML overrides job variable in GetAllVariables": {
			runnerFeatureFlags: map[string]bool{
				testFF: true,
			},
			jobVariables: JobVariables{
				{Key: testFF, Value: "false"},
			},
			expectedFFValue: "true",
			description:     "TOML setting should override job variable in GetAllVariables",
		},
		"job variable appears when no TOML setting": {
			jobVariables: JobVariables{
				{Key: testFF, Value: "true"},
			},
			expectedFFValue: "true",
			description:     "Job variable should appear when no TOML setting exists",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			build := &Build{
				Runner: &RunnerConfig{
					RunnerSettings: RunnerSettings{
						FeatureFlags: tc.runnerFeatureFlags,
					},
				},
				JobResponse: JobResponse{
					Variables: tc.jobVariables,
				},
			}

			// GetAllVariables should now contain the resolved feature flag values
			allVars := build.GetAllVariables()
			actualValue := allVars.Value(testFF)

			assert.Equal(t, tc.expectedFFValue, actualValue, tc.description)

			// Verify IsFeatureFlagOn matches GetAllVariables
			expectedBool := tc.expectedFFValue == "true"
			assert.Equal(t, expectedBool, build.IsFeatureFlagOn(testFF),
				"IsFeatureFlagOn should match the value in GetAllVariables")

			// Explicitly verify that TOML settings take precedence in both methods
			if tc.runnerFeatureFlags != nil && tc.jobVariables != nil {
				assert.Equal(t, tc.runnerFeatureFlags[testFF], build.IsFeatureFlagOn(testFF),
					"TOML settings should take precedence over job variables")
			}
		})
	}
}

func TestStartBuild(t *testing.T) {
	type startBuildArgs struct {
		rootDir               string
		cacheDir              string
		customBuildDirEnabled bool
		sharedDir             bool
		safeDirectoryCheckout bool
	}

	tests := map[string]struct {
		args                          startBuildArgs
		jobVariables                  JobVariables
		expectedBuildDir              string
		expectedCacheDir              string
		expectedSafeDirectoryCheckout bool
		expectedError                 bool
	}{
		"no job specific build dir with no shared dir": {
			args: startBuildArgs{
				rootDir:               "/build",
				cacheDir:              "/cache",
				customBuildDirEnabled: true,
				sharedDir:             false,
				safeDirectoryCheckout: false,
			},
			jobVariables:                  JobVariables{},
			expectedBuildDir:              "/build/test-namespace/test-repo",
			expectedCacheDir:              "/cache/test-namespace/test-repo",
			expectedSafeDirectoryCheckout: false,
			expectedError:                 false,
		},
		"no job specified build dir with shared dir": {
			args: startBuildArgs{
				rootDir:               "/builds",
				cacheDir:              "/cache",
				customBuildDirEnabled: true,
				sharedDir:             true,
				safeDirectoryCheckout: false,
			},
			jobVariables:                  JobVariables{},
			expectedBuildDir:              "/builds/1234/0/test-namespace/test-repo",
			expectedCacheDir:              "/cache/test-namespace/test-repo",
			expectedSafeDirectoryCheckout: false,
			expectedError:                 false,
		},
		"valid GIT_CLONE_PATH was specified": {
			args: startBuildArgs{
				rootDir:               "/builds",
				cacheDir:              "/cache",
				customBuildDirEnabled: true,
				sharedDir:             false,
				safeDirectoryCheckout: false,
			},
			jobVariables: JobVariables{
				{Key: "GIT_CLONE_PATH", Value: "/builds/go/src/gitlab.com/test-namespace/test-repo", Public: true},
			},
			expectedBuildDir:              "/builds/go/src/gitlab.com/test-namespace/test-repo",
			expectedCacheDir:              "/cache/test-namespace/test-repo",
			expectedSafeDirectoryCheckout: false,
			expectedError:                 false,
		},
		"valid GIT_CLONE_PATH using CI_BUILDS_DIR was specified": {
			args: startBuildArgs{
				rootDir:               "/builds",
				cacheDir:              "/cache",
				customBuildDirEnabled: true,
				sharedDir:             false,
				safeDirectoryCheckout: false,
			},
			jobVariables: JobVariables{
				{
					Key:    "GIT_CLONE_PATH",
					Value:  "$CI_BUILDS_DIR/go/src/gitlab.com/test-namespace/test-repo",
					Public: true,
				},
			},
			expectedBuildDir:              "/builds/go/src/gitlab.com/test-namespace/test-repo",
			expectedCacheDir:              "/cache/test-namespace/test-repo",
			expectedSafeDirectoryCheckout: false,
			expectedError:                 false,
		},
		"out-of-bounds GIT_CLONE_PATH was specified": {
			args: startBuildArgs{
				rootDir:               "/builds",
				cacheDir:              "/cache",
				customBuildDirEnabled: true,
				sharedDir:             false,
				safeDirectoryCheckout: false,
			},
			jobVariables: JobVariables{
				{
					Key:    "GIT_CLONE_PATH",
					Value:  "/builds/../outside",
					Public: true,
				},
			},
			expectedError: true,
		},
		"custom build disabled": {
			args: startBuildArgs{
				rootDir:               "/builds",
				cacheDir:              "/cache",
				customBuildDirEnabled: false,
				sharedDir:             false,
				safeDirectoryCheckout: false,
			},
			jobVariables: JobVariables{
				{Key: "GIT_CLONE_PATH", Value: "/builds/go/src/gitlab.com/test-namespace/test-repo", Public: true},
			},
			expectedBuildDir:              "/builds/test-namespace/test-repo",
			expectedCacheDir:              "/cache/test-namespace/test-repo",
			expectedSafeDirectoryCheckout: false,
			expectedError:                 true,
		},
		"invalid GIT_CLONE_PATH was specified": {
			args: startBuildArgs{
				rootDir:               "/builds",
				cacheDir:              "/cache",
				customBuildDirEnabled: true,
				sharedDir:             false,
				safeDirectoryCheckout: false,
			},
			jobVariables: JobVariables{
				{Key: "GIT_CLONE_PATH", Value: "/go/src/gitlab.com/test-namespace/test-repo", Public: true},
			},
			expectedError: true,
		},
		"safeDirectoryCheckout enabled": {
			args: startBuildArgs{
				rootDir:               "/builds",
				cacheDir:              "/cache",
				customBuildDirEnabled: false,
				sharedDir:             false,
				safeDirectoryCheckout: true,
			},
			jobVariables:                  nil,
			expectedBuildDir:              "/builds/test-namespace/test-repo",
			expectedCacheDir:              "/cache/test-namespace/test-repo",
			expectedSafeDirectoryCheckout: true,
			expectedError:                 false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			build := Build{
				JobResponse: JobResponse{
					GitInfo: GitInfo{
						RepoURL: "https://gitlab.com/test-namespace/test-repo.git",
					},
					Variables: test.jobVariables,
				},
				Runner: &RunnerConfig{
					RunnerCredentials: RunnerCredentials{
						Token: "1234",
					},
				},
			}

			err := build.StartBuild(
				test.args.rootDir,
				test.args.cacheDir,
				test.args.customBuildDirEnabled,
				test.args.sharedDir,
				test.args.safeDirectoryCheckout,
			)
			if test.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.expectedBuildDir, build.BuildDir)
			assert.Equal(t, test.args.rootDir, build.RootDir)
			assert.Equal(t, test.expectedCacheDir, build.CacheDir)
			assert.Equal(t, test.expectedSafeDirectoryCheckout, build.SafeDirectoryCheckout)
		})
	}
}

func TestTmpProjectDir(t *testing.T) {
	createTestBuild := func(variables JobVariables) Build {
		return Build{
			JobResponse: JobResponse{
				GitInfo: GitInfo{
					RepoURL: "https://gitlab.com/test-namespace/test-repo.git",
				},
				Variables: variables,
			},
			Runner: &RunnerConfig{
				RunnerCredentials: RunnerCredentials{
					Token: "1234",
				},
			},
		}
	}

	type startBuildArgs struct {
		rootDir               string
		cacheDir              string
		customBuildDirEnabled bool
		sharedDir             bool
	}
	testStartBuildArgs := startBuildArgs{
		rootDir:               "/builds",
		cacheDir:              "/cache",
		customBuildDirEnabled: true,
		sharedDir:             false,
	}

	tests := map[string]struct {
		args                  startBuildArgs
		jobVariables          JobVariables
		expectedTmpProjectDir string
		expectedError         bool
	}{
		"test default build dir": {
			args:                  testStartBuildArgs,
			jobVariables:          nil,
			expectedError:         false,
			expectedTmpProjectDir: "/builds/test-namespace/test-repo.tmp",
		},
		"test custom build dir with double trailing slashes": {
			args: testStartBuildArgs,
			jobVariables: JobVariables{
				{Key: "GIT_CLONE_PATH", Value: "/builds/go/src/gitlab.com/test-namespace/test-repo//", Public: true},
			},
			expectedError:         false,
			expectedTmpProjectDir: "/builds/go/src/gitlab.com/test-namespace/test-repo.tmp",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			build := createTestBuild(tt.jobVariables)

			err := build.StartBuild(
				tt.args.rootDir,
				tt.args.cacheDir,
				tt.args.customBuildDirEnabled,
				tt.args.sharedDir,
				false,
			)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			dir := build.TmpProjectDir()
			assert.Equal(t, tt.expectedTmpProjectDir, dir)
		})
	}
}

func TestSkipBuildStageFeatureFlag(t *testing.T) {
	featureFlagValues := []string{
		"true",
		"false",
	}

	s := NewMockShell(t)

	s.On("GetName").Return("skip-build-stage-shell")
	RegisterShell(s)

	for _, value := range featureFlagValues {
		t.Run(value, func(t *testing.T) {
			build := &Build{
				Runner: &RunnerConfig{},
				JobResponse: JobResponse{
					Variables: JobVariables{
						{
							Key:   featureflags.SkipNoOpBuildStages,
							Value: "false",
						},
					},
				},
			}

			e := NewMockExecutor(t)
			s.On("GenerateScript", mock.Anything, mock.Anything, mock.Anything).Return("script", ErrSkipBuildStage)
			e.On("Shell").Return(&ShellScriptInfo{Shell: "skip-build-stage-shell"})

			if !build.IsFeatureFlagOn(featureflags.SkipNoOpBuildStages) {
				e.On("Run", matchBuildStage(BuildStageAfterScript)).Return(nil).Once()
			}

			err := build.executeStage(context.Background(), BuildStageAfterScript, e)
			assert.NoError(t, err)
		})
	}
}

func TestWaitForTerminal(t *testing.T) {
	cases := []struct {
		name                   string
		cancelFn               func(ctxCancel context.CancelFunc, build *Build)
		jobTimeout             int
		waitForTerminalTimeout time.Duration
		expectedErr            string
	}{
		{
			name: "Cancel build",
			cancelFn: func(ctxCancel context.CancelFunc, build *Build) {
				ctxCancel()
			},
			jobTimeout:             3600,
			waitForTerminalTimeout: time.Hour,
			expectedErr:            "build cancelled, killing session",
		},
		{
			name: "Terminal Timeout",
			cancelFn: func(ctxCancel context.CancelFunc, build *Build) {
				// noop
			},
			jobTimeout:             3600,
			waitForTerminalTimeout: time.Second,
			expectedErr:            "terminal session timed out (maximum time allowed - 1s)",
		},
		{
			name: "System Interrupt",
			cancelFn: func(ctxCancel context.CancelFunc, build *Build) {
				build.SystemInterrupt <- os.Interrupt
			},
			jobTimeout:             3600,
			waitForTerminalTimeout: time.Hour,
			expectedErr:            "terminal disconnected by system signal: interrupt",
		},
		{
			name: "Terminal Disconnect",
			cancelFn: func(ctxCancel context.CancelFunc, build *Build) {
				build.Session.DisconnectCh <- errors.New("user disconnect")
			},
			jobTimeout:             3600,
			waitForTerminalTimeout: time.Hour,
			expectedErr:            "terminal disconnected: user disconnect",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			build := Build{
				Runner: &RunnerConfig{
					RunnerSettings: RunnerSettings{
						Executor: "shell",
					},
				},
				JobResponse: JobResponse{
					RunnerInfo: RunnerInfo{
						Timeout: c.jobTimeout,
					},
				},
				SystemInterrupt: make(chan os.Signal),
			}

			trace := Trace{Writer: os.Stdout}
			build.logger = buildlogger.New(&trace, build.Log(), buildlogger.Options{})
			sess, err := session.NewSession(nil)
			require.NoError(t, err)
			build.Session = sess

			srv := httptest.NewServer(build.Session.Handler())
			defer srv.Close()

			mockConn := terminal.NewMockConn(t)
			mockConn.On("Close").Maybe().Return(nil)
			// On Start upgrade the web socket connection and wait for the
			// timeoutCh to exit, to mock real work made on the websocket.
			mockConn.
				On("Start", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					upgrader := &websocket.Upgrader{}
					r := args[1].(*http.Request)
					w := args[0].(http.ResponseWriter)

					_, _ = upgrader.Upgrade(w, r, nil)
					timeoutCh := args[2].(chan error)

					<-timeoutCh
				}).Once()

			mockTerminal := terminal.NewMockInteractiveTerminal(t)
			mockTerminal.On("Connect").Return(mockConn, nil)
			sess.SetInteractiveTerminal(mockTerminal)

			u := url.URL{
				Scheme: "ws",
				Host:   srv.Listener.Addr().String(),
				Path:   build.Session.Endpoint + "/exec",
			}
			headers := http.Header{
				"Authorization": []string{build.Session.Token},
			}

			conn, resp, err := websocket.DefaultDialer.Dial(u.String(), headers)
			require.NotNil(t, conn)
			require.NoError(t, err)
			defer func() {
				resp.Body.Close()
				conn.Close()
			}()

			ctx, cancel := context.WithTimeout(context.Background(), build.GetBuildTimeout())

			errCh := make(chan error)
			go func() {
				errCh <- build.waitForTerminal(ctx, c.waitForTerminalTimeout)
			}()

			c.cancelFn(cancel, &build)

			assert.EqualError(t, <-errCh, c.expectedErr)
		})
	}
}

func TestBuild_IsLFSSmudgeDisabled(t *testing.T) {
	testCases := map[string]struct {
		isVariableUnset bool
		variableValue   string
		expectedResult  bool
	}{
		"variable not set": {
			isVariableUnset: true,
			expectedResult:  false,
		},
		"variable empty": {
			variableValue:  "",
			expectedResult: false,
		},
		"variable set to true": {
			variableValue:  "true",
			expectedResult: true,
		},
		"variable set to false": {
			variableValue:  "false",
			expectedResult: false,
		},
		"variable set to 1": {
			variableValue:  "1",
			expectedResult: true,
		},
		"variable set to 0": {
			variableValue:  "0",
			expectedResult: false,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			b := &Build{
				JobResponse: JobResponse{
					Variables: JobVariables{},
				},
			}

			if !testCase.isVariableUnset {
				b.Variables = append(
					b.Variables,
					JobVariable{Key: "GIT_LFS_SKIP_SMUDGE", Value: testCase.variableValue, Public: true},
				)
			}

			assert.Equal(t, testCase.expectedResult, b.IsLFSSmudgeDisabled())
		})
	}
}

func TestGitSubmodulePaths(t *testing.T) {
	tests := map[string]struct {
		isVariableSet  bool
		value          string
		expectedResult []string
		expectedError  bool
	}{
		"not defined": {
			isVariableSet:  false,
			value:          "",
			expectedResult: nil,
			expectedError:  false,
		},
		"empty": {
			isVariableSet:  true,
			value:          "",
			expectedResult: nil,
			expectedError:  false,
		},
		"select submodule 1": {
			isVariableSet:  true,
			value:          "submodule1",
			expectedResult: []string{"submodule1"},
			expectedError:  false,
		},
		"select submodule 1 and 2": {
			isVariableSet:  true,
			value:          "submodule1 submodule2",
			expectedResult: []string{"submodule1", "submodule2"},
			expectedError:  false,
		},
		"select submodule 1 and exclude 2": {
			isVariableSet:  true,
			value:          "submodule1 :(exclude)submodule2",
			expectedResult: []string{"submodule1", ":(exclude)submodule2"},
			expectedError:  false,
		},
		"exclude submodule 1": {
			isVariableSet:  true,
			value:          " :(exclude)submodule1",
			expectedResult: []string{":(exclude)submodule1"},
			expectedError:  false,
		},
		"exclude submodule 1 and 2": {
			isVariableSet:  true,
			value:          " :(exclude)submodule1 :(exclude)submodule2 ",
			expectedResult: []string{":(exclude)submodule1", ":(exclude)submodule2"},
			expectedError:  false,
		},
		"exclude submodule with single space": {
			isVariableSet:  true,
			value:          ":(exclude) gitlab-grack",
			expectedResult: nil,
			expectedError:  true,
		},
		"exclude submodule with multiple spaces": {
			isVariableSet:  true,
			value:          ":(exclude)  gitlab-grack",
			expectedResult: nil,
			expectedError:  true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			build := &Build{
				Runner: &RunnerConfig{},
				JobResponse: JobResponse{
					Variables: JobVariables{},
				},
			}

			if test.isVariableSet {
				build.Variables = append(
					build.Variables,
					JobVariable{Key: "GIT_SUBMODULE_PATHS", Value: test.value, Public: true},
				)
			}

			result, err := build.GetSubmodulePaths()
			if test.expectedError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid submodule pathspec")
			} else {
				assert.Equal(t, test.expectedResult, result)
				assert.NoError(t, err)
			}
		})
	}
}

func TestGitCleanFlags(t *testing.T) {
	tests := map[string]struct {
		value          string
		expectedResult []string
	}{
		"empty clean flags": {
			value:          "",
			expectedResult: []string{"-ffdx"},
		},
		"use custom flags": {
			value:          "custom-flags",
			expectedResult: []string{"custom-flags"},
		},
		"use custom flags with multiple arguments": {
			value:          "-ffdx -e cache/",
			expectedResult: []string{"-ffdx", "-e", "cache/"},
		},
		"disabled": {
			value:          "none",
			expectedResult: []string{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			build := &Build{
				Runner: &RunnerConfig{},
				JobResponse: JobResponse{
					Variables: JobVariables{
						{Key: "GIT_CLEAN_FLAGS", Value: test.value},
					},
				},
			}

			result := build.GetGitCleanFlags()
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestGitFetchFlags(t *testing.T) {
	tests := map[string]struct {
		value          string
		expectedResult []string
	}{
		"empty fetch flags": {
			value:          "",
			expectedResult: []string{"--prune", "--quiet"},
		},
		"use custom flags": {
			value:          "custom-flags",
			expectedResult: []string{"custom-flags"},
		},
		"use custom flags with multiple arguments": {
			value:          "--prune --tags --quiet",
			expectedResult: []string{"--prune", "--tags", "--quiet"},
		},
		"disabled": {
			value:          "none",
			expectedResult: []string{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			build := &Build{
				Runner: &RunnerConfig{},
				JobResponse: JobResponse{
					Variables: JobVariables{
						{Key: "GIT_FETCH_EXTRA_FLAGS", Value: test.value},
					},
				},
			}

			result := build.GetGitFetchFlags()
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestGetRepositoryObjectFormat(t *testing.T) {
	tests := map[string]struct {
		value          string
		expectedResult string
	}{
		"empty value": {
			value:          "",
			expectedResult: "sha1",
		},
		"sha1": {
			value:          "sha1",
			expectedResult: "sha1",
		},
		"sha256": {
			value:          "sha256",
			expectedResult: "sha256",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			build := &Build{
				Runner: &RunnerConfig{},
				JobResponse: JobResponse{
					GitInfo: GitInfo{
						RepoObjectFormat: test.value,
					},
				},
			}

			result := build.GetRepositoryObjectFormat()
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestGitSubmoduleUpdateFlags(t *testing.T) {
	tests := map[string]struct {
		value          string
		expectedResult []string
	}{
		"empty update flags": {
			value:          "",
			expectedResult: nil,
		},
		"use custom update flags": {
			value:          "custom-flags",
			expectedResult: []string{"custom-flags"},
		},
		"use custom update flags with multiple arguments": {
			value:          "--remote --jobs 4",
			expectedResult: []string{"--remote", "--jobs", "4"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			build := &Build{
				Runner: &RunnerConfig{},
				JobResponse: JobResponse{
					Variables: JobVariables{
						{Key: "GIT_SUBMODULE_UPDATE_FLAGS", Value: test.value},
					},
				},
			}

			result := build.GetGitSubmoduleUpdateFlags()
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestDefaultVariables(t *testing.T) {
	tests := map[string]struct {
		jobVariables  JobVariables
		rootDir       string
		key           string
		expectedValue string
	}{
		"get default CI_SERVER value": {
			jobVariables:  JobVariables{},
			rootDir:       "/builds",
			key:           "CI_SERVER",
			expectedValue: "yes",
		},
		"get default CI_PROJECT_DIR value": {
			jobVariables:  JobVariables{},
			rootDir:       "/builds",
			key:           "CI_PROJECT_DIR",
			expectedValue: "/builds/test-namespace/test-repo",
		},
		"get overwritten CI_PROJECT_DIR value": {
			jobVariables: JobVariables{
				{Key: "GIT_CLONE_PATH", Value: "/builds/go/src/gitlab.com/gitlab-org/gitlab-runner", Public: true},
			},
			rootDir:       "/builds",
			key:           "CI_PROJECT_DIR",
			expectedValue: "/builds/go/src/gitlab.com/gitlab-org/gitlab-runner",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			build := Build{
				JobResponse: JobResponse{
					GitInfo: GitInfo{
						RepoURL: "https://gitlab.com/test-namespace/test-repo.git",
					},
					Variables: test.jobVariables,
				},
				Runner: &RunnerConfig{
					RunnerCredentials: RunnerCredentials{
						Token: "1234",
					},
				},
			}

			err := build.StartBuild(test.rootDir, "/cache", true, false, false)
			assert.NoError(t, err)

			variable := build.GetAllVariables().Get(test.key)
			assert.Equal(t, test.expectedValue, variable)
		})
	}
}

func TestBuildFinishTimeout(t *testing.T) {
	tests := map[string]bool{
		"channel returns first": true,
		"timeout returns first": false,
	}

	for name, chanFirst := range tests {
		t.Run(name, func(t *testing.T) {
			logger, hooks := test.NewNullLogger()
			build := Build{
				logger: buildlogger.New(nil, logrus.NewEntry(logger), buildlogger.Options{}),
			}
			buildFinish := make(chan error, 1)
			timeout := 10 * time.Millisecond

			if chanFirst {
				buildFinish <- errors.New("job finish error")
			}

			build.waitForBuildFinish(buildFinish, timeout)

			entry := hooks.LastEntry()

			if chanFirst {
				assert.Nil(t, entry)
				return
			}

			assert.NotNil(t, entry)
		})
	}
}

func TestProjectUniqueName(t *testing.T) {
	tests := map[string]struct {
		build        *Build
		expectedName string
	}{
		"project non rfc1132 unique name": {
			build: &Build{
				Runner: &RunnerConfig{
					RunnerCredentials: RunnerCredentials{
						Token: "Ze_n8E6en622WxxSg4r8",
					},
				},
				JobResponse: JobResponse{
					JobInfo: JobInfo{
						ProjectID: 1234567890,
					},
				},
				ProjectRunnerID: 0,
			},
			expectedName: "runner-zen8e6en-project-1234567890-concurrent-0",
		},
		"project normal unique name": {
			build: &Build{
				Runner: &RunnerConfig{
					RunnerCredentials: RunnerCredentials{
						Token: "xYzWabc-Ij3xlKjmoPO9",
					},
				},
				JobResponse: JobResponse{
					JobInfo: JobInfo{
						ProjectID: 1234567890,
					},
				},
				ProjectRunnerID: 0,
			},
			expectedName: "runner-xyzwabc-i-project-1234567890-concurrent-0",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expectedName, test.build.ProjectUniqueName())
		})
	}
}

func TestProjectUniqueShortName(t *testing.T) {
	tests := map[string]struct {
		build        *Build
		expectedName string
	}{
		"project non rfc1132 unique name": {
			build: &Build{
				Runner: &RunnerConfig{
					RunnerCredentials: RunnerCredentials{
						Token: "Ze_n8E6en622WxxSg4r8",
					},
				},
				JobResponse: JobResponse{
					JobInfo: JobInfo{
						ProjectID: 1234567890,
					},
				},
				ProjectRunnerID: 0,
			},
			expectedName: "runner-zen8e6en-1234567890-0-0",
		},
		"project normal unique name without build id": {
			build: &Build{
				Runner: &RunnerConfig{
					RunnerCredentials: RunnerCredentials{
						Token: "xYzWabc-Ij3xlKjmoPO9",
					},
				},
				JobResponse: JobResponse{
					JobInfo: JobInfo{
						ProjectID: 1234567890,
					},
				},
				ProjectRunnerID: 0,
			},
			expectedName: "runner-xyzwabc-i-1234567890-0-0",
		},
		"project normal unique name with build id": {
			build: &Build{
				Runner: &RunnerConfig{
					RunnerCredentials: RunnerCredentials{
						Token: "xYzWabc-Ij3xlKjmoPO9",
					},
				},
				JobResponse: JobResponse{
					ID: 12345,
					JobInfo: JobInfo{
						ProjectID: 1234567890,
					},
				},
				ProjectRunnerID: 222222,
			},
			expectedName: "runner-xyzwabc-i-1234567890-222222-12345",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expectedName, test.build.ProjectUniqueShortName())
		})
	}
}

func TestBuildStages(t *testing.T) {
	scriptOnlyBuild, err := GetRemoteSuccessfulBuild()
	require.NoError(t, err)

	multistepBuild, err := GetRemoteSuccessfulMultistepBuild()
	require.NoError(t, err)

	tests := map[string]struct {
		jobResponse    JobResponse
		expectedStages []BuildStage
	}{
		"script only build": {
			jobResponse:    scriptOnlyBuild,
			expectedStages: append(staticBuildStages, "step_script"),
		},
		"multistep build": {
			jobResponse:    multistepBuild,
			expectedStages: append(staticBuildStages, "step_script", "step_release"),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			build := &Build{
				JobResponse: tt.jobResponse,
			}
			assert.ElementsMatch(t, tt.expectedStages, build.BuildStages())
		})
	}
}

func TestBuild_GetExecutorJobSectionAttempts(t *testing.T) {
	tests := []struct {
		attempts         string
		expectedAttempts int
		expectedErr      bool
	}{
		{
			attempts:         "",
			expectedAttempts: 1,
		},
		{
			attempts:         "3",
			expectedAttempts: 3,
		},
		{
			attempts:         "0",
			expectedAttempts: DefaultExecutorStageAttempts,
			expectedErr:      true,
		},
		{
			attempts:         "99",
			expectedAttempts: DefaultExecutorStageAttempts,
			expectedErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.attempts, func(t *testing.T) {
			build := Build{
				JobResponse: JobResponse{
					Variables: JobVariables{
						JobVariable{
							Key:   ExecutorJobSectionAttempts,
							Value: tt.attempts,
						},
					},
				},
			}

			attempts := build.GetExecutorJobSectionAttempts()
			if tt.expectedErr {
				assert.NotEmpty(t, build.Settings().Errors)
			}
			assert.Equal(t, tt.expectedAttempts, attempts)
		})
	}
}

func TestBuild_getFeatureFlagInfo(t *testing.T) {
	const changedFeatureFlags = "FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY:true"
	tests := []struct {
		value          string
		expectedStatus string
	}{
		{
			value:          "true",
			expectedStatus: changedFeatureFlags,
		},
		{
			value:          "1",
			expectedStatus: changedFeatureFlags,
		},
		{
			value:          "invalid",
			expectedStatus: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			b := Build{
				JobResponse: JobResponse{
					Variables: JobVariables{
						{
							Key:    featureflags.UseLegacyKubernetesExecutionStrategy,
							Value:  tt.value,
							Public: true,
						},
					},
				},
				Runner: &RunnerConfig{},
			}

			assert.Equal(t, tt.expectedStatus, b.getFeatureFlagInfo())
		})
	}
}

func setupSuccessfulMockExecutor(
	t *testing.T,
	prepareFn func(options ExecutorPrepareOptions) error,
) *MockExecutorProvider {
	executor, provider := setupMockExecutorAndProvider(t)

	// We run everything once
	executor.On("Prepare", mock.Anything).Return(prepareFn).Once()
	executor.On("Finish", nil).Once()
	executor.On("Cleanup").Once()

	// Run script successfully
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	executor.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(nil).Once()
	executor.On("Run", matchBuildStage("step_script")).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageAfterScript)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageArchiveOnSuccessCache)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageUploadOnSuccessArtifacts)).
		Return(nil).
		Once()
	executor.On("Run", matchBuildStage(BuildStageCleanup)).
		Return(nil).
		Once()

	return provider
}

func setupMockExecutorAndProvider(t *testing.T) (*MockExecutor, *MockExecutorProvider) {
	e := NewMockExecutor(t)
	p := NewMockExecutorProvider(t)

	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()
	p.On("Create").Return(e).Once()

	return e, p
}

func registerExecutorWithSuccessfulBuild(t *testing.T, p *MockExecutorProvider, rc *RunnerConfig) *Build {
	require.NotNil(t, rc)

	RegisterExecutorProviderForTest(t, t.Name(), p)

	successfulBuild, err := GetSuccessfulBuild()
	require.NoError(t, err)
	if rc.RunnerSettings.Executor == "" {
		// Ensure we set the executor name if not already defined
		rc.RunnerSettings.Executor = t.Name()
	}
	build, err := NewBuild(successfulBuild, rc, nil, nil)
	assert.NoError(t, err)
	return build
}

func runSuccessfulMockBuild(t *testing.T, prepareFn func(options ExecutorPrepareOptions) error) *Build {
	p := setupSuccessfulMockExecutor(t, prepareFn)

	build := registerExecutorWithSuccessfulBuild(t, p, new(RunnerConfig))
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)

	return build
}

func TestSecretsResolving(t *testing.T) {
	exampleVariables := JobVariables{
		{Key: "key", Value: "value"},
	}

	setupFailureExecutorMocks := func(t *testing.T) *MockExecutorProvider {
		p := NewMockExecutorProvider(t)

		p.On("CanCreate").Return(true).Once()
		p.On("GetDefaultShell").Return("bash").Once()
		p.On("GetFeatures", mock.Anything).Return(nil).Once()

		return p
	}

	secrets := Secrets{
		"TEST_SECRET": Secret{
			Vault: &VaultSecret{},
		},
	}

	tests := map[string]struct {
		secrets                 Secrets
		resolverCreationError   error
		prepareExecutorProvider func(t *testing.T) *MockExecutorProvider
		returnVariables         JobVariables
		resolvingError          error
		expectedVariables       JobVariables
		expectedError           error
	}{
		"secrets not present": {
			prepareExecutorProvider: func(t *testing.T) *MockExecutorProvider {
				return setupSuccessfulMockExecutor(t, func(options ExecutorPrepareOptions) error { return nil })
			},
			expectedError: nil,
		},
		"error on creating resolver": {
			secrets:                 secrets,
			resolverCreationError:   assert.AnError,
			prepareExecutorProvider: setupFailureExecutorMocks,
			expectedError:           assert.AnError,
		},
		"error on secrets resolving": {
			secrets:                 secrets,
			prepareExecutorProvider: setupFailureExecutorMocks,
			returnVariables:         exampleVariables,
			resolvingError:          assert.AnError,
			expectedVariables:       nil,
			expectedError:           assert.AnError,
		},
		"secrets resolved": {
			secrets: secrets,
			prepareExecutorProvider: func(t *testing.T) *MockExecutorProvider {
				return setupSuccessfulMockExecutor(t, func(options ExecutorPrepareOptions) error { return nil })
			},
			returnVariables:   exampleVariables,
			resolvingError:    nil,
			expectedVariables: exampleVariables,
			expectedError:     nil,
		},
		"secret not found - FF_SECRET_RESOLVING_FAILS_IF_MISSING enabled": {
			secrets:                 secrets,
			prepareExecutorProvider: setupFailureExecutorMocks,
			returnVariables:         nil,
			resolvingError:          fmt.Errorf("%w: %s", ErrSecretNotFound, "secret_key"),
			expectedVariables:       nil,
			expectedError:           ErrSecretNotFound,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			secretsResolverMock := NewMockSecretsResolver(t)
			p := tt.prepareExecutorProvider(t)

			RegisterExecutorProviderForTest(t, t.Name(), p)

			successfulBuild, err := GetSuccessfulBuild()
			require.NoError(t, err)

			successfulBuild.Secrets = tt.secrets

			if tt.resolverCreationError == nil && tt.secrets != nil {
				secretsResolverMock.On("Resolve", tt.secrets).
					Return(tt.returnVariables, tt.resolvingError).
					Once()
			}

			rc := new(RunnerConfig)
			rc.RunnerSettings.Executor = t.Name()

			build, err := NewBuild(successfulBuild, rc, nil, nil)
			assert.NoError(t, err)

			build.secretsResolver = func(_ logger, _ SecretResolverRegistry, _ func(string) bool) (SecretsResolver, error) {
				return secretsResolverMock, tt.resolverCreationError
			}

			err = build.Run(&Config{}, &Trace{Writer: os.Stdout})

			assert.Equal(t, tt.expectedVariables, build.secretsVariables)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestSetTraceStatus(t *testing.T) {
	tests := map[string]struct {
		err    error
		assert func(*testing.T, *MockJobTrace, error)
	}{
		"nil error is successful": {
			err: nil,
			assert: func(t *testing.T, mt *MockJobTrace, err error) {
				mt.On("Success").Return(nil).Once()
			},
		},
		"build error, script failure": {
			err: &BuildError{FailureReason: ScriptFailure},
			assert: func(t *testing.T, mt *MockJobTrace, err error) {
				mt.On("Fail", err, JobFailureData{Reason: ScriptFailure}).Return(nil).Once()
			},
		},
		"build error, wrapped script failure": {
			err: fmt.Errorf("wrapped: %w", &BuildError{FailureReason: ScriptFailure}),
			assert: func(t *testing.T, mt *MockJobTrace, err error) {
				mt.On("Fail", err, JobFailureData{Reason: ScriptFailure}).Return(nil).Once()
			},
		},
		"non-build error": {
			err: fmt.Errorf("some error"),
			assert: func(t *testing.T, mt *MockJobTrace, err error) {
				mt.On("Fail", err, JobFailureData{Reason: RunnerSystemFailure}).Return(nil).Once()
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			b := &Build{
				Runner: &RunnerConfig{},
			}

			trace := NewMockJobTrace(t)
			trace.On("IsStdout").Return(true)
			trace.On("Write", mock.Anything).Return(0, nil)

			var be *BuildError
			if errors.As(tc.err, &be) {
				trace.On("SetSupportedFailureReasonMapper", mock.Anything).Once()
			}

			tc.assert(t, trace, tc.err)
			b.setTraceStatus(trace, tc.err)
		})
	}
}

func Test_GetDebugServicePolicy(t *testing.T) {
	tests := map[string]struct {
		variable JobVariable
		want     bool
		wantLog  string
	}{
		"empty": {want: false},
		"disabled": {
			variable: JobVariable{Key: "CI_DEBUG_SERVICES", Value: "false", Public: true},
			want:     false,
		},
		"bogus value": {
			variable: JobVariable{Key: "CI_DEBUG_SERVICES", Value: "blammo", Public: true},
			want:     false,
			wantLog:  "CI_DEBUG_SERVICES: expected bool got \"blammo\", using default value: false",
		},
		"enabled": {
			variable: JobVariable{Key: "CI_DEBUG_SERVICES", Value: "true", Public: true},
			want:     true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			b := &Build{
				Runner:      &RunnerConfig{},
				JobResponse: JobResponse{Variables: []JobVariable{tt.variable}},
			}

			got := b.IsCIDebugServiceEnabled()

			assert.Equal(t, tt.want, got)

			if tt.wantLog == "" {
				assert.Empty(t, b.Settings().Errors)
			} else {
				assert.Contains(t, errors.Join(b.Settings().Errors...).Error(), tt.wantLog)
			}
		})
	}
}

func Test_expandContainerOptions(t *testing.T) {
	testCases := map[string]struct {
		jobVars  JobVariables
		image    Image
		services Services
	}{
		"no expansion required": {
			image: Image{Name: "alpine:latest", Alias: "jobctr"},
			services: Services{
				{Name: "postgres:latest", Alias: "db, pg"},
				{Name: "redis:latest", Alias: "cache"},
			},
		},
		"expansion required": {
			jobVars: JobVariables{
				{Key: "JOB_IMAGE", Value: "alpine:latest"},
				{Key: "JOB_ALIAS", Value: "jobctr"},
				{Key: "DB_IMAGE", Value: "postgres:latest"},
				{Key: "DB_IMAGE_ALIAS", Value: "db"},
				{Key: "CACHE_IMAGE", Value: "redis:latest"},
				{Key: "CACHE_IMAGE_ALIAS", Value: "cache"},
			},
			image: Image{Name: "$JOB_IMAGE", Alias: "$JOB_ALIAS"},
			services: Services{
				{Name: "$DB_IMAGE", Alias: "$DB_IMAGE_ALIAS, pg"},
				{Name: "$CACHE_IMAGE", Alias: "$CACHE_IMAGE_ALIAS"},
			},
		},
	}

	for name, tt := range testCases {
		t.Run(name, func(t *testing.T) {
			b := &Build{
				Runner: &RunnerConfig{},
				JobResponse: JobResponse{
					Variables: tt.jobVars,
					Image:     tt.image,
					Services:  tt.services,
				},
			}
			b.GetAllVariables()
			b.expandContainerOptions()

			assert.Equal(t, "alpine:latest", b.Image.Name)
			assert.Equal(t, "jobctr", b.Image.Alias)

			assert.Len(t, b.Services, 2)

			assert.Equal(t, "postgres:latest", b.Services[0].Name)
			assert.Equal(t, []string{"db", "pg"}, b.Services[0].Aliases())
			assert.Equal(t, "redis:latest", b.Services[1].Name)
			assert.Equal(t, []string{"cache"}, b.Services[1].Aliases())
		})
	}
}

func TestPrintPolicyOptions(t *testing.T) {
	falseValue := false
	trueValue := true
	testCases := []struct {
		desc          string
		policyOptions PolicyOptions
		contains      []string
	}{
		{
			desc: "without policy options",
		},
		{
			desc: "not a policy job",
			policyOptions: PolicyOptions{
				PolicyJob: false,
			},
		},
		{
			desc: "policy job without override",
			policyOptions: PolicyOptions{
				PolicyJob: true,
				Name:      "Test Policy",
			},
			contains: []string{`Job triggered by policy \"Test Policy\".`, "Variables defined in the policy take precedence over matching user-defined CI/CD variables for this job."},
		},
		{
			desc: "policy job with override allowed",
			policyOptions: PolicyOptions{
				PolicyJob:               true,
				Name:                    "Test Policy",
				VariableOverrideAllowed: &trueValue,
			},
			contains: []string{`Job triggered by policy \"Test Policy\".`, "User-defined CI/CD variables are allowed in this job according to the pipeline execution policy."},
		},
		{
			desc: "policy job with override allowed with exceptions",
			policyOptions: PolicyOptions{
				PolicyJob:                  true,
				Name:                       "Test Policy",
				VariableOverrideAllowed:    &trueValue,
				VariableOverrideExceptions: []string{"EXCEPTION_VAR1", "EXCEPTION_VAR2"},
			},
			contains: []string{`Job triggered by policy \"Test Policy\".`, "User-defined CI/CD variables are allowed in this job (except for EXCEPTION_VAR1, EXCEPTION_VAR2) according to the pipeline execution policy."},
		},
		{
			desc: "policy job with override denied",
			policyOptions: PolicyOptions{
				PolicyJob:               true,
				Name:                    "Test Policy",
				VariableOverrideAllowed: &falseValue,
			},
			contains: []string{`Job triggered by policy \"Test Policy\".`, "User-defined CI/CD variables are ignored in this job according to the pipeline execution policy."},
		},
		{
			desc: "policy job with override denied with exceptions",
			policyOptions: PolicyOptions{
				PolicyJob:                  true,
				Name:                       "Test Policy",
				VariableOverrideAllowed:    &falseValue,
				VariableOverrideExceptions: []string{"EXCEPTION_VAR1", "EXCEPTION_VAR2"},
			},
			contains: []string{`Job triggered by policy \"Test Policy\".`, "User-defined CI/CD variables are ignored in this job (except for EXCEPTION_VAR1, EXCEPTION_VAR2) according to the pipeline execution policy."},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			logs := bytes.Buffer{}
			lentry := logrus.New()
			lentry.Out = &logs
			logger := buildlogger.New(nil, logrus.NewEntry(lentry), buildlogger.Options{})

			b := &Build{
				Runner: &RunnerConfig{},
				JobResponse: JobResponse{
					PolicyOptions: tc.policyOptions,
				},
				logger: logger,
			}

			b.printPolicyOptions()

			if len(tc.contains) == 0 {
				assert.Empty(t, logs.String())
			} else {
				for i := range tc.contains {
					assert.Contains(t, logs.String(), tc.contains[i])
				}
			}
		})
	}
}

func TestGetStageTimeoutContexts(t *testing.T) {
	defaultTimeouts := []stageTimeout{
		{configName: "RUNNER_SCRIPT_TIMEOUT", defaultTimeout: 0},
		{configName: "RUNNER_AFTER_SCRIPT_TIMEOUT", defaultTimeout: 5 * time.Minute},
	}

	tests := map[string]struct {
		variables  map[string]string
		expected   map[string]time.Duration
		contains   []string
		jobTimeout time.Duration
	}{
		"after_script must have a timeout, even if set to zero": {
			variables: map[string]string{
				"RUNNER_AFTER_SCRIPT_TIMEOUT": "0s",
			},
			expected: map[string]time.Duration{
				"RUNNER_SCRIPT_TIMEOUT":       time.Hour,
				"RUNNER_AFTER_SCRIPT_TIMEOUT": 5 * time.Minute,
			},
			jobTimeout: time.Hour,
		},
		"no timeout provided": {
			variables: map[string]string{},
			expected: map[string]time.Duration{
				"RUNNER_SCRIPT_TIMEOUT":       time.Hour,
				"RUNNER_AFTER_SCRIPT_TIMEOUT": 5 * time.Minute,
			},
			jobTimeout: time.Hour,
		},
		"timeout absolute": {
			variables: map[string]string{
				"RUNNER_SCRIPT_TIMEOUT": "5m",
			},
			expected: map[string]time.Duration{
				"RUNNER_SCRIPT_TIMEOUT":       5 * time.Minute,
				"RUNNER_AFTER_SCRIPT_TIMEOUT": 5 * time.Minute,
			},
			jobTimeout: time.Hour,
		},
		"timeout last relative": {
			variables: map[string]string{
				"RUNNER_SCRIPT_TIMEOUT":       "5m",
				"RUNNER_AFTER_SCRIPT_TIMEOUT": "-10m",
			},
			expected: map[string]time.Duration{
				"RUNNER_SCRIPT_TIMEOUT":       5 * time.Minute,
				"RUNNER_AFTER_SCRIPT_TIMEOUT": 5 * time.Minute,
			},
			contains:   []string{"Ignoring relative RUNNER_AFTER_SCRIPT_TIMEOUT timeout: -10m"},
			jobTimeout: time.Hour,
		},
		"timeout first relative": {
			variables: map[string]string{
				"RUNNER_SCRIPT_TIMEOUT":       "-5m",
				"RUNNER_AFTER_SCRIPT_TIMEOUT": "10m",
			},
			expected: map[string]time.Duration{
				"RUNNER_SCRIPT_TIMEOUT":       time.Hour,
				"RUNNER_AFTER_SCRIPT_TIMEOUT": 10 * time.Minute,
			},
			contains:   []string{"Ignoring relative RUNNER_SCRIPT_TIMEOUT timeout: -5m"},
			jobTimeout: time.Hour,
		},
		"timeout both relative": {
			variables: map[string]string{
				"RUNNER_SCRIPT_TIMEOUT":       "-15m",
				"RUNNER_AFTER_SCRIPT_TIMEOUT": "-40m",
			},
			expected: map[string]time.Duration{
				"RUNNER_SCRIPT_TIMEOUT":       1 * time.Hour,
				"RUNNER_AFTER_SCRIPT_TIMEOUT": 5 * time.Minute,
			},
			contains: []string{
				"Ignoring relative RUNNER_SCRIPT_TIMEOUT timeout: -15",
				"Ignoring relative RUNNER_AFTER_SCRIPT_TIMEOUT timeout: -40m",
			},
			jobTimeout: time.Hour,
		},
		"timeout relative and exceeds timeout": {
			variables: map[string]string{
				"RUNNER_SCRIPT_TIMEOUT":       "-40m",
				"RUNNER_AFTER_SCRIPT_TIMEOUT": "-40m",
			},
			expected: map[string]time.Duration{
				"RUNNER_SCRIPT_TIMEOUT":       time.Hour,
				"RUNNER_AFTER_SCRIPT_TIMEOUT": 5 * time.Minute,
			},
			contains: []string{
				"Ignoring relative RUNNER_SCRIPT_TIMEOUT timeout: -40m",
				"Ignoring relative RUNNER_AFTER_SCRIPT_TIMEOUT timeout: -40m",
			},
			jobTimeout: time.Hour,
		},
		"timeout relative and exceeds timeout and no time left": {
			variables: map[string]string{
				"RUNNER_SCRIPT_TIMEOUT":       "-40m",
				"RUNNER_AFTER_SCRIPT_TIMEOUT": "-40m",
			},
			expected: map[string]time.Duration{
				"RUNNER_SCRIPT_TIMEOUT":       1 * time.Millisecond,
				"RUNNER_AFTER_SCRIPT_TIMEOUT": 1 * time.Millisecond,
			},
			contains: []string{
				"Ignoring relative RUNNER_SCRIPT_TIMEOUT timeout: -40m",
				"Ignoring relative RUNNER_AFTER_SCRIPT_TIMEOUT timeout: -40m",
			},
			jobTimeout: time.Millisecond,
		},
		"timeout is invalid": {
			variables: map[string]string{
				"RUNNER_SCRIPT_TIMEOUT": "foobar",
			},
			expected: map[string]time.Duration{
				"RUNNER_SCRIPT_TIMEOUT":       0,
				"RUNNER_AFTER_SCRIPT_TIMEOUT": time.Millisecond,
			},
			contains:   []string{"Ignoring malformed RUNNER_SCRIPT_TIMEOUT timeout: foobar"},
			jobTimeout: time.Millisecond,
		},
		"timeout when no parent timeout": {
			variables: map[string]string{
				"RUNNER_SCRIPT_TIMEOUT": "-10m",
			},
			expected: map[string]time.Duration{
				"RUNNER_SCRIPT_TIMEOUT":       0,
				"RUNNER_AFTER_SCRIPT_TIMEOUT": 5 * time.Minute,
			},
			contains: []string{"Ignoring relative RUNNER_SCRIPT_TIMEOUT timeout: -10m"},
		},
		"script timeout longer than job timeout": {
			variables: map[string]string{
				"RUNNER_SCRIPT_TIMEOUT": "60m",
			},
			expected: map[string]time.Duration{
				"RUNNER_SCRIPT_TIMEOUT":       40 * time.Minute,
				"RUNNER_AFTER_SCRIPT_TIMEOUT": 5 * time.Minute,
			},
			contains:   []string{"RUNNER_SCRIPT_TIMEOUT timeout: 60m is longer than job timeout."},
			jobTimeout: 40 * time.Minute,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			logs := bytes.Buffer{}
			lentry := logrus.New()
			lentry.Out = &logs
			logger := buildlogger.New(nil, logrus.NewEntry(lentry), buildlogger.Options{})

			b := &Build{
				Runner: &RunnerConfig{},
				logger: logger,
			}
			for key, val := range tc.variables {
				b.Variables = append(b.Variables, JobVariable{
					Key:   key,
					Value: val,
				})
			}

			ctx := context.Background()
			if tc.jobTimeout > 0 {
				var cancel func()
				ctx, cancel = context.WithTimeout(ctx, tc.jobTimeout)
				defer cancel()
			}

			for key, ctxProvider := range b.getStageTimeoutContexts(ctx, defaultTimeouts...) {
				ctx, cancel := ctxProvider()
				defer cancel()

				deadline, _ := ctx.Deadline()
				if !deadline.IsZero() {
					assert.WithinDuration(t, time.Now().Add(tc.expected[key]), deadline, time.Second, key)
				}
			}

			if len(tc.contains) == 0 {
				assert.Empty(t, logs.String())
			} else {
				for i := range tc.contains {
					assert.Contains(t, logs.String(), tc.contains[i])
				}
			}
		})
	}
}

func Test_logUsedImages(t *testing.T) {
	const (
		testImage1 = "test_image:latest"
		testImage2 = "service_image:v1.0"
		testImage3 = "registry.gitlab.example.com/my/project/image@sha256:123456"

		testPlatform = "platform"
	)

	tests := map[string]struct {
		featureOn    bool
		image        Image
		services     Services
		assertImages func(t *testing.T, images []string, platforms []string)
	}{
		"FF disabled": {
			featureOn: false,
			image:     Image{Name: testImage1},
			services: Services{
				{Name: testImage2},
				{Name: testImage3},
			},
			assertImages: func(t *testing.T, images []string, _ []string) {
				assert.Empty(t, images)
			},
		},
		"no images defined": {
			featureOn: true,
			assertImages: func(t *testing.T, images []string, _ []string) {
				assert.Empty(t, images)
			},
		},
		"job image defined": {
			featureOn: true,
			image: Image{
				Name: testImage1,
				ExecutorOptions: ImageExecutorOptions{
					Docker: ImageDockerOptions{
						Platform: testPlatform,
					},
				},
			},
			assertImages: func(t *testing.T, images []string, platforms []string) {
				assert.Len(t, images, 1)
				assert.Contains(t, images, testImage1)

				assert.Len(t, platforms, 1)
				assert.Contains(t, platforms, testPlatform)
			},
		},
		"service images defined": {
			featureOn: true,
			services: Services{
				{Name: testImage1},
				{
					Name: testImage2,
					ExecutorOptions: ImageExecutorOptions{
						Docker: ImageDockerOptions{
							Platform: testPlatform,
						},
					},
				},
			},
			assertImages: func(t *testing.T, images []string, platforms []string) {
				assert.Len(t, images, 2)
				assert.Contains(t, images, testImage1)
				assert.Contains(t, images, testImage2)

				assert.Len(t, platforms, 1)
				assert.Contains(t, platforms, testPlatform)
			},
		},
		"all images defined": {
			featureOn: true,
			image:     Image{Name: testImage1},
			services: Services{
				{Name: testImage2},
				{Name: testImage3},
			},
			assertImages: func(t *testing.T, images []string, _ []string) {
				assert.Len(t, images, 3)
				assert.Contains(t, images, testImage1)
				assert.Contains(t, images, testImage2)
				assert.Contains(t, images, testImage3)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			logger, hook := test.NewNullLogger()

			b := &Build{
				Runner: &RunnerConfig{
					RunnerSettings: RunnerSettings{
						FeatureFlags: map[string]bool{
							featureflags.LogImagesConfiguredForJob: tt.featureOn,
						},
					},
					RunnerCredentials: RunnerCredentials{
						Logger: logger,
					},
				},
				JobResponse: JobResponse{
					Image:    tt.image,
					Services: tt.services,
				},
			}

			b.logUsedImages()

			var images []string
			var platforms []string
			for _, entry := range hook.AllEntries() {
				image, ok := entry.Data["image_name"]
				if !ok {
					continue
				}
				images = append(images, image.(string))

				platform, ok := entry.Data["image_platform"]
				if !ok {
					continue
				}
				platforms = append(platforms, platform.(string))
			}

			tt.assertImages(t, images, platforms)
		})
	}
}

func TestBuildStageMetrics(t *testing.T) {
	p := setupSuccessfulMockExecutor(t, func(options ExecutorPrepareOptions) error { return nil })

	rc := &RunnerConfig{}
	build := registerExecutorWithSuccessfulBuild(t, p, rc)
	build.Runner.Environment = append(build.Runner.Environment, fmt.Sprintf("%s=true", featureflags.ExportHighCardinalityMetrics))

	// each expected build stage should be called twice, for start and for end
	stagesMap := make(map[BuildStage]int)

	stageFn := func(stage BuildStage) {
		stagesMap[stage]++
	}

	build.OnBuildStageStartFn = stageFn
	build.OnBuildStageEndFn = stageFn

	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)

	expectedStages := []BuildStage{
		BuildStagePrepare, BuildStagePrepareExecutor, BuildStageRestoreCache, BuildStageUploadOnSuccessArtifacts,
		BuildStageGetSources, BuildStageDownloadArtifacts, BuildStageCleanup, BuildStageAfterScript, BuildStageArchiveOnSuccessCache,
		BuildStage("step_script"),
	}

	for _, s := range expectedStages {
		assert.Equal(t, stagesMap[s], 2)
		delete(stagesMap, s)
	}

	assert.Len(t, stagesMap, 0)
}

func TestBuildStageMetricsFailBuild(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider(t)
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	executor.On("Cleanup").Once()

	// Set up a failing a build script
	thrownErr := &BuildError{Inner: errors.New("test error"), ExitCode: 1}
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	executor.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	executor.On("Run", mock.Anything).Return(thrownErr).Times(3)
	executor.On("Run", matchBuildStage(BuildStageCleanup)).Return(nil).Once()
	executor.On("Finish", thrownErr).Once()

	RegisterExecutorProviderForTest(t, t.Name(), provider)

	failedBuild, err := GetFailedBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: failedBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: t.Name(),
			},
		},
	}

	build.Runner.Environment = append(build.Runner.Environment, fmt.Sprintf("%s=true", featureflags.ExportHighCardinalityMetrics))

	// each expected build stage should be called twice, for start and for end
	stagesMap := make(map[BuildStage]int)

	stageFn := func(stage BuildStage) {
		stagesMap[stage]++
	}

	build.OnBuildStageStartFn = stageFn
	build.OnBuildStageEndFn = stageFn

	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	expectedErr := new(BuildError)
	assert.ErrorIs(t, err, expectedErr)

	expectedStages := []BuildStage{
		BuildStageArchiveOnFailureCache, BuildStageCleanup, BuildStageGetSources, BuildStagePrepare,
		BuildStagePrepareExecutor, BuildStageUploadOnFailureArtifacts,
	}

	for _, s := range expectedStages {
		assert.Equal(t, stagesMap[s], 2)
		delete(stagesMap, s)
	}

	assert.Len(t, stagesMap, 0)
}

func TestBuildDurationsAndBoundaryTimes(t *testing.T) {
	rc := new(RunnerConfig)
	rc.RunnerSettings.Executor = t.Name()

	build, err := NewBuild(JobResponse{}, rc, nil, nil)
	require.NoError(t, err)

	startedAt1 := build.StartedAt()

	assert.False(t, startedAt1.IsZero(), "StartedAt should not be a zero-time")
	assert.True(t, build.FinishedAt().IsZero(), "FinishedAt should be a zero-time")

	time.Sleep(10 * time.Millisecond)
	currentDuration1 := build.CurrentDuration()
	assert.True(t, currentDuration1 >= 10*time.Millisecond, "Current job duration should be greater tha 10ms")

	time.Sleep(10 * time.Millisecond)
	currentDuration2 := build.CurrentDuration()
	assert.True(t, currentDuration2 >= 20*time.Millisecond, "Current job duration should be greater tha 20ms")
	assert.NotEqual(t, currentDuration1, currentDuration2, "Subsequent CurrentDuration() values shouldn't be equal")

	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, time.Duration(0), build.FinalDuration(), "If ensureFinishedAt() wasn't called, final duration should be equal to 0")

	// Mark the job as finished!
	build.ensureFinishedAt()

	finalDuration1 := build.FinalDuration()
	finishedAt1 := build.FinishedAt()
	assert.True(t, finalDuration1 >= 30*time.Millisecond, "Final duration should be greater than 30ms")
	assert.False(t, finishedAt1.IsZero(), "FinishedAt should not be a zero-time")

	time.Sleep(10 * time.Millisecond)
	startedAt2 := build.StartedAt()
	finishedAt2 := build.FinishedAt()
	finalDuration2 := build.FinalDuration()

	assert.Equal(t, finalDuration1, finalDuration2, "Subsequent FinalDuration() values should be equal")
	assert.Equal(t, finishedAt1, finishedAt2, "FinishedAt() should not change")
	assert.Equal(t, startedAt1, startedAt2, "StartedAt() should not change")
}

func TestBuild_RunCallsEnsureFinishedAt(t *testing.T) {
	tests := map[string]struct {
		executorRunError error
		assertError      func(t *testing.T, err error)
	}{
		"succeeded job": {
			executorRunError: nil,
		},
		"failed job": {
			executorRunError: assert.AnError,
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			executor := NewMockExecutor(t)
			executor.EXPECT().Prepare(mock.Anything).Return(nil)
			executor.EXPECT().Run(mock.Anything).Return(tt.executorRunError)
			executor.EXPECT().Shell().Return(&ShellScriptInfo{Shell: "script-shell"}).Maybe()
			executor.EXPECT().Finish(mock.Anything)
			executor.EXPECT().Cleanup()

			ep := NewMockExecutorProvider(t)
			ep.EXPECT().GetDefaultShell().Return("bash")
			ep.EXPECT().CanCreate().Return(true)
			ep.EXPECT().GetFeatures(mock.Anything).Return(nil)
			ep.EXPECT().Create().Return(executor)

			RegisterExecutorProviderForTest(t, t.Name(), ep)

			rc := new(RunnerConfig)
			rc.RunnerSettings.Executor = t.Name()

			interrupt := make(chan os.Signal, 1)

			build, err := NewBuild(JobResponse{}, rc, interrupt, nil)
			require.NoError(t, err)

			// Some of the job execution steps use the configurable number of attempts
			// before they report failure. That includes, for example, the predefined
			// get_sources step.
			// For these steps, the loop that handles subsequent attempts may use
			// the exponential backoff delay, when the FF is set to true, which is true.
			// That is done, unfortunately, even when there is only one attempt to be
			// executed.
			// As the tests here are returning error early (which includes also context
			// cancel caused by simulating job cancel or runner process interrupt), this
			// backoff causes an additional 5 seconds delay, that we don't need here.
			// By disabling the feature flag, we speed up the tests.
			build.initSettings()
			build.buildSettings.FeatureFlags[featureflags.UseExponentialBackoffStageRetry] = false

			require.Zero(t, build.finishedAt)

			trace := NewMockJobTrace(t)
			trace.EXPECT().SetAbortFunc(mock.Anything)
			trace.EXPECT().SetCancelFunc(mock.AnythingOfType("context.CancelFunc")).Maybe()
			trace.EXPECT().IsStdout().Return(true)
			trace.EXPECT().Write(mock.Anything).Return(0, nil)
			trace.EXPECT().Fail(mock.Anything, mock.Anything).Return(nil).Maybe()
			trace.EXPECT().Success().Return(nil).Maybe()
			trace.EXPECT().SetSupportedFailureReasonMapper(mock.Anything).Maybe()

			err = build.Run(&Config{}, trace)
			if tt.assertError != nil {
				tt.assertError(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.NotZero(t, build.finishedAt)
		})
	}
}

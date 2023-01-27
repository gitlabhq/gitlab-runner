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

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/session"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

func init() {
	s := MockShell{}
	s.On("GetName").Return("script-shell")
	s.On("GenerateScript", mock.Anything, mock.Anything).Return("script", nil)
	RegisterShell(&s)
}

func TestBuildPredefinedVariables(t *testing.T) {
	for _, rootDir := range []string{"/root/dir1", "/root/dir2"} {
		t.Run(rootDir, func(t *testing.T) {
			build := runSuccessfulMockBuild(t, func(options ExecutorPrepareOptions) error {
				return options.Build.StartBuild(rootDir, "/cache/dir", false, false)
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
				return options.Build.StartBuild("/root/dir", "/cache/dir", false, false)
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
			executor, provider := setupMockExecutorAndProvider()
			defer executor.AssertExpectations(t)
			defer provider.AssertExpectations(t)

			tt.setupMockExecutor(executor)

			RegisterExecutorProvider(t.Name(), provider)

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
				return options.Build.StartBuild("/root/dir", "/cache/dir", false, false)
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
	p, assertFn := setupSuccessfulMockExecutor(t, func(options ExecutorPrepareOptions) error {
		options.Config.Docker.Credentials.Host = "10.0.0.2"
		return nil
	})
	defer assertFn()

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

	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(&e).Times(3)

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

	build := registerExecutorWithSuccessfulBuild(t, &p, new(RunnerConfig))
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestPrepareFailure(t *testing.T) {
	PreparationRetryInterval = 0

	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(&e).Times(3)

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("prepare failed")).Times(3)
	e.On("Cleanup").Times(3)

	build := registerExecutorWithSuccessfulBuild(t, &p, new(RunnerConfig))
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "prepare failed")
}

func TestPrepareFailureOnBuildError(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider()
	defer executor.AssertExpectations(t)
	defer provider.AssertExpectations(t)
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

	e := new(MockExecutor)
	defer e.AssertExpectations(t)

	p := new(MockExecutorProvider)
	defer p.AssertExpectations(t)

	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()
	p.On("Create").Return(e).Once()

	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	e.On("Cleanup").Once()
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(testErr).Once()
	e.On("Finish", mock.Anything).Once()

	RegisterExecutorProvider("build-run-prepare-environment-failure-on-build-error", p)

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
	executor, provider := setupMockExecutorAndProvider()
	defer executor.AssertExpectations(t)
	defer provider.AssertExpectations(t)
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

	RegisterExecutorProvider("build-run-job-failure", provider)

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

	trace := new(MockJobTrace)
	defer trace.AssertExpectations(t)
	trace.On("Write", mock.Anything).Return(0, nil)
	trace.On("IsStdout").Return(true)
	trace.On("SetCancelFunc", mock.Anything).Once()
	trace.On("SetAbortFunc", mock.Anything).Once()
	trace.On("SetMasked", mock.Anything).Once()
	trace.On("Fail", thrownErr, JobFailureData{Reason: ScriptFailure, ExitCode: 1}).Once()

	err = build.Run(&Config{}, trace)

	expectedErr := new(BuildError)
	assert.ErrorIs(t, err, expectedErr)
}

func TestJobFailureOnExecutionTimeout(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider()
	defer executor.AssertExpectations(t)
	defer provider.AssertExpectations(t)

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

	trace := new(MockJobTrace)
	defer trace.AssertExpectations(t)
	trace.On("Write", mock.Anything).Return(0, nil)
	trace.On("IsStdout").Return(true)
	trace.On("SetCancelFunc", mock.Anything).Once()
	trace.On("SetAbortFunc", mock.Anything).Once()
	trace.On("SetMasked", mock.Anything).Once()
	trace.On("Fail", mock.Anything, JobFailureData{Reason: JobExecutionTimeout}).Run(func(arguments mock.Arguments) {
		assert.Error(t, arguments.Get(0).(error))
	}).Once()

	err := build.Run(&Config{}, trace)

	expectedErr := &BuildError{FailureReason: JobExecutionTimeout}
	assert.ErrorIs(t, err, expectedErr)
}

func TestRunFailureRunsAfterScriptAndArtifactsOnFailure(t *testing.T) {
	executor, provider := setupMockExecutorAndProvider()
	defer executor.AssertExpectations(t)
	defer provider.AssertExpectations(t)
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

	RegisterExecutorProvider("build-run-run-failure", provider)

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
	executor, provider := setupMockExecutorAndProvider()
	defer executor.AssertExpectations(t)
	defer provider.AssertExpectations(t)
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	executor.On("Cleanup").Once()

	// Fail a build script
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	executor.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	executor.On("Run", matchBuildStage(BuildStageGetSources)).Return(errors.New("build fail")).Times(3)
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
	executor, provider := setupMockExecutorAndProvider()
	defer executor.AssertExpectations(t)
	defer provider.AssertExpectations(t)
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
	executor, provider := setupMockExecutorAndProvider()
	defer executor.AssertExpectations(t)
	defer provider.AssertExpectations(t)
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
	executor, provider := setupMockExecutorAndProvider()
	defer executor.AssertExpectations(t)
	defer provider.AssertExpectations(t)
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
	executor, provider := setupMockExecutorAndProvider()
	defer executor.AssertExpectations(t)
	defer provider.AssertExpectations(t)
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
	executor, provider := setupMockExecutorAndProvider()
	defer executor.AssertExpectations(t)
	defer provider.AssertExpectations(t)
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
	executor, provider := setupMockExecutorAndProvider()
	defer provider.AssertExpectations(t)
	defer executor.AssertExpectations(t)
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
	executor, provider := setupMockExecutorAndProvider()
	defer provider.AssertExpectations(t)

	// We run everything once
	executor.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	executor.On("Finish", mock.Anything).Twice()
	executor.On("Cleanup").Twice()

	// Run script successfully
	executor.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})

	executor.On("Run", mock.Anything).Return(nil)
	executor.On("Run", mock.Anything).Return(errors.New("build fail")).Once()
	executor.On("Run", mock.Anything).Return(nil)

	build := registerExecutorWithSuccessfulBuild(t, provider, new(RunnerConfig))
	build.Variables = append(build.Variables, JobVariable{Key: "GET_SOURCES_ATTEMPTS", Value: "3"})
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)
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
			expectedLogOutput:         "CI_DEBUG_TRACE usage is disabled on this Runner",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			logger, hooks := test.NewNullLogger()

			build := &Build{
				logger: NewBuildLogger(nil, logrus.NewEntry(logger)),
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
				output, err := hooks.LastEntry().String()
				require.NoError(t, err)
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
		"Windows-style BuildDir (CMD or PS)": {
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
	)

	testCases := map[string]struct {
		runner                    RunnerSettings
		jobTokenVariableOverwrite string
		expectedURL               string
	}{
		"using clone_url with http protocol": {
			runner: RunnerSettings{
				CloneURL: "http://test.local/",
			},
			expectedURL: "http://gitlab-ci-token:job-token@test.local/my/project.git",
		},
		"using clone_url with https protocol": {
			runner: RunnerSettings{
				CloneURL: "https://test.local",
			},
			expectedURL: "https://gitlab-ci-token:job-token@test.local/my/project.git",
		},
		"using clone_url with relative URL": {
			runner: RunnerSettings{
				CloneURL: "https://test.local/gitlab",
			},
			expectedURL: "https://gitlab-ci-token:job-token@test.local/gitlab/my/project.git",
		},
		"using clone_url with relative URL with trailing slash": {
			runner: RunnerSettings{
				CloneURL: "https://test.local/gitlab/",
			},
			expectedURL: "https://gitlab-ci-token:job-token@test.local/gitlab/my/project.git",
		},
		"using clone_url with ssh protocol": {
			runner: RunnerSettings{
				CloneURL: "ssh://git@test.local/",
			},
			expectedURL: "ssh://git@test.local/my/project.git",
		},
		"using clone_url with ssh protocol and default username": {
			runner: RunnerSettings{
				CloneURL: "ssh://test.local/",
			},
			expectedURL: "ssh://git@test.local/my/project.git",
		},
		"not using clone_url": {
			runner:      RunnerSettings{},
			expectedURL: exampleRepoURL,
		},
		"overwriting job token with variable and clone_url": {
			runner: RunnerSettings{
				CloneURL: "https://test.local",
			},
			jobTokenVariableOverwrite: "wrong-token",
			expectedURL:               "https://gitlab-ci-token:job-token@test.local/my/project.git",
		},
		"overwriting job token with variable and no clone_url": {
			runner:                    RunnerSettings{},
			jobTokenVariableOverwrite: "wrong-token",
			expectedURL:               exampleRepoURL,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			build := &Build{
				Runner: &RunnerConfig{
					RunnerSettings: tc.runner,
				},
				allVariables: JobVariables{
					JobVariable{Key: "CI_PROJECT_PATH", Value: exampleProjectPath},
				},
				JobResponse: JobResponse{
					Token: exampleJobToken,
					GitInfo: GitInfo{
						RepoURL: exampleRepoURL,
					},
				},
			}

			if tc.jobTokenVariableOverwrite != "" {
				build.allVariables = append(build.allVariables, JobVariable{
					Key:   "CI_JOB_TOKEN",
					Value: tc.jobTokenVariableOverwrite,
				})
			}

			assert.Equal(t, tc.expectedURL, build.GetRemoteURL())
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

func TestStartBuild(t *testing.T) {
	type startBuildArgs struct {
		rootDir               string
		cacheDir              string
		customBuildDirEnabled bool
		sharedDir             bool
	}

	tests := map[string]struct {
		args             startBuildArgs
		jobVariables     JobVariables
		expectedBuildDir string
		expectedCacheDir string
		expectedError    bool
	}{
		"no job specific build dir with no shared dir": {
			args: startBuildArgs{
				rootDir:               "/build",
				cacheDir:              "/cache",
				customBuildDirEnabled: true,
				sharedDir:             false,
			},
			jobVariables:     JobVariables{},
			expectedBuildDir: "/build/test-namespace/test-repo",
			expectedCacheDir: "/cache/test-namespace/test-repo",
			expectedError:    false,
		},
		"no job specified build dir with shared dir": {
			args: startBuildArgs{
				rootDir:               "/builds",
				cacheDir:              "/cache",
				customBuildDirEnabled: true,
				sharedDir:             true,
			},
			jobVariables:     JobVariables{},
			expectedBuildDir: "/builds/1234/0/test-namespace/test-repo",
			expectedCacheDir: "/cache/test-namespace/test-repo",
			expectedError:    false,
		},
		"valid GIT_CLONE_PATH was specified": {
			args: startBuildArgs{
				rootDir:               "/builds",
				cacheDir:              "/cache",
				customBuildDirEnabled: true,
				sharedDir:             false,
			},
			jobVariables: JobVariables{
				{Key: "GIT_CLONE_PATH", Value: "/builds/go/src/gitlab.com/test-namespace/test-repo", Public: true},
			},
			expectedBuildDir: "/builds/go/src/gitlab.com/test-namespace/test-repo",
			expectedCacheDir: "/cache/test-namespace/test-repo",
			expectedError:    false,
		},
		"valid GIT_CLONE_PATH using CI_BUILDS_DIR was specified": {
			args: startBuildArgs{
				rootDir:               "/builds",
				cacheDir:              "/cache",
				customBuildDirEnabled: true,
				sharedDir:             false,
			},
			jobVariables: JobVariables{
				{
					Key:    "GIT_CLONE_PATH",
					Value:  "$CI_BUILDS_DIR/go/src/gitlab.com/test-namespace/test-repo",
					Public: true,
				},
			},
			expectedBuildDir: "/builds/go/src/gitlab.com/test-namespace/test-repo",
			expectedCacheDir: "/cache/test-namespace/test-repo",
			expectedError:    false,
		},
		"out-of-bounds GIT_CLONE_PATH was specified": {
			args: startBuildArgs{
				rootDir:               "/builds",
				cacheDir:              "/cache",
				customBuildDirEnabled: true,
				sharedDir:             false,
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
			},
			jobVariables: JobVariables{
				{Key: "GIT_CLONE_PATH", Value: "/builds/go/src/gitlab.com/test-namespace/test-repo", Public: true},
			},
			expectedBuildDir: "/builds/test-namespace/test-repo",
			expectedCacheDir: "/cache/test-namespace/test-repo",
			expectedError:    true,
		},
		"invalid GIT_CLONE_PATH was specified": {
			args: startBuildArgs{
				rootDir:               "/builds",
				cacheDir:              "/cache",
				customBuildDirEnabled: true,
				sharedDir:             false,
			},
			jobVariables: JobVariables{
				{Key: "GIT_CLONE_PATH", Value: "/go/src/gitlab.com/test-namespace/test-repo", Public: true},
			},
			expectedError: true,
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
			)
			if test.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.expectedBuildDir, build.BuildDir)
			assert.Equal(t, test.args.rootDir, build.RootDir)
			assert.Equal(t, test.expectedCacheDir, build.CacheDir)
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

	s := new(MockShell)
	defer s.AssertExpectations(t)

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

			e := &MockExecutor{}
			defer e.AssertExpectations(t)

			s.On("GenerateScript", mock.Anything, mock.Anything).Return("script", ErrSkipBuildStage)
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
			build.logger = NewBuildLogger(&trace, build.Log())
			sess, err := session.NewSession(nil)
			require.NoError(t, err)
			build.Session = sess

			srv := httptest.NewServer(build.Session.Handler())
			defer srv.Close()

			mockConn := terminal.MockConn{}
			defer mockConn.AssertExpectations(t)
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

			mockTerminal := terminal.MockInteractiveTerminal{}
			defer mockTerminal.AssertExpectations(t)
			mockTerminal.On("Connect").Return(&mockConn, nil)
			sess.SetInteractiveTerminal(&mockTerminal)

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
			expectedResult: []string{},
			expectedError:  false,
		},
		"empty": {
			isVariableSet:  true,
			value:          "",
			expectedResult: []string{},
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

func TestGitSubmoduleUpdateFlags(t *testing.T) {
	tests := map[string]struct {
		value          string
		expectedResult []string
	}{
		"empty update flags": {
			value:          "",
			expectedResult: []string{},
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

			err := build.StartBuild(test.rootDir, "/cache", true, false)
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
				logger: NewBuildLogger(nil, logrus.NewEntry(logger)),
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
			expectedName: "runner-zen8e6e-project-1234567890-concurrent-0",
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
			expectedName: "runner-xyzwabc--project-1234567890-concurrent-0",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expectedName, test.build.ProjectUniqueName())
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
		expectedErr      error
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
			expectedAttempts: 0,
			expectedErr:      &invalidAttemptError{},
		},
		{
			attempts:         "99",
			expectedAttempts: 0,
			expectedErr:      &invalidAttemptError{},
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

			attempts, err := build.GetExecutorJobSectionAttempts()
			assert.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedAttempts, attempts)
		})
	}
}

func TestBuild_getFeatureFlagInfo(t *testing.T) {
	const changedFeatureFlags = "FF_CMD_DISABLE_DELAYED_ERROR_LEVEL_EXPANSION:true"
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
							Key:    featureflags.CmdDisableDelayedErrorLevelExpansion,
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
) (*MockExecutorProvider, func()) {
	executor, provider := setupMockExecutorAndProvider()
	assertFn := func() {
		executor.AssertExpectations(t)
		provider.AssertExpectations(t)
	}

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

	return provider, assertFn
}

func setupMockExecutorAndProvider() (*MockExecutor, *MockExecutorProvider) {
	e := new(MockExecutor)
	p := new(MockExecutorProvider)

	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()
	p.On("Create").Return(e).Once()

	return e, p
}

func registerExecutorWithSuccessfulBuild(t *testing.T, p *MockExecutorProvider, rc *RunnerConfig) *Build {
	require.NotNil(t, rc)

	RegisterExecutorProvider(t.Name(), p)

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
	p, assertFn := setupSuccessfulMockExecutor(t, prepareFn)
	defer assertFn()

	build := registerExecutorWithSuccessfulBuild(t, p, new(RunnerConfig))
	err := build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)

	return build
}

func TestSecretsResolving(t *testing.T) {
	exampleVariables := JobVariables{
		{Key: "key", Value: "value"},
	}

	setupFailureExecutorMocks := func(t *testing.T) (*MockExecutorProvider, func()) {
		e := new(MockExecutor)
		p := new(MockExecutorProvider)

		p.On("CanCreate").Return(true).Once()
		p.On("GetDefaultShell").Return("bash").Once()
		p.On("GetFeatures", mock.Anything).Return(nil).Once()

		assertFn := func() {
			e.AssertExpectations(t)
			p.AssertExpectations(t)
		}

		return p, assertFn
	}

	secrets := Secrets{
		"TEST_SECRET": Secret{
			Vault: &VaultSecret{},
		},
	}

	tests := map[string]struct {
		secrets                 Secrets
		resolverCreationError   error
		prepareExecutorProvider func(t *testing.T) (*MockExecutorProvider, func())
		returnVariables         JobVariables
		resolvingError          error
		expectedVariables       JobVariables
		expectedError           error
	}{
		"secrets not present": {
			prepareExecutorProvider: func(t *testing.T) (*MockExecutorProvider, func()) {
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
			prepareExecutorProvider: func(t *testing.T) (*MockExecutorProvider, func()) {
				return setupSuccessfulMockExecutor(t, func(options ExecutorPrepareOptions) error { return nil })
			},
			returnVariables:   exampleVariables,
			resolvingError:    nil,
			expectedVariables: exampleVariables,
			expectedError:     nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			secretsResolverMock := new(MockSecretsResolver)
			defer secretsResolverMock.AssertExpectations(t)

			p, assertFn := tt.prepareExecutorProvider(t)
			defer assertFn()

			RegisterExecutorProvider(t.Name(), p)

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

			build.secretsResolver = func(_ logger, _ SecretResolverRegistry) (SecretsResolver, error) {
				return secretsResolverMock, tt.resolverCreationError
			}

			err = build.Run(&Config{}, &Trace{Writer: os.Stdout})

			assert.Equal(t, tt.expectedVariables, build.secretsVariables)

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestResolvedSecretsSetMasked(t *testing.T) {
	const expectedMaskPhrase = "resolved$value"

	p, assertFn := setupSuccessfulMockExecutor(t, func(options ExecutorPrepareOptions) error {
		return nil
	})
	defer assertFn()

	RegisterExecutorProvider(t.Name(), p)

	rc := new(RunnerConfig)
	rc.RunnerSettings.Executor = t.Name()

	successfulBuild, err := GetSuccessfulBuild()
	require.NoError(t, err)

	successfulBuild.Secrets = Secrets{
		"TEST_SECRET": Secret{
			Vault: &VaultSecret{},
		},
	}

	build, err := NewBuild(successfulBuild, rc, nil, nil)
	assert.NoError(t, err)

	secretsResolverMock := new(MockSecretsResolver)
	defer secretsResolverMock.AssertExpectations(t)

	secretsResolverMock.On("Resolve", successfulBuild.Secrets).Return(JobVariables{
		{Key: "key", Value: expectedMaskPhrase, Masked: true, Raw: true},
	}, nil).Once()

	build.secretsResolver = func(_ logger, _ SecretResolverRegistry) (SecretsResolver, error) {
		return secretsResolverMock, nil
	}

	trace := new(MockJobTrace)
	defer trace.AssertExpectations(t)
	trace.On("Write", mock.Anything).Return(0, nil)
	trace.On("IsStdout").Return(true)
	trace.On("SetCancelFunc", mock.Anything).Once()
	trace.On("SetAbortFunc", mock.Anything).Once()
	trace.On("Success").Once()

	// ensure that variables returned from the secrets
	// resolver get passed to SetMasked
	trace.On("SetMasked", MaskOptions{Phrases: []string{expectedMaskPhrase}}).Once()

	err = build.Run(&Config{}, trace)
	assert.NoError(t, err)
}

func TestBuildSupportedFailureReasons(t *testing.T) {
	supportedReason := JobFailureReason("supported")
	unsupportedReason := JobFailureReason("unsupported")

	tests := map[string]struct {
		supported      []JobFailureReason
		reason         JobFailureReason
		expectedReason JobFailureReason
	}{
		"empty list with widely supported reason": {
			supported:      nil,
			reason:         ScriptFailure,
			expectedReason: ScriptFailure,
		},
		"empty list with unsupported reason": {
			supported:      nil,
			reason:         unsupportedReason,
			expectedReason: UnknownFailure,
		},
		"populated list with widely supported reason": {
			supported:      []JobFailureReason{supportedReason},
			reason:         ScriptFailure,
			expectedReason: ScriptFailure,
		},
		"populated list with supported reason": {
			supported:      []JobFailureReason{supportedReason},
			reason:         supportedReason,
			expectedReason: supportedReason,
		},
		"populated list with unsupported reason": {
			supported:      []JobFailureReason{supportedReason},
			reason:         unsupportedReason,
			expectedReason: UnknownFailure,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			b := &Build{
				Runner: &RunnerConfig{},
				JobResponse: JobResponse{
					Features: GitlabFeatures{
						FailureReasons: tc.supported,
					},
				},
			}
			b.logger = NewBuildLogger(nil, b.Log())

			err := &BuildError{
				FailureReason: tc.reason,
			}

			trace := new(MockJobTrace)
			defer trace.AssertExpectations(t)
			trace.On(
				"Fail",
				err,
				JobFailureData{
					Reason:   tc.expectedReason,
					ExitCode: 0,
				},
			).Once()

			b.setTraceStatus(trace, err)
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
				mt.On("Success").Once()
			},
		},
		"build error, script failure": {
			err: &BuildError{FailureReason: ScriptFailure},
			assert: func(t *testing.T, mt *MockJobTrace, err error) {
				mt.On("Fail", err, JobFailureData{Reason: ScriptFailure}).Once()
			},
		},
		"build error, wrapped script failure": {
			err: fmt.Errorf("wrapped: %w", &BuildError{FailureReason: ScriptFailure}),
			assert: func(t *testing.T, mt *MockJobTrace, err error) {
				mt.On("Fail", err, JobFailureData{Reason: ScriptFailure}).Once()
			},
		},
		"non-build error": {
			err: fmt.Errorf("some error"),
			assert: func(t *testing.T, mt *MockJobTrace, err error) {
				mt.On("Fail", err, JobFailureData{Reason: RunnerSystemFailure}).Once()
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			b := &Build{
				Runner: &RunnerConfig{},
			}
			b.logger = NewBuildLogger(nil, b.Log())

			trace := new(MockJobTrace)
			defer trace.AssertExpectations(t)

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
			wantLog:  "failed to parse value 'blammo' for CI_DEBUG_SERVICES variable:",
		},
		"enabled": {
			variable: JobVariable{Key: "CI_DEBUG_SERVICES", Value: "true", Public: true},
			want:     true,
		},
	}

	logs := bytes.Buffer{}
	lentry := logrus.New()
	lentry.Out = &logs
	logger := NewBuildLogger(nil, logrus.NewEntry(lentry))

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			logs.Reset()

			b := &Build{
				Runner:      &RunnerConfig{},
				logger:      logger,
				JobResponse: JobResponse{Variables: []JobVariable{tt.variable}},
			}

			got := b.IsCIDebugServiceEnabled()

			assert.Equal(t, tt.want, got)

			if tt.wantLog == "" {
				assert.Empty(t, logs.String())
			} else {
				assert.Contains(t, logs.String(), tt.wantLog)
			}
		})
	}
}

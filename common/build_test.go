package common

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/session"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

func init() {
	s := MockShell{}
	s.On("GetName").Return("script-shell")
	s.On("GenerateScript", mock.Anything, mock.Anything).Return("script", nil)
	RegisterShell(&s)
}

func TestBuildRun(t *testing.T) {
	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor only once
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(&e).Once()

	// We run everything once
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	e.On("Finish", nil).Once()
	e.On("Cleanup").Once()

	// Run script successfully
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageUserScript)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageAfterScript)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageArchiveCache)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageUploadOnSuccessArtifacts)).Return(nil).Once()

	RegisterExecutorProvider("build-run-test", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-test",
			},
		},
	}
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestBuildPredefinedVariables(t *testing.T) {
	for _, rootDir := range []string{"/root/dir1", "/root/dir2"} {
		t.Run(rootDir, func(t *testing.T) {
			build := runSuccessfulMockBuild(t, func(options ExecutorPrepareOptions) error {
				return options.Build.StartBuild(rootDir, "/cache/dir", false, false)
			})

			projectDir := build.GetAllVariables().Get("CI_PROJECT_DIR")
			assert.NotEmpty(t, projectDir, "should have CI_PROJECT_DIR")
		})
	}
}

func runSuccessfulMockBuild(t *testing.T, prepareFn func(options ExecutorPrepareOptions) error) *Build {
	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor only once
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(&e).Once()

	// We run everything once
	e.On("Prepare", mock.Anything).Return(prepareFn).Once()
	e.On("Finish", nil).Once()
	e.On("Cleanup").Once()

	// Run script successfully
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageUserScript)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageAfterScript)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageArchiveCache)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageUploadOnSuccessArtifacts)).Return(nil).Once()

	RegisterExecutorProvider(t.Name(), &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: t.Name(),
			},
		},
	}
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)

	return build
}

func TestJobImageExposed(t *testing.T) {
	tests := map[string]struct {
		image           string
		vars            []JobVariable
		expectVarExists bool
		expectImageName string
	}{
		"normal image exposed": {
			image:           "alpine:3.11",
			expectVarExists: true,
			expectImageName: "alpine:3.11",
		},
		"image with variable expansion": {
			image:           "${IMAGE}:3.11",
			vars:            []JobVariable{{Key: "IMAGE", Value: "alpine", Public: true}},
			expectVarExists: true,
			expectImageName: "alpine:3.11",
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
				actualJobImage := build.GetAllVariables().Get("CI_JOB_IMAGE")
				assert.Equal(t, tt.expectImageName, actualJobImage)
			}
		})
	}
}

func TestBuildRunNoModifyConfig(t *testing.T) {
	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor only once
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()
	p.On("Create").Return(&e).Once()

	// Attempt to modify the Config object
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(func(options ExecutorPrepareOptions) error {
			options.Config.Docker.Credentials.Host = "10.0.0.2"
			return nil
		}).Once()

	// We run everything else once
	e.On("Finish", nil).Once()
	e.On("Cleanup").Once()

	// Run script successfully
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageUserScript)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageAfterScript)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageArchiveCache)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageUploadOnSuccessArtifacts)).Return(nil).Once()

	RegisterExecutorProvider("build-run-nomodify-test", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	rc := &RunnerConfig{
		RunnerSettings: RunnerSettings{
			Executor: "build-run-nomodify-test",
			Docker: &DockerConfig{
				Credentials: docker.Credentials{
					Host: "10.0.0.1",
				},
			},
		},
	}
	build, err := NewBuild(successfulBuild, rc, nil, nil)
	assert.NoError(t, err)

	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)
	assert.Equal(t, "10.0.0.1", rc.Docker.Credentials.Host)
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

	RegisterExecutorProvider("build-run-retry-prepare", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-retry-prepare",
			},
		},
	}
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
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

	RegisterExecutorProvider("build-run-prepare-failure", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-prepare-failure",
			},
		},
	}
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "prepare failed")
}

func TestPrepareFailureOnBuildError(t *testing.T) {
	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(&e).Times(1)

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(&BuildError{}).Times(1)
	e.On("Cleanup").Times(1)

	RegisterExecutorProvider("build-run-prepare-failure-on-build-error", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-prepare-failure-on-build-error",
			},
		},
	}
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.IsType(t, err, &BuildError{})
}

func TestJobFailure(t *testing.T) {
	e := new(MockExecutor)
	defer e.AssertExpectations(t)

	p := new(MockExecutorProvider)
	defer p.AssertExpectations(t)

	// Create executor
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(e).Times(1)

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Times(1)
	e.On("Cleanup").Times(1)

	// Succeed a build script
	thrownErr := &BuildError{Inner: errors.New("test error")}
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", mock.Anything).Return(thrownErr)
	e.On("Finish", thrownErr).Once()

	RegisterExecutorProvider("build-run-job-failure", p)

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
	trace.On("SetMasked", mock.Anything).Once()
	trace.On("Fail", thrownErr, ScriptFailure).Once()

	err = build.Run(&Config{}, trace)
	require.IsType(t, &BuildError{}, err)
}

func TestJobFailureOnExecutionTimeout(t *testing.T) {
	e := new(MockExecutor)
	defer e.AssertExpectations(t)

	p := new(MockExecutorProvider)
	defer p.AssertExpectations(t)

	// Create executor
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(e).Times(1)

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Times(1)
	e.On("Cleanup").Times(1)

	// Succeed a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStageUserScript)).Run(func(arguments mock.Arguments) {
		time.Sleep(2 * time.Second)
	}).Return(nil)
	e.On("Run", mock.Anything).Return(nil)
	e.On("Finish", mock.Anything).Once()

	RegisterExecutorProvider("build-run-job-failure-on-execution-timeout", p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)

	successfulBuild.RunnerInfo.Timeout = 1

	build := &Build{
		JobResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-job-failure-on-execution-timeout",
			},
		},
	}

	trace := new(MockJobTrace)
	defer trace.AssertExpectations(t)
	trace.On("Write", mock.Anything).Return(0, nil)
	trace.On("IsStdout").Return(true)
	trace.On("SetCancelFunc", mock.Anything).Once()
	trace.On("SetMasked", mock.Anything).Once()
	trace.On("Fail", mock.Anything, JobExecutionTimeout).Run(func(arguments mock.Arguments) {
		assert.Error(t, arguments.Get(0).(error))
	}).Once()

	err = build.Run(&Config{}, trace)
	require.IsType(t, &BuildError{}, err)
}

func matchBuildStage(buildStage BuildStage) interface{} {
	return mock.MatchedBy(func(cmd ExecutorCommand) bool {
		return cmd.Stage == buildStage
	})
}

func TestRunFailureRunsAfterScriptAndArtifactsOnFailure(t *testing.T) {
	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(&e).Once()

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("Cleanup").Once()

	// Fail a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageUserScript)).Return(errors.New("build fail")).Once()
	e.On("Run", matchBuildStage(BuildStageAfterScript)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageUploadOnFailureArtifacts)).Return(nil).Once()
	e.On("Finish", errors.New("build fail")).Once()

	RegisterExecutorProvider("build-run-run-failure", &p)

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
	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(&e).Once()

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("Cleanup")

	// Fail a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageGetSources)).Return(errors.New("build fail")).Times(3)
	e.On("Run", matchBuildStage(BuildStageUploadOnFailureArtifacts)).Return(nil).Once()
	e.On("Finish", errors.New("build fail")).Once()

	RegisterExecutorProvider("build-get-sources-run-failure", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-get-sources-run-failure",
			},
		},
	}

	build.Variables = append(build.Variables, JobVariable{Key: "GET_SOURCES_ATTEMPTS", Value: "3"})
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "build fail")
}

func TestArtifactDownloadRunFailure(t *testing.T) {
	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(&e).Once()

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("Cleanup")

	// Fail a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(errors.New("build fail")).Times(3)
	e.On("Run", matchBuildStage(BuildStageUploadOnFailureArtifacts)).Return(nil).Once()
	e.On("Finish", errors.New("build fail")).Once()

	RegisterExecutorProvider("build-artifacts-run-failure", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-artifacts-run-failure",
			},
		},
	}

	build.Variables = append(build.Variables, JobVariable{Key: "ARTIFACT_DOWNLOAD_ATTEMPTS", Value: "3"})
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "build fail")
}

func TestArtifactUploadRunFailure(t *testing.T) {
	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(&e).Once()

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("Cleanup")

	// Successful build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"}).Times(8)
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageUserScript)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageAfterScript)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageArchiveCache)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageUploadOnSuccessArtifacts)).Return(errors.New("upload fail")).Once()
	e.On("Finish", errors.New("upload fail")).Once()

	RegisterExecutorProvider("build-upload-artifacts-run-failure", &p)

	successfulBuild, err := GetSuccessfulBuild()
	successfulBuild.Artifacts = make(Artifacts, 1)
	successfulBuild.Artifacts[0] = Artifact{
		Name:      "my-artifact",
		Untracked: false,
		Paths:     ArtifactPaths{"cached/*"},
		When:      ArtifactWhenAlways,
	}
	assert.NoError(t, err)
	build := &Build{
		JobResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-upload-artifacts-run-failure",
			},
		},
	}

	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "upload fail")
}

func TestRestoreCacheRunFailure(t *testing.T) {
	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(&e).Once()

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("Cleanup")

	// Fail a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(errors.New("build fail")).Times(3)
	e.On("Run", matchBuildStage(BuildStageUploadOnFailureArtifacts)).Return(nil).Once()
	e.On("Finish", errors.New("build fail")).Once()

	RegisterExecutorProvider("build-cache-run-failure", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-cache-run-failure",
			},
		},
	}

	build.Variables = append(build.Variables, JobVariable{Key: "RESTORE_CACHE_ATTEMPTS", Value: "3"})
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "build fail")
}

func TestRunWrongAttempts(t *testing.T) {
	e := MockExecutor{}

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(&e)

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("Cleanup")

	// Fail a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", mock.Anything).Return(nil).Once()
	e.On("Run", mock.Anything).Return(errors.New("number of attempts out of the range [1, 10] for stage: get_sources"))
	e.On("Finish", errors.New("number of attempts out of the range [1, 10] for stage: get_sources"))

	RegisterExecutorProvider("build-run-attempt-failure", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-attempt-failure",
			},
		},
	}

	build.Variables = append(build.Variables, JobVariable{Key: "GET_SOURCES_ATTEMPTS", Value: "0"})
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "number of attempts out of the range [1, 10] for stage: get_sources")
}

func TestRunSuccessOnSecondAttempt(t *testing.T) {
	e := MockExecutor{}
	p := MockExecutorProvider{}

	// Create executor only once
	p.On("CanCreate").Return(true).Once()
	p.On("GetDefaultShell").Return("bash").Once()
	p.On("GetFeatures", mock.Anything).Return(nil).Twice()

	p.On("Create").Return(&e).Once()

	// We run everything once
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	e.On("Finish", mock.Anything).Twice()
	e.On("Cleanup").Twice()

	// Run script successfully
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})

	e.On("Run", mock.Anything).Return(nil)
	e.On("Run", mock.Anything).Return(errors.New("build fail")).Once()
	e.On("Run", mock.Anything).Return(nil)

	RegisterExecutorProvider("build-run-success-second-attempt", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		JobResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-success-second-attempt",
			},
		},
	}

	build.Variables = append(build.Variables, JobVariable{Key: "GET_SOURCES_ATTEMPTS", Value: "3"})
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
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
				build.Variables = append(build.Variables, JobVariable{Key: "CI_DEBUG_TRACE", Value: testCase.debugTraceVariableValue, Public: true})
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
	buildDir := "/tmp/test-build/dir"
	build := Build{
		BuildDir: buildDir,
	}

	vars := build.GetAllVariables().StringList()

	assert.Contains(t, vars, "CI_PROJECT_DIR="+filepath.FromSlash(buildDir))
	assert.Contains(t, vars, "CI_SERVER=yes")
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
				tmp := present
				present = absent
				absent = tmp
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
	testCases := []struct {
		runner RunnerSettings
		result string
	}{
		{
			runner: RunnerSettings{
				CloneURL: "http://test.local/",
			},
			result: "http://gitlab-ci-token:1234567@test.local/h5bp/html5-boilerplate.git",
		},
		{
			runner: RunnerSettings{
				CloneURL: "https://test.local",
			},
			result: "https://gitlab-ci-token:1234567@test.local/h5bp/html5-boilerplate.git",
		},
		{
			runner: RunnerSettings{},
			result: "http://fallback.url",
		},
	}

	for _, tc := range testCases {
		build := &Build{
			Runner: &RunnerConfig{
				RunnerSettings: tc.runner,
			},
			allVariables: JobVariables{
				JobVariable{Key: "CI_JOB_TOKEN", Value: "1234567"},
				JobVariable{Key: "CI_PROJECT_PATH", Value: "h5bp/html5-boilerplate"},
			},
			JobResponse: JobResponse{
				GitInfo: GitInfo{RepoURL: "http://fallback.url"},
			},
		}

		assert.Equal(t, tc.result, build.GetRemoteURL())
	}
}

type featureFlagOnTestCase struct {
	value          string
	expectedStatus bool
	expectedError  bool
}

func TestIsFeatureFlagOn(t *testing.T) {
	hook := test.NewGlobal()

	tests := map[string]featureFlagOnTestCase{
		"no value": {
			value:          "",
			expectedStatus: false,
			expectedError:  false,
		},
		"true": {
			value:          "true",
			expectedStatus: true,
			expectedError:  false,
		},
		"1": {
			value:          "1",
			expectedStatus: true,
			expectedError:  false,
		},
		"false": {
			value:          "false",
			expectedStatus: false,
			expectedError:  false,
		},
		"0": {
			value:          "0",
			expectedStatus: false,
			expectedError:  false,
		},
		"invalid value": {
			value:          "test",
			expectedStatus: false,
			expectedError:  true,
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			build := new(Build)
			build.Variables = JobVariables{
				{Key: "FF_TEST_FEATURE", Value: testCase.value},
			}

			status := build.IsFeatureFlagOn("FF_TEST_FEATURE")
			assert.Equal(t, testCase.expectedStatus, status)

			entry := hook.LastEntry()
			if testCase.expectedError {
				require.NotNil(t, entry)

				logrusOutput, err := entry.String()
				require.NoError(t, err)

				assert.Contains(t, logrusOutput, "Error while parsing the value of feature flag")
			} else {
				assert.Nil(t, entry)
			}

			hook.Reset()
		})
	}
}

func TestAllowToOverwriteFeatureFlagWithRunnerVariables(t *testing.T) {
	tests := map[string]struct {
		variable      string
		expectedValue bool
	}{
		"it has default value of FF": {
			variable:      "",
			expectedValue: false,
		},
		"it enables FF": {
			variable:      "FF_USE_LEGACY_VOLUMES_MOUNTING_ORDER=true",
			expectedValue: true,
		},
		"it disable FF": {
			variable:      "FF_USE_LEGACY_VOLUMES_MOUNTING_ORDER=false",
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

			result := build.IsFeatureFlagOn("FF_USE_LEGACY_VOLUMES_MOUNTING_ORDER")
			assert.Equal(t, test.expectedValue, result)
		})
	}
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
				{Key: "GIT_CLONE_PATH", Value: "$CI_BUILDS_DIR/go/src/gitlab.com/test-namespace/test-repo", Public: true},
			},
			expectedBuildDir: "/builds/go/src/gitlab.com/test-namespace/test-repo",
			expectedCacheDir: "/cache/test-namespace/test-repo",
			expectedError:    false,
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

			err := build.StartBuild(test.args.rootDir, test.args.cacheDir, test.args.customBuildDirEnabled, test.args.sharedDir)
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

			srv := httptest.NewServer(build.Session.Mux())
			defer srv.Close()

			mockConn := terminal.MockConn{}
			defer mockConn.AssertExpectations(t)
			mockConn.On("Close").Maybe().Return(nil)
			// On Start upgrade the web socket connection and wait for the
			// timeoutCh to exit, to mock real work made on the websocket.
			mockConn.On("Start", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
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

			conn, _, err := websocket.DefaultDialer.Dial(u.String(), headers)
			require.NotNil(t, conn)
			require.NoError(t, err)
			defer conn.Close()

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
				b.Variables = append(b.Variables, JobVariable{Key: "GIT_LFS_SKIP_SMUDGE", Value: testCase.variableValue, Public: true})
			}

			assert.Equal(t, testCase.expectedResult, b.IsLFSSmudgeDisabled())
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
		build        Build
		expectedName string
	}{
		"project non rfc1132 unique name": {
			build: Build{
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
		"project non rfc1132 unique name longer than 63 char": {
			build: Build{
				Runner: &RunnerConfig{
					RunnerCredentials: RunnerCredentials{
						Token: "Ze_n8E6en622WxxSg4r8",
					},
				},
				JobResponse: JobResponse{
					JobInfo: JobInfo{
						ProjectID: 123456789012345,
					},
				},
				ProjectRunnerID: 123456789012345,
			},
			expectedName: "runner-zen8e6e-project-123456789012345-concurrent-1234567890123",
		},
		"project normal unique name": {
			build: Build{
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

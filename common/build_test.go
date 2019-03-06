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
	e.On("Finish", nil).Return().Once()
	e.On("Cleanup").Return().Once()

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

	RegisterExecutor("build-run-test", &p)

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
			options.Config.Docker.DockerCredentials.Host = "10.0.0.2"
			return nil
		}).Once()

	// We run everything else once
	e.On("Finish", nil).Return().Once()
	e.On("Cleanup").Return().Once()

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

	RegisterExecutor("build-run-nomodify-test", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	rc := &RunnerConfig{
		RunnerSettings: RunnerSettings{
			Executor: "build-run-nomodify-test",
			Docker: &DockerConfig{
				DockerCredentials: docker_helpers.DockerCredentials{
					Host: "10.0.0.1",
				},
			},
		},
	}
	build, err := NewBuild(successfulBuild, rc, nil, nil)
	assert.NoError(t, err)

	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)
	assert.Equal(t, "10.0.0.1", rc.Docker.DockerCredentials.Host)
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
	e.On("Cleanup").Return().Times(3)

	// Succeed a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", mock.Anything).Return(nil)
	e.On("Finish", nil).Return().Once()

	RegisterExecutor("build-run-retry-prepare", &p)

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
	e.On("Cleanup").Return().Times(3)

	RegisterExecutor("build-run-prepare-failure", &p)

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
	e.On("Cleanup").Return().Times(1)

	RegisterExecutor("build-run-prepare-failure-on-build-error", &p)

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
	e.On("Cleanup").Return().Times(1)

	// Succeed a build script
	thrownErr := &BuildError{Inner: errors.New("test error")}
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", mock.Anything).Return(thrownErr)
	e.On("Finish", thrownErr).Return().Once()

	RegisterExecutor("build-run-job-failure", p)

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
	e.On("Cleanup").Return().Times(1)

	// Succeed a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStageUserScript)).Run(func(arguments mock.Arguments) {
		time.Sleep(2 * time.Second)
	}).Return(nil)
	e.On("Run", mock.Anything).Return(nil)
	e.On("Finish", mock.Anything).Return().Once()

	RegisterExecutor("build-run-job-failure-on-execution-timeout", p)

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
	e.On("Cleanup").Return().Once()

	// Fail a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageUserScript)).Return(errors.New("build fail")).Once()
	e.On("Run", matchBuildStage(BuildStageAfterScript)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageUploadOnFailureArtifacts)).Return(nil).Once()
	e.On("Finish", errors.New("build fail")).Return().Once()

	RegisterExecutor("build-run-run-failure", &p)

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
	e.On("Cleanup").Return()

	// Fail a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageGetSources)).Return(errors.New("build fail")).Times(3)
	e.On("Run", matchBuildStage(BuildStageUploadOnFailureArtifacts)).Return(nil).Once()
	e.On("Finish", errors.New("build fail")).Return().Once()

	RegisterExecutor("build-get-sources-run-failure", &p)

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
	e.On("Cleanup").Return()

	// Fail a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageDownloadArtifacts)).Return(errors.New("build fail")).Times(3)
	e.On("Run", matchBuildStage(BuildStageUploadOnFailureArtifacts)).Return(nil).Once()
	e.On("Finish", errors.New("build fail")).Return().Once()

	RegisterExecutor("build-artifacts-run-failure", &p)

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
	e.On("Cleanup").Return()

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
	e.On("Finish", errors.New("upload fail")).Return().Once()

	RegisterExecutor("build-upload-artifacts-run-failure", &p)

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
	e.On("Cleanup").Return()

	// Fail a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", matchBuildStage(BuildStagePrepare)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageGetSources)).Return(nil).Once()
	e.On("Run", matchBuildStage(BuildStageRestoreCache)).Return(errors.New("build fail")).Times(3)
	e.On("Run", matchBuildStage(BuildStageUploadOnFailureArtifacts)).Return(nil).Once()
	e.On("Finish", errors.New("build fail")).Return().Once()

	RegisterExecutor("build-cache-run-failure", &p)

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
	e.On("Cleanup").Return()

	// Fail a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", mock.Anything).Return(nil).Once()
	e.On("Run", mock.Anything).Return(errors.New("Number of attempts out of the range [1, 10] for stage: get_sources"))
	e.On("Finish", errors.New("Number of attempts out of the range [1, 10] for stage: get_sources")).Return()

	RegisterExecutor("build-run-attempt-failure", &p)

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
	assert.EqualError(t, err, "Number of attempts out of the range [1, 10] for stage: get_sources")
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
	e.On("Finish", mock.Anything).Return().Twice()
	e.On("Cleanup").Return().Twice()

	// Run script successfully
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})

	e.On("Run", mock.Anything).Return(nil)
	e.On("Run", mock.Anything).Return(errors.New("build fail")).Once()
	e.On("Run", mock.Anything).Return(nil)

	RegisterExecutor("build-run-success-second-attempt", &p)

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
	build := &Build{}
	assert.False(t, build.IsDebugTraceEnabled(), "IsDebugTraceEnabled should be false if CI_DEBUG_TRACE is not set")

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)

	successfulBuild.Variables = append(successfulBuild.Variables, JobVariable{Key: "CI_DEBUG_TRACE", Value: "false", Public: true, Internal: true})
	build = &Build{
		JobResponse: successfulBuild,
	}
	assert.False(t, build.IsDebugTraceEnabled(), "IsDebugTraceEnabled should be false if CI_DEBUG_TRACE is set to false")

	successfulBuild, err = GetSuccessfulBuild()
	assert.NoError(t, err)

	successfulBuild.Variables = append(successfulBuild.Variables, JobVariable{Key: "CI_DEBUG_TRACE", Value: "true", Public: true, Internal: true})
	build = &Build{
		JobResponse: successfulBuild,
	}
	assert.True(t, build.IsDebugTraceEnabled(), "IsDebugTraceEnabled should be true if CI_DEBUG_TRACE is set to true")
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
			expectedErr:            "Terminal session timed out (maximum time allowed - 1s)",
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

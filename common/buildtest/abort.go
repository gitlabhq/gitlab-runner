package buildtest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type withContext struct {
}

func (c *withContext) WithContext(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(ctx)
	cancel(assert.AnError)

	return context.WithCancel(ctx)
}

//nolint:funlen,gocognit
func RunBuildWithCancel(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	abortIncludeStages := []common.BuildStage{
		common.BuildStagePrepare,
		common.BuildStageGetSources,
	}
	abortExcludeStages := []common.BuildStage{
		common.BuildStageRestoreCache,
		common.BuildStageDownloadArtifacts,
		common.BuildStageAfterScript,
		common.BuildStageArchiveOnSuccessCache,
		common.BuildStageArchiveOnFailureCache,
		common.BuildStageUploadOnFailureArtifacts,
		common.BuildStageUploadOnSuccessArtifacts,
	}

	cancelIncludeStages := []common.BuildStage{
		common.BuildStagePrepare,
		common.BuildStageGetSources,
		common.BuildStageAfterScript,
	}
	cancelExcludeStages := []common.BuildStage{
		common.BuildStageArchiveOnSuccessCache,
		common.BuildStageUploadOnSuccessArtifacts,

		common.BuildStageRestoreCache,
		common.BuildStageDownloadArtifacts,
		common.BuildStageArchiveOnFailureCache,
		common.BuildStageUploadOnFailureArtifacts,
	}

	tests := map[string]struct {
		setupFn       func(*common.Build)
		onUserStep    func(*common.Build, common.JobTrace)
		includesStage []common.BuildStage
		excludesStage []common.BuildStage
		expectedErr   error
	}{
		"job script timeout": {
			setupFn: func(build *common.Build) {
				build.Variables = append(build.Variables, common.JobVariable{
					Key:   "RUNNER_SCRIPT_TIMEOUT",
					Value: "5s",
				})
			},
			includesStage: []common.BuildStage{
				common.BuildStagePrepare,
				common.BuildStageGetSources,
				common.BuildStageAfterScript,
			},
			excludesStage: []common.BuildStage{
				common.BuildStageRestoreCache,
				common.BuildStageDownloadArtifacts,
				common.BuildStageArchiveOnSuccessCache,
				common.BuildStageArchiveOnFailureCache,
				common.BuildStageUploadOnFailureArtifacts,
				common.BuildStageUploadOnSuccessArtifacts,
			},
			expectedErr: &common.BuildError{FailureReason: common.JobExecutionTimeout},
		},
		"system interrupt": {
			onUserStep: func(build *common.Build, _ common.JobTrace) {
				build.SystemInterrupt <- os.Interrupt
			},
			includesStage: abortIncludeStages,
			excludesStage: abortExcludeStages,
			expectedErr:   &common.BuildError{FailureReason: common.RunnerSystemFailure},
		},
		"job is aborted": {
			onUserStep: func(_ *common.Build, trace common.JobTrace) {
				trace.Abort()
			},
			includesStage: abortIncludeStages,
			excludesStage: abortExcludeStages,
			expectedErr:   &common.BuildError{FailureReason: common.JobCanceled},
		},
		"job is canceling": {
			onUserStep: func(_ *common.Build, trace common.JobTrace) {
				trace.Cancel()
			},
			includesStage: cancelIncludeStages,
			excludesStage: cancelExcludeStages,
			expectedErr:   &common.BuildError{FailureReason: common.JobCanceled},
		},
	}

	resp, err := common.GetRemoteLongRunningBuildWithAfterScript()
	require.NoError(t, err)

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			build := &common.Build{
				JobResponse:     resp,
				Runner:          config,
				SystemInterrupt: make(chan os.Signal, 1),
			}
			buf := new(bytes.Buffer)
			trace := &common.Trace{Writer: io.MultiWriter(buf, os.Stdout)}

			if tc.onUserStep != nil {
				done := OnUserStage(build, func() {
					tc.onUserStep(build, trace)
				})
				defer done()
			}

			if setup != nil {
				setup(t, build)
			}
			if tc.setupFn != nil {
				tc.setupFn(build)
			}

			err := RunBuildWithTrace(t, build, trace)
			t.Log(buf.String())

			assert.True(t, errors.Is(err, tc.expectedErr), "expected: %[1]T (%[1]v), got: %[2]T (%[2]v)", tc.expectedErr, err)

			for _, stage := range tc.includesStage {
				assert.Contains(t, buf.String(), common.GetStageDescription(stage))
			}
			for _, stage := range tc.excludesStage {
				assert.NotContains(t, buf.String(), common.GetStageDescription(stage))
			}
		})
	}
}

func RunBuildWithExecutorCancel(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	resp, err := common.GetRemoteLongRunningBuildWithAfterScript()
	require.NoError(t, err)

	build := &common.Build{
		JobResponse:     resp,
		Runner:          config,
		SystemInterrupt: make(chan os.Signal, 1),
	}
	build.ExecutorData = &withContext{}

	buf := new(bytes.Buffer)
	trace := &common.Trace{Writer: io.MultiWriter(buf, os.Stdout)}

	err = RunBuildWithTrace(t, build, trace)
	t.Log(buf.String())

	assert.ErrorIs(t, err, assert.AnError)
}

package buildtest

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace"
)

func RunRemoteBuildWithJobOutputLimitExceeded(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	runBuildWithJobOutputLimitExceeded(t, config, setup, common.GetRemoteSuccessfulBuild)
}

func RunBuildWithJobOutputLimitExceeded(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	runBuildWithJobOutputLimitExceeded(t, config, setup, common.GetSuccessfulBuild)
}

type jobOutputLimitExceededTestCase struct {
	jobResponse func(t *testing.T, g baseJobGetter) common.JobResponse
	handleTrace func(t *testing.T, done chan struct{}, traceBuffer *trace.Buffer, trace common.JobTrace)
	assertError func(t *testing.T, err error)
}

var jobOutputLimitExceededTestCases = map[string]jobOutputLimitExceededTestCase{
	"successful job": {
		jobResponse: func(t *testing.T, baseJobGetter baseJobGetter) common.JobResponse {
			return getJobResponseWithCommands(t, baseJobGetter, "echo Hello World", "exit 0")
		},
		handleTrace: func(t *testing.T, done chan struct{}, traceBuffer *trace.Buffer, trace common.JobTrace) {},
		assertError: func(t *testing.T, err error) {
			assert.NoError(t, err)
		},
	},
	"failed job": {
		jobResponse: func(t *testing.T, baseJobGetter baseJobGetter) common.JobResponse {
			return getJobResponseWithCommands(t, baseJobGetter, "echo Hello World", "exit 1")
		},
		handleTrace: func(t *testing.T, done chan struct{}, traceBuffer *trace.Buffer, trace common.JobTrace) {},
		assertError: func(t *testing.T, err error) {
			var expectedErr *common.BuildError
			if assert.ErrorAs(t, err, &expectedErr) {
				assert.Equal(t, 1, expectedErr.ExitCode)
				assert.Empty(t, expectedErr.FailureReason)
			}
		},
	},
	"canceled job": {
		jobResponse: func(t *testing.T, baseJobGetter baseJobGetter) common.JobResponse {
			return getJobResponseWithCommands(t, baseJobGetter, "echo Hello World", "sleep 10", "exit 0")
		},
		handleTrace: func(t *testing.T, done chan struct{}, traceBuffer *trace.Buffer, trace common.JobTrace) {
			for {
				b, berr := traceBuffer.Bytes(0, 1024*1024)
				require.NoError(t, berr)

				if strings.Contains(string(b), "Job's log exceeded limit of") {
					trace.Cancel()
				}

				select {
				case <-time.After(50 * time.Millisecond):
				case <-done:
					return
				}
			}
		},
		assertError: func(t *testing.T, err error) {
			var expectedErr *common.BuildError
			if assert.ErrorAs(t, err, &expectedErr) {
				assert.Equal(t, 0, expectedErr.ExitCode)
				assert.Equal(t, common.JobCanceled, expectedErr.FailureReason)
			}
		},
	},
}

// nolint:funlen
func runBuildWithJobOutputLimitExceeded(
	t *testing.T,
	config *common.RunnerConfig,
	setup BuildSetupFn,
	baseJob func() (common.JobResponse, error),
) {
	for tn, tt := range jobOutputLimitExceededTestCases {
		t.Run(tn, func(t *testing.T) {
			build := &common.Build{
				JobResponse:     tt.jobResponse(t, baseJob),
				Runner:          config,
				SystemInterrupt: make(chan os.Signal, 1),
			}

			if setup != nil {
				setup(build)
			}

			runBuildWithJobOutputLimitExceededCase(t, tt, build)
		})
	}
}

func runBuildWithJobOutputLimitExceededCase(t *testing.T, tt jobOutputLimitExceededTestCase, build *common.Build) {
	traceBuffer, err := trace.New()
	require.NoError(t, err)

	traceBuffer.SetLimit(12)

	jobTrace := &common.Trace{Writer: traceBuffer}

	done := make(chan struct{})
	defer close(done)

	go tt.handleTrace(t, done, traceBuffer, jobTrace)

	err = RunBuildWithTrace(t, build, jobTrace)

	b, berr := traceBuffer.Bytes(0, 1024*1024)
	require.NoError(t, berr)

	log := string(b)
	assert.Contains(t, log, "Running")
	assert.NotContains(t, log, "with gitlab-runner")
	assert.Contains(t, log, "Job's log exceeded limit of 12 bytes.")
	assert.Contains(t, log, "Job execution will continue but no more output will be collected.")

	tt.assertError(t, err)
}

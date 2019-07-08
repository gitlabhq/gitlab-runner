package custom

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/custom/command"
)

type executorTestCase struct {
	config common.RunnerConfig

	commandStdoutContent string
	commandStderrContent string
	commandErr           error

	doNotMockCommandFactory bool

	adjustExecutor func(t *testing.T, e *executor)

	assertCommandFactory func(t *testing.T, tt executorTestCase, ctx context.Context, executable string, args []string, options command.CreateOptions)
	assertOutput         func(t *testing.T, output string)
	expectedError        string
}

func getRunnerConfig(custom *common.CustomConfig) common.RunnerConfig {
	rc := common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			BuildsDir: "/builds",
			CacheDir:  "/cache",
			Shell:     "bash",
		},
	}

	if custom != nil {
		rc.Custom = custom
	}

	return rc
}

func prepareExecutorForCleanup(t *testing.T, tt executorTestCase) (*executor, *bytes.Buffer) {
	e, options, out := prepareExecutor(t, tt)

	e.Config = *options.Config
	e.Build = options.Build
	e.Trace = options.Trace
	e.BuildLogger = common.NewBuildLogger(e.Trace, e.Build.Log())

	return e, out
}

func prepareExecutor(t *testing.T, tt executorTestCase) (*executor, common.ExecutorPrepareOptions, *bytes.Buffer) {
	out := bytes.NewBuffer([]byte{})

	successfulBuild, err := common.GetSuccessfulBuild()
	require.NoError(t, err)

	successfulBuild.ID = jobID()

	trace := new(common.MockJobTrace)
	defer trace.AssertExpectations(t)

	trace.On("Write", mock.Anything).
		Run(func(args mock.Arguments) {
			_, err := io.Copy(out, bytes.NewReader(args.Get(0).([]byte)))
			require.NoError(t, err)
		}).
		Return(0, nil).
		Maybe()
	trace.On("IsStdout").
		Return(false).
		Maybe()

	options := common.ExecutorPrepareOptions{
		Build: &common.Build{
			JobResponse: successfulBuild,
			Runner:      &tt.config,
		},
		Config:  &tt.config,
		Context: context.Background(),
		Trace:   trace,
	}

	e := new(executor)

	return e, options, out
}

var currentJobID = 0

func jobID() int {
	i := currentJobID
	currentJobID++

	return i
}

func assertOutput(t *testing.T, tt executorTestCase, out *bytes.Buffer) {
	if tt.assertOutput == nil {
		return
	}

	tt.assertOutput(t, out.String())
}

func mockCommandFactory(t *testing.T, tt executorTestCase) func() {
	if tt.doNotMockCommandFactory {
		return func() {}
	}

	outputs := commandOutputs{
		stdout: nil,
		stderr: nil,
	}

	cmd := new(command.MockCommand)
	cmd.On("Run").
		Run(func(_ mock.Arguments) {
			if tt.commandStdoutContent != "" && outputs.stdout != nil {
				_, err := fmt.Fprintln(outputs.stdout, tt.commandStdoutContent)
				require.NoError(t, err, "Unexpected error on mocking command output to stdout")
			}

			if tt.commandStderrContent != "" && outputs.stderr != nil {
				_, err := fmt.Fprintln(outputs.stderr, tt.commandStderrContent)
				require.NoError(t, err, "Unexpected error on mocking command output to stderr")
			}
		}).
		Return(tt.commandErr)

	oldFactory := commandFactory
	commandFactory = func(ctx context.Context, executable string, args []string, options command.CreateOptions) command.Command {
		if tt.assertCommandFactory != nil {
			tt.assertCommandFactory(t, tt, ctx, executable, args, options)
		}

		outputs.stdout = options.Stdout
		outputs.stderr = options.Stderr

		return cmd
	}

	return func() {
		cmd.AssertExpectations(t)
		commandFactory = oldFactory
	}
}

func TestExecutor_Prepare(t *testing.T) {
	tests := map[string]executorTestCase{
		"AbstractExecutor.Prepare failure": {
			config:                  common.RunnerConfig{},
			doNotMockCommandFactory: true,
			expectedError:           "the builds_dir is not configured",
		},
		"custom executor not set": {
			config:                  getRunnerConfig(nil),
			doNotMockCommandFactory: true,
			expectedError:           "custom executor not configured",
		},
		"custom executor set without RunExec": {
			config:                  getRunnerConfig(&common.CustomConfig{}),
			doNotMockCommandFactory: true,
			expectedError:           "custom executor is missing RunExec",
		},
		"custom executor set": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec: "bash",
			}),
			doNotMockCommandFactory: true,
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Using Custom executor...")
			},
		},
		"custom executor set with PrepareExec": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:     "bash",
				PrepareExec: "echo",
				PrepareArgs: []string{"test"},
			}),
			assertCommandFactory: func(t *testing.T, tt executorTestCase, ctx context.Context, executable string, args []string, options command.CreateOptions) {
				assert.Equal(t, tt.config.Custom.PrepareExec, executable)
				assert.Equal(t, tt.config.Custom.PrepareArgs, args)
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Using Custom executor...")
			},
		},
		"custom executor set with PrepareExec with error": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:     "bash",
				PrepareExec: "echo",
				PrepareArgs: []string{"test"},
			}),
			commandErr: errors.New("test-error"),
			assertCommandFactory: func(t *testing.T, tt executorTestCase, ctx context.Context, executable string, args []string, options command.CreateOptions) {
				assert.Equal(t, tt.config.Custom.PrepareExec, executable)
				assert.Equal(t, tt.config.Custom.PrepareArgs, args)
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Using Custom executor...")
			},
			expectedError: "test-error",
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			defer mockCommandFactory(t, tt)()

			e, options, out := prepareExecutor(t, tt)
			err := e.Prepare(options)

			assertOutput(t, tt, out)

			if tt.expectedError == "" {
				assert.NoError(t, err)

				return
			}

			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestExecutor_Cleanup(t *testing.T) {
	tests := map[string]executorTestCase{
		"custom executor not set": {
			config: getRunnerConfig(nil),
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "custom executor not configured")
			},
			doNotMockCommandFactory: true,
		},
		"custom executor set without RunExec": {
			config: getRunnerConfig(&common.CustomConfig{}),
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "custom executor is missing RunExec")
			},
			doNotMockCommandFactory: true,
		},
		"custom executor set": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec: "bash",
			}),
			doNotMockCommandFactory: true,
		},
		"custom executor set with CleanupExec": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:     "bash",
				CleanupExec: "echo",
				CleanupArgs: []string{"test"},
			}),
			assertCommandFactory: func(t *testing.T, tt executorTestCase, ctx context.Context, executable string, args []string, options command.CreateOptions) {
				assert.Equal(t, tt.config.Custom.CleanupExec, executable)
				assert.Equal(t, tt.config.Custom.CleanupArgs, args)
			},
			assertOutput: func(t *testing.T, output string) {
				assert.NotContains(t, output, "WARNING: Cleanup script failed:")
			},
		},
		"custom executor set with CleanupExec with error": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:     "bash",
				CleanupExec: "unknown",
			}),
			commandStdoutContent: "some output message in commands output",
			commandStderrContent: "some error message in commands output",
			commandErr:           errors.New("test-error"),
			assertCommandFactory: func(t *testing.T, tt executorTestCase, ctx context.Context, executable string, args []string, options command.CreateOptions) {
				assert.Equal(t, tt.config.Custom.CleanupExec, executable)
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "WARNING: Cleanup script failed: test-error")
			},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			defer mockCommandFactory(t, tt)()

			e, out := prepareExecutorForCleanup(t, tt)
			e.Cleanup()

			assertOutput(t, tt, out)
		})
	}
}

func TestExecutor_Run(t *testing.T) {
	tests := map[string]executorTestCase{
		"Run fails on tempdir operations": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec: "bash",
			}),
			doNotMockCommandFactory: true,
			adjustExecutor: func(t *testing.T, e *executor) {
				curDir, err := os.Getwd()
				require.NoError(t, err)
				e.tempDir = filepath.Join(curDir, "unknown")
			},
			expectedError: func() string {
				if runtime.GOOS == "windows" {
					return "The system cannot find the file specified"
				}

				return "no such file or directory"
			}(),
		},
		"Run executes job": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec: "bash",
			}),
			assertCommandFactory: func(t *testing.T, tt executorTestCase, ctx context.Context, executable string, args []string, options command.CreateOptions) {
				assert.Equal(t, tt.config.Custom.RunExec, executable)
			},
		},
		"Run executes job with error": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:     "bash",
				CleanupExec: "unknown",
			}),
			commandErr: errors.New("test-error"),
			assertCommandFactory: func(t *testing.T, tt executorTestCase, ctx context.Context, executable string, args []string, options command.CreateOptions) {
				assert.Equal(t, tt.config.Custom.RunExec, executable)
			},
			expectedError: "test-error",
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			defer mockCommandFactory(t, tt)()

			e, options, out := prepareExecutor(t, tt)

			err := e.Prepare(options)
			require.NoError(t, err)

			if tt.adjustExecutor != nil {
				tt.adjustExecutor(t, e)
			}

			err = e.Run(common.ExecutorCommand{
				Context: context.Background(),
			})

			assertOutput(t, tt, out)

			if tt.expectedError == "" {
				assert.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

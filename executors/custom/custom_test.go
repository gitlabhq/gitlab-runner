//go:build !integration

package custom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/custom/command"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

type executorTestCase struct {
	config common.RunnerConfig

	commandStdoutContent string
	commandStderrContent string
	commandErr           error

	doNotMockCommandFactory bool

	adjustExecutor func(t *testing.T, e *executor)

	assertBuild          func(t *testing.T, b *common.Build)
	assertCommandFactory func(
		t *testing.T,
		tt executorTestCase,
		ctx context.Context,
		executable string,
		args []string,
		cmdOpts process.CommandOptions,
		options command.Options,
	)
	assertOutput   func(t *testing.T, output string)
	assertExecutor func(t *testing.T, e *executor)
	expectedError  string
}

func getRunnerConfig(custom *common.CustomConfig) common.RunnerConfig {
	rc := common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "RuNnErToKeN",
		},
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

var currentJobID = int64(0)

func jobID() int64 {
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
	commandFactory =
		func(
			ctx context.Context,
			executable string,
			args []string,
			cmdOpts process.CommandOptions,
			options command.Options,
		) command.Command {
			if tt.assertCommandFactory != nil {
				tt.assertCommandFactory(t, tt, ctx, executable, args, cmdOpts, options)
			}

			outputs.stdout = cmdOpts.Stdout
			outputs.stderr = cmdOpts.Stderr

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
			expectedError:           "custom executor not configured",
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
		"custom executor set with ConfigExec with error": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:    "bash",
				ConfigExec: "echo",
				ConfigArgs: []string{"test"},
			}),
			commandErr: errors.New("test-error"),
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				assert.Equal(t, tt.config.Custom.ConfigExec, executable)
				assert.Equal(t, tt.config.Custom.ConfigArgs, args)
			},
			assertOutput: func(t *testing.T, output string) {
				assert.NotContains(t, output, "Using Custom executor...")
			},
			expectedError: "test-error",
		},
		"custom executor set with ConfigExec with invalid JSON": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:    "bash",
				ConfigExec: "echo",
			}),
			commandStdoutContent: "abcd",
			commandErr:           nil,
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				assert.Equal(t, tt.config.Custom.ConfigExec, executable)
			},
			assertOutput: func(t *testing.T, output string) {
				assert.NotContains(t, output, "Using Custom executor...")
			},
			expectedError: "error while parsing JSON output: invalid character 'a' looking for beginning of value",
		},
		"custom executor set with ConfigExec with empty JSON": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:    "bash",
				ConfigExec: "echo",
			}),
			commandStdoutContent: "",
			commandErr:           nil,
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				assert.Equal(t, tt.config.Custom.ConfigExec, executable)
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Using Custom executor...")
			},
			assertBuild: func(t *testing.T, b *common.Build) {
				assert.Equal(t, "/builds/project-0", b.BuildDir)
				assert.Equal(t, "/cache/project-0", b.CacheDir)
			},
			assertExecutor: func(t *testing.T, e *executor) {
				assert.Nil(t, e.jobEnv)
			},
		},
		"custom executor set with ConfigExec with undefined builds_dir": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:    "bash",
				ConfigExec: "echo",
			}),
			commandStdoutContent: `{"builds_dir":""}`,
			commandErr:           nil,
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				assert.Equal(t, tt.config.Custom.ConfigExec, executable)
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Using Custom executor...")
			},
			expectedError: "the builds_dir is not configured",
		},
		"custom executor set with ConfigExec and driver info missing name": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:    "bash",
				ConfigExec: "echo",
			}),
			commandStdoutContent: `{
				"driver": {
					"version": "v0.0.1"
				}
			}`,
			commandErr: nil,
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				assert.Equal(t, tt.config.Custom.ConfigExec, executable)
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Using Custom executor...")
			},
		},
		"custom executor set with ConfigExec and driver info missing version": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:    "bash",
				ConfigExec: "echo",
			}),
			commandStdoutContent: `{
				"driver": {
					"name": "test driver"
				}
			}`,
			commandErr: nil,
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				assert.Equal(t, tt.config.Custom.ConfigExec, executable)
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Using Custom executor with driver test driver...")
			},
		},
		"custom executor set with ConfigExec": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:    "bash",
				ConfigExec: "echo",
			}),
			commandStdoutContent: `{
				"hostname": "custom-hostname",
				"builds_dir": "/some/build/directory",
				"cache_dir": "/some/cache/directory",
				"builds_dir_is_shared":true,
				"driver": {
					"name": "test driver",
					"version": "v0.0.1"
				}
			}`,
			commandErr: nil,
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				assert.Equal(t, tt.config.Custom.ConfigExec, executable)
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Using Custom executor with driver test driver v0.0.1...")
			},
			assertBuild: func(t *testing.T, b *common.Build) {
				assert.Equal(t, "custom-hostname", b.Hostname)
				assert.Equal(t, "/some/build/directory/RuNnErTo/0/project-0", b.BuildDir)
				assert.Equal(t, "/some/cache/directory/project-0", b.CacheDir)
			},
		},
		"custom executor set with PrepareExec": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:     "bash",
				PrepareExec: "echo",
				PrepareArgs: []string{"test"},
			}),
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
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
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				assert.Equal(t, tt.config.Custom.PrepareExec, executable)
				assert.Equal(t, tt.config.Custom.PrepareArgs, args)
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Using Custom executor...")
			},
			expectedError: "test-error",
		},
		"custom executor set with valid job_env": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:    "bash",
				ConfigExec: "echo",
			}),
			commandStdoutContent: `{
				"builds_dir": "/some/build/directory",
				"job_env": {
					"FOO": "Hello",
					"BAR": "World"
				}
			}`,
			commandErr: nil,
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				assert.Equal(t, tt.config.Custom.ConfigExec, executable)
			},
			assertBuild: func(t *testing.T, b *common.Build) {
				assert.Equal(t, "/some/build/directory/project-0", b.BuildDir)
			},
			assertExecutor: func(t *testing.T, e *executor) {
				assert.Len(t, e.jobEnv, 2)
				require.Contains(t, e.jobEnv, "FOO")
				assert.Equal(t, "Hello", e.jobEnv["FOO"])
				require.Contains(t, e.jobEnv, "BAR")
				assert.Equal(t, "World", e.jobEnv["BAR"])
			},
		},
		"custom executor set with valid job_env, verify variable order and prefix": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:     "run-executable",
				ConfigExec:  "config-executable",
				PrepareExec: "prepare-executable",
				PrepareArgs: []string{"test"},
			}),
			commandStdoutContent: `{
				"builds_dir": "/some/build/directory",
				"job_env": {
					"FOO": "Hello"
				}
			}`,
			commandErr: nil,
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				if executable != "prepare-executable" {
					return
				}

				require.True(t, len(cmdOpts.Env) >= 2, "cmdOpts.Env must contain 2 elements or more")
				assert.Equal(t, "FOO=Hello", cmdOpts.Env[0], "first env var must be FOO")
				assert.True(
					t,
					strings.HasPrefix(cmdOpts.Env[1], "CUSTOM_ENV_"),
					"must be followed by CUSTOM_ENV_* variables",
				)
			},
		},
		"job response file specified in file": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:     "run-executable",
				ConfigExec:  "config-executable",
				PrepareExec: "prepare-executable",
				PrepareArgs: []string{"test"},
			}),
			commandStdoutContent: `{
				"builds_dir": "/some/build/directory"
			}`,
			commandErr: nil,
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				assert.NotEmpty(t, options.JobResponseFile)
			},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			defer mockCommandFactory(t, tt)()

			e, options, out := prepareExecutor(t, tt)
			err := e.Prepare(options)

			assertOutput(t, tt, out)

			if tt.assertBuild != nil {
				tt.assertBuild(t, e.Build)
			}

			if tt.assertExecutor != nil {
				tt.assertExecutor(t, e)
			}

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
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
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
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				assert.Equal(t, tt.config.Custom.CleanupExec, executable)
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "WARNING: Cleanup script failed: test-error")
			},
		},
		"custom executor set with valid job_env, verify variable order and prefix": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:     "bash",
				CleanupExec: "echo",
				CleanupArgs: []string{"test"},
			}),
			adjustExecutor: func(t *testing.T, e *executor) {
				e.jobEnv = map[string]string{"FOO": "Hello"}
			},
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				require.True(t, len(cmdOpts.Env) >= 2, "cmdOpts.Env must contain 2 elements or more")
				assert.Equal(t, "FOO=Hello", cmdOpts.Env[0], "first env var must be FOO")
				assert.True(
					t,
					strings.HasPrefix(cmdOpts.Env[1], "CUSTOM_ENV_"),
					"must be followed by CUSTOM_ENV_* variables",
				)
			},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			defer mockCommandFactory(t, tt)()

			e, out := prepareExecutorForCleanup(t, tt)

			if tt.adjustExecutor != nil {
				tt.adjustExecutor(t, e)
			}

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
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				assert.Equal(t, tt.config.Custom.RunExec, executable)
				assert.Len(t, args, 2)
				assert.Equal(t, "build_script", args[1])
			},
		},
		"Run executes job with error": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec:     "bash",
				CleanupExec: "unknown",
			}),
			commandErr: errors.New("test-error"),
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				assert.Equal(t, tt.config.Custom.RunExec, executable)
			},
			expectedError: "test-error",
		},
		"custom executor set with valid job_env, verify variable order and prefix": {
			config: getRunnerConfig(&common.CustomConfig{
				RunExec: "bash",
			}),
			adjustExecutor: func(t *testing.T, e *executor) {
				e.jobEnv = map[string]string{"FOO": "Hello"}
			},
			assertCommandFactory: func(
				t *testing.T,
				tt executorTestCase,
				ctx context.Context,
				executable string,
				args []string,
				cmdOpts process.CommandOptions,
				options command.Options,
			) {
				require.True(t, len(cmdOpts.Env) >= 2, "cmdOpts.Env must contain 2 elements or more")
				assert.Equal(t, "FOO=Hello", cmdOpts.Env[0], "first env var must be FOO")
				assert.True(
					t,
					strings.HasPrefix(cmdOpts.Env[1], "CUSTOM_ENV_"),
					"must be followed by CUSTOM_ENV_* variables",
				)
			},
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
				Stage:   "step_script",
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

func TestExecutor_Env(t *testing.T) {
	ciJobImageEnv := "CUSTOM_ENV_CI_JOB_IMAGE"

	runnerConfig := getRunnerConfig(&common.CustomConfig{
		RunExec:     "bash",
		PrepareExec: "echo",
		CleanupExec: "bash",
	})

	//nolint:lll
	assertCommandFactory := func(expectedImageName string) func(
		t *testing.T,
		tt executorTestCase,
		ctx context.Context,
		executable string,
		args []string,
		cmdOpts process.CommandOptions,
		options command.Options,
	) {
		return func(
			t *testing.T,
			tt executorTestCase,
			ctx context.Context,
			executable string,
			args []string,
			cmdOpts process.CommandOptions,
			options command.Options,
		) {
			for _, env := range cmdOpts.Env {
				pair := strings.Split(env, "=")
				if pair[0] == ciJobImageEnv {
					assert.Equal(t, expectedImageName, pair[1])
					break
				}
			}
		}
	}

	adjustExecutorFactory := func(imageName string) func(t *testing.T, e *executor) {
		return func(t *testing.T, e *executor) {
			// the build is assumed to be non-nil across the executor codebase
			e.Build.Image = common.Image{Name: imageName}
		}
	}

	tests := map[string]executorTestCase{
		"custom executor set " + ciJobImageEnv: {
			config:               runnerConfig,
			adjustExecutor:       adjustExecutorFactory("test_image"),
			assertCommandFactory: assertCommandFactory("test_image"),
		},
		"custom executor set empty " + ciJobImageEnv: {
			config:               runnerConfig,
			adjustExecutor:       adjustExecutorFactory(""),
			assertCommandFactory: assertCommandFactory(""),
		},
		"custom executor set expanded " + ciJobImageEnv: {
			config: runnerConfig,
			adjustExecutor: func(t *testing.T, e *executor) {
				e.Build.Variables = append(e.Build.Variables, common.JobVariable{
					Key:   "to_expand",
					Value: "expanded",
				})
				adjustExecutorFactory("image:$to_expand")(t, e)
			},
			assertCommandFactory: assertCommandFactory("image:expanded"),
		},
		"custom executor set no variable to expand " + ciJobImageEnv: {
			config:               runnerConfig,
			adjustExecutor:       adjustExecutorFactory("image:$nothing_to_expand"),
			assertCommandFactory: assertCommandFactory("image:"),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			defer mockCommandFactory(t, tt)()

			e, options, _ := prepareExecutor(t, tt)
			e.Config = *options.Config
			e.Build = options.Build
			e.Trace = options.Trace
			e.BuildLogger = common.NewBuildLogger(e.Trace, e.Build.Log())
			if tt.adjustExecutor != nil {
				tt.adjustExecutor(t, e)
			}

			err := e.Prepare(options)
			assert.NoError(t, err)

			err = e.Run(common.ExecutorCommand{
				Context: context.Background(),
			})
			assert.NoError(t, err)

			e.Cleanup()
		})
	}
}

func TestExecutor_ServicesEnv(t *testing.T) {
	const CIJobServicesEnv = "CUSTOM_ENV_CI_JOB_SERVICES"

	runnerConfig := getRunnerConfig(&common.CustomConfig{
		RunExec:     "bash",
		PrepareExec: "echo",
		CleanupExec: "bash",
	})

	adjustExecutorServices := func(services common.Services) func(t *testing.T, e *executor) {
		return func(t *testing.T, e *executor) {
			e.Build.Services = services
		}
	}

	assertEnvValue := func(expectedServices []jsonService) func(
		t *testing.T,
		tt executorTestCase,
		ctx context.Context,
		executable string,
		args []string,
		cmdOpts process.CommandOptions,
		options command.Options,
	) {
		return func(
			t *testing.T,
			tt executorTestCase,
			ctx context.Context,
			executable string,
			args []string,
			cmdOpts process.CommandOptions,
			options command.Options,
		) {
			for _, env := range cmdOpts.Env {
				pair := strings.Split(env, "=")
				if pair[0] == CIJobServicesEnv {
					expectedServicesSerialized, _ := json.Marshal(expectedServices)

					assert.Equal(t, string(expectedServicesSerialized), pair[1])
					break
				}
			}
		}
	}

	assertEmptyEnv := func() func(
		t *testing.T,
		tt executorTestCase,
		ctx context.Context,
		executable string,
		args []string,
		cmdOpts process.CommandOptions,
		options command.Options,
	) {
		return func(
			t *testing.T,
			tt executorTestCase,
			ctx context.Context,
			executable string,
			args []string,
			cmdOpts process.CommandOptions,
			options command.Options,
		) {
			for _, env := range cmdOpts.Env {
				pair := strings.Split(env, "=")
				if pair[0] == CIJobServicesEnv {
					assert.Equal(t, "", pair[1])
					break
				}
			}
		}
	}

	tests := map[string]executorTestCase{
		"returns only name when service name is the only definition": {
			config: runnerConfig,
			adjustExecutor: adjustExecutorServices(common.Services{
				{
					Name: "ruby:latest",
				},
			}),
			assertCommandFactory: assertEnvValue(
				[]jsonService{
					{
						Name:       "ruby:latest",
						Alias:      "",
						Entrypoint: nil,
						Command:    nil,
					},
				},
			),
		},
		"returns full service definition": {
			config: runnerConfig,
			adjustExecutor: adjustExecutorServices(common.Services{
				{
					Name:       "ruby:latest",
					Alias:      "henk-ruby",
					Entrypoint: []string{"path", "to", "entrypoint"},
					Command:    []string{"path", "to", "command"},
				},
			}),
			assertCommandFactory: assertEnvValue(
				[]jsonService{
					{
						Name:       "ruby:latest",
						Alias:      "henk-ruby",
						Entrypoint: []string{"path", "to", "entrypoint"},
						Command:    []string{"path", "to", "command"},
					},
				},
			),
		},
		"returns both simple and full service definitions": {
			config: runnerConfig,
			adjustExecutor: adjustExecutorServices(common.Services{
				{
					Name:       "python:latest",
					Alias:      "henk-python",
					Entrypoint: []string{"entrypoint.sh"},
					Command:    []string{"command --test"},
				},
				{
					Name: "python:alpine",
				},
			}),
			assertCommandFactory: assertEnvValue(
				[]jsonService{
					{
						Name:       "python:latest",
						Alias:      "henk-python",
						Entrypoint: []string{"entrypoint.sh"},
						Command:    []string{"command --test"},
					},
					{
						Name:       "python:alpine",
						Alias:      "",
						Entrypoint: nil,
						Command:    nil,
					},
				},
			),
		},
		"does not create env CI_JOB_SERVICES": {
			config:               runnerConfig,
			adjustExecutor:       adjustExecutorServices(common.Services{}),
			assertCommandFactory: assertEmptyEnv(),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			defer mockCommandFactory(t, tt)()

			e, options, _ := prepareExecutor(t, tt)
			e.Config = *options.Config
			e.Build = options.Build
			e.Trace = options.Trace
			e.BuildLogger = common.NewBuildLogger(e.Trace, e.Build.Log())
			if tt.adjustExecutor != nil {
				tt.adjustExecutor(t, e)
			}

			err := e.Prepare(options)
			assert.NoError(t, err)

			err = e.Run(common.ExecutorCommand{
				Context: context.Background(),
			})
			assert.NoError(t, err)

			e.Cleanup()
		})
	}
}

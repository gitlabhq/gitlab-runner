//go:build !integration

package instance

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

// The instance executor suspends the whole instance, so its provider must
// reject a suspending job on a shared instance before acquiring one.
func TestSuspendRequiresDedicatedInstance(t *testing.T) {
	cfg := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			FeatureFlags: map[string]bool{featureflags.SuspendableEnvironments: true},
			Autoscaler:   &common.AutoscalerConfig{MaxUseCount: 0},
			Instance:     &common.InstanceConfig{UseCommonBuildDir: true},
		},
	}
	cfg.ID = 1
	cfg.Autoscaler.StateStorage.Enabled = true

	build := &common.Build{Runner: cfg}
	build.Job.SuspendOptions.SuspendOnSuccess = true

	e := NewProvider("gitlab-runner").Create()
	require.NotNil(t, e)

	err := e.Prepare(common.ExecutorPrepareOptions{
		Config:  cfg,
		Context: t.Context(),
		Build:   build,
	})

	var buildErr *common.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Equal(t, common.ConfigurationError, buildErr.FailureReason)
	assert.Contains(t, err.Error(), "max_use_count = 1")
}

func TestPrepare_UseCommonBuildDir(t *testing.T) {
	const expectedWarning = "use_common_build_dir has no effect: requires vm isolation or max_use_count = 1"

	tests := map[string]struct {
		instanceConfig       *common.InstanceConfig
		autoscalerConfig     *common.AutoscalerConfig
		expectedSharedBuilds bool
		expectedWarning      bool
	}{
		"use_common_build_dir false retains default shared builds dir and emits no warning": {
			instanceConfig: &common.InstanceConfig{
				UseCommonBuildDir: false,
			},
			autoscalerConfig: &common.AutoscalerConfig{
				MaxUseCount: 3,
			},
			expectedSharedBuilds: true,
			expectedWarning:      false,
		},
		"use_common_build_dir with vm isolation disables shared builds dir with no warning": {
			instanceConfig: &common.InstanceConfig{
				UseCommonBuildDir: true,
			},
			autoscalerConfig: &common.AutoscalerConfig{
				VMIsolation: common.VMIsolation{
					Enabled: true,
				},
				MaxUseCount: 3,
			},
			expectedSharedBuilds: false,
			expectedWarning:      false,
		},
		"use_common_build_dir with max_use_count 1 disables shared builds dir with no warning": {
			instanceConfig: &common.InstanceConfig{
				UseCommonBuildDir: true,
			},
			autoscalerConfig: &common.AutoscalerConfig{
				MaxUseCount: 1,
			},
			expectedSharedBuilds: false,
			expectedWarning:      false,
		},
		"use_common_build_dir without vm isolation and max_use_count > 1 emits warning to BuildLogger": {
			instanceConfig: &common.InstanceConfig{
				UseCommonBuildDir: true,
			},
			autoscalerConfig: &common.AutoscalerConfig{
				MaxUseCount: 3,
			},
			expectedSharedBuilds: true,
			expectedWarning:      true,
		},
		"use_common_build_dir without vm isolation and max_use_count 0 emits warning to BuildLogger": {
			instanceConfig: &common.InstanceConfig{
				UseCommonBuildDir: true,
			},
			autoscalerConfig: &common.AutoscalerConfig{
				MaxUseCount: 0,
			},
			expectedSharedBuilds: true,
			expectedWarning:      true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			runnerConfig := &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					BuildsDir:  t.TempDir(),
					CacheDir:   t.TempDir(),
					Executor:   "instance",
					Shell:      "bash",
					Instance:   tt.instanceConfig,
					Autoscaler: tt.autoscalerConfig,
				},
			}

			mockClient := executors.NewMockClient(t)
			mockEnv := executors.NewMockEnvironment(t)
			mockEnv.EXPECT().Prepare(mock.Anything, mock.Anything, mock.Anything).Return(mockClient, nil).Maybe()

			build := &common.Build{
				Job: spec.Job{
					ID: 1,
				},
				Runner:       runnerConfig,
				ExecutorData: mockEnv,
			}

			trace := &captureTrace{}
			logger := buildlogger.New(trace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: executors.ExecutorOptions{
						DefaultBuildsDir: "builds",
						DefaultCacheDir:  "cache",
						SharedBuildsDir:  true,
						Shell: common.ShellScriptInfo{
							Shell:         "bash",
							RunnerCommand: "gitlab-runner",
						},
					},
				},
			}

			options := common.ExecutorPrepareOptions{
				Context:     t.Context(),
				Config:      runnerConfig,
				Build:       build,
				BuildLogger: logger,
			}

			err := e.Prepare(options)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedSharedBuilds, e.SharedBuildsDir)

			if tt.expectedWarning {
				assert.Contains(t, trace.String(), expectedWarning)
			} else {
				assert.NotContains(t, trace.String(), expectedWarning)
			}
		})
	}
}

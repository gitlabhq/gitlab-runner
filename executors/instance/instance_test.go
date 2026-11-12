//go:build !integration

package instance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
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

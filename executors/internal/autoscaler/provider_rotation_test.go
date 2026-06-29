//go:build !integration

package autoscaler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/fleeting/fleeting"
	dummy "gitlab.com/gitlab-org/fleeting/fleeting/plugin/fleeting-plugin-dummy"
	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"gitlab.com/gitlab-org/fleeting/taskscaler"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// dummyFleetingPlugin wraps an in-memory dummy.InstanceGroup so the provider can
// drive a real taskscaler without talking to a cloud provider.
type dummyFleetingPlugin struct {
	group *dummy.InstanceGroup
}

func (p *dummyFleetingPlugin) InstanceGroup() fleetingprovider.InstanceGroup { return p.group }
func (p *dummyFleetingPlugin) Kill()                                         {}

// hasRunningInstance reports whether the instance group has at least one
// instance in the running state.
func hasRunningInstance(group *dummy.InstanceGroup) bool {
	for _, inst := range group.List() {
		if inst.State == fleetingprovider.StateRunning {
			return true
		}
	}
	return false
}

// TestProviderTokenRotationDoesNotPruneInstance is a black-box test that wires
// the real taskscaler to an in-memory fleeting provider (no cloud) and
// reproduces the failure from issue #39380 end to end: a runner acquires an instance (as
// if a job were running on it), its authentication token is rotated in place,
// and the provider must keep using the same taskscaler. A second taskscaler
// would re-discover the running instance as pre-existing and prune it mid-job,
// which is observable here as a Decrease call on the instance group.
func TestProviderTokenRotationDoesNotPruneInstance(t *testing.T) {
	// dummy.InstanceGroup.Update auto-completes Creating->Running transitions,
	// so no separate transition driver goroutine is needed.
	group := &dummy.InstanceGroup{MaxSize: 5}

	ep := common.NewMockExecutorProvider(t)
	p := New(ep, Config{}).(*provider)
	p.taskscalerNew = taskscaler.New // real taskscaler, not a mock
	p.fleetingRunPlugin = func(string, []byte, ...fleeting.PluginOption) (fleetingPlugin, error) {
		return &dummyFleetingPlugin{group: group}, nil
	}

	config := common.NewTestRunnerConfig().
		WithAutoscalerConfig(
			common.NewTestAutoscalerConfig().
				WithPolicies(common.AutoscalerPolicyConfig{IdleCount: 1}).
				AutoscalerConfig,
		).
		WithURL("https://gitlab.example.com").
		WithID(42).
		WithToken("token-before-rotation").
		RunnerConfig
	config.Autoscaler.MaxInstances = 5
	config.Autoscaler.UpdateInterval = 100 * time.Millisecond
	config.Autoscaler.UpdateIntervalWhenExpecting = 100 * time.Millisecond
	config.Autoscaler.StateStorage.Enabled = true
	config.Autoscaler.StateStorage.Dir = t.TempDir()

	// t.Context() is canceled just before t.Cleanup runs, so use a fresh context
	// here to give Shutdown a chance to tear the taskscaler down cleanly.
	t.Cleanup(func() { p.Shutdown(context.Background(), nil) }) //nolint:usetesting // t.Context() is already canceled at Cleanup time; a fresh Background context is required

	// Bring the runner up. Acquire creates the taskscaler and applies the idle
	// policy, which scales the group up to a running instance owned by it.
	// Reserve may report no capacity until that instance finishes provisioning;
	// that's expected here — we only need the idle instance to come up.
	if _, err := p.Acquire(config); err != nil {
		// No free capacity is expected while the idle instance provisions;
		// any other error is a real failure.
		var noCapacity *common.NoFreeExecutorError
		require.ErrorAs(t, err, &noCapacity)
	}
	ts1 := p.getRunnerTaskscaler(config)
	require.NotNil(t, ts1)

	require.Eventually(t, func() bool { return hasRunningInstance(group) }, 10*time.Second, 50*time.Millisecond)
	require.Zero(t, group.CallCount("Decrease"))

	// Rotate the authentication token in place, exactly as resetOneRunnerToken does.
	config.Token = "token-after-rotation"

	// The provider must reuse the existing taskscaler, not create a second one.
	ts2, refresh, err := p.init(config)
	require.NoError(t, err)
	assert.False(t, refresh, "init must reuse the existing scaler after token rotation, not create a second one")
	assert.Same(t, ts1, ts2, "expected the existing taskscaler to be reused after token rotation")

	// The running instance must never be pruned. Under the issue #39380 bug a second taskscaler
	// would discover it as pre-existing and call Decrease within a few cycles.
	assert.Never(t, func() bool { return group.CallCount("Decrease") > 0 }, 2*time.Second, 100*time.Millisecond)
}

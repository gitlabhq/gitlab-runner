//go:build !integration

package autoscaler

import (
	"context"
	"errors"
	"net"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/fleeting/fleeting"
	"gitlab.com/gitlab-org/fleeting/fleeting/connector"
	fleetingmocks "gitlab.com/gitlab-org/fleeting/fleeting/connector/mocks"
	dummy "gitlab.com/gitlab-org/fleeting/fleeting/plugin/fleeting-plugin-dummy"
	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"gitlab.com/gitlab-org/fleeting/taskscaler"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
)

// TestPrepare_UnavailableInstanceIsRetried is a black-box test for issue #39145.
// It wires the real taskscaler to an in-memory fleeting provider with two idle
// instances, makes the instance picked by the first attempt unusable, and drives
// Prepare through the same retry loop as Build.retryCreateExecutor. The job must
// end up prepared on the other instance.
//
// Original test by Cam Swords (note 3962726590); adapted to use a real
// *net.OpError{Op:"dial"} for the unreachable-host case so it hits the
// instance-failure detection path in classifyDialError.
func TestPrepare_UnavailableInstanceIsRetried(t *testing.T) {
	// connRefusedErr is the error shape fleeting's net.Dialer returns on
	// connection refused: *net.OpError{Op:"dial"} wraps syscall.ECONNREFUSED.
	connRefusedErr := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}

	tests := map[string]struct {
		// failDial is called for every dial to the first instance the job lands
		// on, and returns the error that dial fails with.
		failDial func(ctx context.Context, group *dummy.InstanceGroup, id string, first bool) error
	}{
		"instance is externally removed while dialing": {
			failDial: func(ctx context.Context, group *dummy.InstanceGroup, id string, first bool) error {
				if !first {
					// Not the bad instance — return a plain refused to keep the
					// test fast; the good-instance dial returns a mock client.
					return connRefusedErr
				}

				// Simulate the cloud provider terminating the VM mid-dial, and
				// wait for taskscaler to notice and cancel the acquisition.
				group.RemoveInstance(id)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(10 * time.Second):
					return errors.New("taskscaler did not cancel the acquisition of the removed instance")
				}
			},
		},
		"instance is unreachable": {
			failDial: func(context.Context, *dummy.InstanceGroup, string, bool) error {
				return connRefusedErr
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			group := &dummy.InstanceGroup{MaxSize: 2}
			p, config := newBlackBoxProvider(t, group, 2)

			acqRef := acquireWhenIdleInstances(t, p, group, config)
			t.Cleanup(func() { p.Release(config, acqRef) })

			var (
				mu     sync.Mutex
				dialed []string
				bad    string
			)
			acqRef.dialAcquisitionInstance = func(ctx context.Context, _ fleetingprovider.ConnectInfo, _ connector.DialOptions) (connector.Client, error) {
				id := acqRef.acq.InstanceID()

				mu.Lock()
				dialed = append(dialed, id)
				first := bad == ""
				if first {
					bad = id
				}
				isBad := id == bad
				mu.Unlock()

				if isBad {
					return nil, tc.failDial(ctx, group, id, first)
				}
				return fleetingmocks.NewClient(t), nil
			}

			logger, _ := test.NewNullLogger()
			bl := buildlogger.New(nil, logrus.NewEntry(logger), buildlogger.Options{})

			// The inner executor prepares on the acquisition, as the docker
			// executor does through executors.Environment.
			inner := common.NewMockExecutor(t)
			inner.EXPECT().Prepare(mock.Anything).RunAndReturn(func(opts common.ExecutorPrepareOptions) error {
				_, err := acqRef.Prepare(opts.Context, bl, opts)
				return err
			})

			build := &common.Build{Runner: config, ExecutorData: acqRef}
			opts := common.ExecutorPrepareOptions{Config: config, Context: t.Context(), Build: build}

			attempts, err := prepareWithRetries(p, inner, opts)

			mu.Lock()
			defer mu.Unlock()
			t.Logf("attempts=%d dialed=%v bad=%s", attempts, dialed, bad)

			require.NoError(t, err, "job must be prepared on another instance")
			require.NotNil(t, acqRef.acq)
			assert.NotEqual(t, bad, acqRef.acq.InstanceID(), "job must not stay on the unavailable instance")
		})
	}
}

// prepareWithRetries mirrors Build.retryCreateExecutor, minus the retry
// interval: a BuildError ends preparation, any other error is retried with a
// new executor sharing the same acquisitionRef.
func prepareWithRetries(p *provider, inner common.Executor, opts common.ExecutorPrepareOptions) (int, error) {
	var err error
	for attempt := 1; attempt <= common.PreparationRetries; attempt++ {
		err = (&executor{Executor: inner, provider: p}).Prepare(opts)
		if err == nil {
			return attempt, nil
		}

		if _, ok := errors.AsType[*common.BuildError](err); ok {
			return attempt, err
		}
	}

	return common.PreparationRetries, err
}

// newBlackBoxProvider returns a provider driving a real taskscaler over the
// dummy instance group, with idleCount instances kept idle.
func newBlackBoxProvider(t *testing.T, group *dummy.InstanceGroup, idleCount int) (*provider, *common.RunnerConfig) {
	t.Helper()

	p := New(common.NewMockExecutorProvider(t), Config{}).(*provider)
	p.taskscalerNew = taskscaler.New
	p.fleetingRunPlugin = func(string, []byte, ...fleeting.PluginOption) (fleetingPlugin, error) {
		return &dummyFleetingPlugin{group: group}, nil
	}

	config := common.NewTestRunnerConfig().
		WithAutoscalerConfig(
			common.NewTestAutoscalerConfig().
				WithPolicies(common.AutoscalerPolicyConfig{IdleCount: idleCount}).
				AutoscalerConfig,
		).
		WithURL("https://gitlab.example.com").
		WithID(42).
		WithToken("glrt-test-token").
		RunnerConfig
	config.Autoscaler.MaxInstances = group.MaxSize
	config.Autoscaler.UpdateInterval = 100 * time.Millisecond
	config.Autoscaler.UpdateIntervalWhenExpecting = 100 * time.Millisecond

	// t.Context() is canceled just before t.Cleanup runs, so use a fresh context.
	t.Cleanup(func() { p.Shutdown(context.Background(), nil) }) //nolint:usetesting // t.Context() is already canceled at Cleanup time

	return p, config
}

// acquireWhenIdleInstances reserves capacity once every idle instance is
// running, so the first Acquire has more than one instance to pick from.
func acquireWhenIdleInstances(t *testing.T, p *provider, group *dummy.InstanceGroup, config *common.RunnerConfig) *acquisitionRef {
	t.Helper()

	var data common.ExecutorData
	acquire := func() bool {
		var err error
		data, err = p.Acquire(config)
		if err != nil {
			var noCapacity *common.NoFreeExecutorError
			require.ErrorAs(t, err, &noCapacity)
			return false
		}
		return true
	}

	// The first Acquire starts the taskscaler, which scales up to the idle count.
	if !acquire() {
		require.Eventually(t, func() bool {
			running := 0
			for _, inst := range group.List() {
				if inst.State == fleetingprovider.StateRunning {
					running++
				}
			}
			return running == group.MaxSize
		}, 10*time.Second, 50*time.Millisecond)

		require.Eventually(t, acquire, 10*time.Second, 50*time.Millisecond)
	}

	acqRef, ok := data.(*acquisitionRef)
	require.True(t, ok)

	return acqRef
}

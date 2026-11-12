//go:build !integration

package autoscaler

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/fleeting/fleeting/connector"
	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"gitlab.com/gitlab-org/fleeting/taskscaler"
	"gitlab.com/gitlab-org/fleeting/taskscaler/mocks"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

type releasingExecutorProvider struct {
	*common.MockExecutorProvider
	release func(key string) error
}

func (p releasingExecutorProvider) ReleaseEnvironment(_ context.Context, _ *common.RunnerConfig, conn executors.Client, key string) error {
	if conn == nil {
		return errors.New("no connection")
	}
	return p.release(key)
}

type fakeConnectorClient struct{ closed bool }

func (c *fakeConnectorClient) Run(context.Context, connector.RunOptions) error { return nil }
func (c *fakeConnectorClient) Dial(string, string) (net.Conn, error)           { return nil, nil }
func (c *fakeConnectorClient) DialRun(context.Context, string) (net.Conn, error) {
	return nil, nil
}
func (c *fakeConnectorClient) Close() error {
	c.closed = true
	return nil
}

func sweepTestConfig(stateStorage, vmIsolation bool) *common.RunnerConfig {
	autoscalerCfg := common.NewTestAutoscalerConfig().AutoscalerConfig
	autoscalerCfg.StateStorage.Enabled = stateStorage
	autoscalerCfg.VMIsolation.Enabled = vmIsolation

	return common.NewTestRunnerConfig().
		WithAutoscalerConfig(autoscalerCfg).
		WithToken("sweep").
		RunnerConfig
}

func TestSweepRuntimeEnvironments(t *testing.T) {
	now := time.Now()
	errRelease := errors.New("release failed")

	tests := map[string]struct {
		slots        []taskscaler.ExpiredSlot
		vmIsolation  bool
		noReleaser   bool
		connectErr   error
		dialErr      error
		releaseErr   error
		wantReleased []string
		wantFreed    []string
		wantErr      error
	}{
		"nothing expired": {},
		"releases and frees a slot on a running instance": {
			slots:        []taskscaler.ExpiredSlot{{Key: "key-1", InstanceID: "inst-1"}},
			wantReleased: []string{"key-1"},
			wantFreed:    []string{"key-1"},
		},
		"skips a slot on a stopped instance": {
			slots: []taskscaler.ExpiredSlot{{Key: "key-1", InstanceStopped: true}},
		},
		"skips a slot whose instance stopped since expiry": {
			slots:      []taskscaler.ExpiredSlot{{Key: "key-1"}},
			connectErr: fmt.Errorf("wrapped: %w", taskscaler.ErrInstanceStopped),
		},
		"keeps a slot whose connect info fails": {
			slots:      []taskscaler.ExpiredSlot{{Key: "key-1"}},
			connectErr: errRelease,
			wantErr:    errRelease,
		},
		"keeps a slot whose instance cannot be dialed": {
			slots:   []taskscaler.ExpiredSlot{{Key: "key-1"}},
			dialErr: errRelease,
			wantErr: errRelease,
		},
		"keeps a slot whose release fails": {
			slots:        []taskscaler.ExpiredSlot{{Key: "fails"}},
			releaseErr:   errRelease,
			wantReleased: []string{"fails"},
			wantErr:      errRelease,
		},
		"frees a slot whose environment is already gone": {
			slots:        []taskscaler.ExpiredSlot{{Key: "fails"}},
			releaseErr:   fmt.Errorf("wrapped: %w", ErrEnvironmentNotFound),
			wantReleased: []string{"fails"},
			wantFreed:    []string{"fails"},
		},
		"frees without connecting under vm isolation": {
			slots:       []taskscaler.ExpiredSlot{{Key: "key-1"}},
			vmIsolation: true,
			wantFreed:   []string{"key-1"},
		},
		"frees without connecting for an executor without a releaser": {
			slots:      []taskscaler.ExpiredSlot{{Key: "key-1"}},
			noReleaser: true,
			wantFreed:  []string{"key-1"},
		},
		"continues past a failed slot": {
			slots:        []taskscaler.ExpiredSlot{{Key: "fails"}, {Key: "key-2"}},
			releaseErr:   errRelease, // only for "fails"
			wantReleased: []string{"fails", "key-2"},
			wantFreed:    []string{"key-2"},
			wantErr:      errRelease,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := sweepTestConfig(true, tt.vmIsolation)

			var released, freed []string
			var ep common.ExecutorProvider = releasingExecutorProvider{
				MockExecutorProvider: common.NewMockExecutorProvider(t),
				release: func(key string) error {
					released = append(released, key)
					// releaseErr applies only to the slot keyed "fails"
					if key == "fails" {
						return tt.releaseErr
					}
					return nil
				},
			}
			if tt.noReleaser {
				ep = common.NewMockExecutorProvider(t)
			}

			ts := mocks.NewTaskscaler(t)
			ts.EXPECT().ExpireSuspended(now, common.RuntimeEnvironmentSweepGrace).Return(tt.slots)
			ts.EXPECT().ExpiredConnectInfo(mock.Anything, mock.Anything).
				Return(fleetingprovider.ConnectInfo{}, tt.connectErr).Maybe()
			ts.EXPECT().FreeExpired(mock.Anything).RunAndReturn(func(key string) error {
				freed = append(freed, key)
				return nil
			}).Maybe()

			p := New(ep, Config{}).(*provider)
			p.scalers[runnerScalerKey(cfg)] = scaler{internal: ts, shutdown: func(context.Context) {}, configLoadedAt: cfg.ConfigLoadedAt}

			var dialed []*fakeConnectorClient
			p.dialInstance = func(context.Context, fleetingprovider.ConnectInfo, connector.DialOptions) (connector.Client, error) {
				if tt.dialErr != nil {
					return nil, tt.dialErr
				}
				c := &fakeConnectorClient{}
				dialed = append(dialed, c)
				return c, nil
			}

			err := p.SweepRuntimeEnvironments(t.Context(), cfg, now)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantReleased, released)
			assert.Equal(t, tt.wantFreed, freed)
			for _, c := range dialed {
				assert.True(t, c.closed, "instance connection must be closed")
			}
		})
	}
}

func TestSweepRuntimeEnvironmentsContinuesPastFailedFree(t *testing.T) {
	cfg := sweepTestConfig(true, false)
	now := time.Now()
	errFree := errors.New("free failed")

	var released, freed []string
	ep := releasingExecutorProvider{
		MockExecutorProvider: common.NewMockExecutorProvider(t),
		release: func(key string) error {
			released = append(released, key)
			return nil
		},
	}

	ts := mocks.NewTaskscaler(t)
	ts.EXPECT().ExpireSuspended(now, common.RuntimeEnvironmentSweepGrace).
		Return([]taskscaler.ExpiredSlot{{Key: "fails"}, {Key: "key-2"}})
	ts.EXPECT().ExpiredConnectInfo(mock.Anything, mock.Anything).Return(fleetingprovider.ConnectInfo{}, nil)
	ts.EXPECT().FreeExpired(mock.Anything).RunAndReturn(func(key string) error {
		freed = append(freed, key)
		if key == "fails" {
			return errFree
		}
		return nil
	})

	p := New(ep, Config{}).(*provider)
	p.scalers[runnerScalerKey(cfg)] = scaler{internal: ts, shutdown: func(context.Context) {}, configLoadedAt: cfg.ConfigLoadedAt}
	p.dialInstance = func(context.Context, fleetingprovider.ConnectInfo, connector.DialOptions) (connector.Client, error) {
		return &fakeConnectorClient{}, nil
	}

	err := p.SweepRuntimeEnvironments(t.Context(), cfg, now)
	require.ErrorIs(t, err, errFree)
	assert.Equal(t, []string{"fails", "key-2"}, released)
	assert.Equal(t, []string{"fails", "key-2"}, freed)
}

func TestSweepRuntimeEnvironmentsTimesOutEachRelease(t *testing.T) {
	orig := expiredReleaseTimeout
	expiredReleaseTimeout = 100 * time.Millisecond
	t.Cleanup(func() { expiredReleaseTimeout = orig })

	cfg := sweepTestConfig(true, false)
	now := time.Now()

	var released, freed []string
	ep := releasingExecutorProvider{
		MockExecutorProvider: common.NewMockExecutorProvider(t),
		release: func(key string) error {
			released = append(released, key)
			return nil
		},
	}

	ts := mocks.NewTaskscaler(t)
	ts.EXPECT().ExpireSuspended(now, common.RuntimeEnvironmentSweepGrace).
		Return([]taskscaler.ExpiredSlot{{Key: "hangs", InstanceID: "inst-1"}, {Key: "key-2", InstanceID: "inst-2"}})
	ts.EXPECT().ExpiredConnectInfo(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, key string) (fleetingprovider.ConnectInfo, error) {
			if key == "hangs" {
				<-ctx.Done()
				return fleetingprovider.ConnectInfo{}, ctx.Err()
			}
			return fleetingprovider.ConnectInfo{}, nil
		})
	ts.EXPECT().FreeExpired(mock.Anything).RunAndReturn(func(key string) error {
		freed = append(freed, key)
		return nil
	})

	p := New(ep, Config{}).(*provider)
	p.scalers[runnerScalerKey(cfg)] = scaler{internal: ts, shutdown: func(context.Context) {}, configLoadedAt: cfg.ConfigLoadedAt}
	p.dialInstance = func(context.Context, fleetingprovider.ConnectInfo, connector.DialOptions) (connector.Client, error) {
		return &fakeConnectorClient{}, nil
	}

	// The sweep context outlives both slots' timeouts, so only the per-slot
	// timeout can end the hung release.
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	err := p.SweepRuntimeEnvironments(ctx, cfg, now)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NoError(t, ctx.Err(), "the sweep context must not have expired")
	assert.Equal(t, []string{"key-2"}, released)
	assert.Equal(t, []string{"key-2"}, freed)
}

func TestSweepRuntimeEnvironmentsSkipsTimedOutInstance(t *testing.T) {
	orig := expiredReleaseTimeout
	expiredReleaseTimeout = 100 * time.Millisecond
	t.Cleanup(func() { expiredReleaseTimeout = orig })

	cfg := sweepTestConfig(true, false)
	now := time.Now()

	var released, connected []string
	ep := releasingExecutorProvider{
		MockExecutorProvider: common.NewMockExecutorProvider(t),
		release: func(key string) error {
			released = append(released, key)
			return nil
		},
	}

	ts := mocks.NewTaskscaler(t)
	ts.EXPECT().ExpireSuspended(now, common.RuntimeEnvironmentSweepGrace).
		Return([]taskscaler.ExpiredSlot{
			{Key: "hangs", InstanceID: "inst-1"},
			{Key: "same-instance", InstanceID: "inst-1"},
			{Key: "key-3", InstanceID: "inst-2"},
		})
	ts.EXPECT().ExpiredConnectInfo(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, key string) (fleetingprovider.ConnectInfo, error) {
			connected = append(connected, key)
			if key == "hangs" {
				<-ctx.Done()
				return fleetingprovider.ConnectInfo{}, ctx.Err()
			}
			return fleetingprovider.ConnectInfo{}, nil
		})
	ts.EXPECT().FreeExpired("key-3").Return(nil)

	p := New(ep, Config{}).(*provider)
	p.scalers[runnerScalerKey(cfg)] = scaler{internal: ts, shutdown: func(context.Context) {}, configLoadedAt: cfg.ConfigLoadedAt}
	p.dialInstance = func(context.Context, fleetingprovider.ConnectInfo, connector.DialOptions) (connector.Client, error) {
		return &fakeConnectorClient{}, nil
	}

	err := p.SweepRuntimeEnvironments(t.Context(), cfg, now)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, []string{"hangs", "key-3"}, connected, "the timed-out instance's other slot must wait for the next sweep")
	assert.Equal(t, []string{"key-3"}, released)
}

func TestSweepRuntimeEnvironmentsStopsWhenContextEnds(t *testing.T) {
	cfg := sweepTestConfig(true, false)
	now := time.Now()

	ts := mocks.NewTaskscaler(t)
	ts.EXPECT().ExpireSuspended(now, common.RuntimeEnvironmentSweepGrace).
		Return([]taskscaler.ExpiredSlot{{Key: "key-1", InstanceID: "inst-1"}})

	p := New(common.NewMockExecutorProvider(t), Config{}).(*provider)
	p.scalers[runnerScalerKey(cfg)] = scaler{internal: ts, shutdown: func(context.Context) {}, configLoadedAt: cfg.ConfigLoadedAt}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	require.ErrorIs(t, p.SweepRuntimeEnvironments(ctx, cfg, now), context.Canceled)
}

func TestSweepRuntimeEnvironmentsWithoutStateStorage(t *testing.T) {
	p := New(common.NewMockExecutorProvider(t), Config{}).(*provider)

	require.NoError(t, p.SweepRuntimeEnvironments(t.Context(), sweepTestConfig(false, false), time.Now()))
	assert.Empty(t, p.scalers, "the taskscaler must not be started")
}

func TestSweepRuntimeEnvironmentsStartsTaskscaler(t *testing.T) {
	cfg := sweepTestConfig(true, false)
	cfg.Autoscaler.StateStorage.Dir = t.TempDir()
	now := time.Now()

	ts := mocks.NewTaskscaler(t)
	ts.EXPECT().ConfigureSchedule().Return(nil)
	ts.EXPECT().ExpireSuspended(now, common.RuntimeEnvironmentSweepGrace).Return(nil)

	p := New(common.NewMockExecutorProvider(t), Config{}).(*provider)
	p.taskscalerNew = mockTaskscalerNew(ts, false)
	p.fleetingRunPlugin = mockFleetingRunPlugin(false)

	require.NoError(t, p.SweepRuntimeEnvironments(t.Context(), cfg, now))
	assert.Contains(t, p.scalers, runnerScalerKey(cfg))
}

//go:build !integration

package autoscaler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/fleeting/fleeting"
	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"gitlab.com/gitlab-org/fleeting/taskscaler"
	"gitlab.com/gitlab-org/fleeting/taskscaler/mocks"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestInit(t *testing.T) {
	tokenTaskscaler := mocks.NewTaskscaler(t) // for comparing by memory address

	tests := map[string]struct {
		scalers              map[string]taskscaler.Taskscaler
		config               *common.RunnerConfig
		newTaskscalerErr     bool
		fleetingRunPluginErr bool
		wantTaskscaler       taskscaler.Taskscaler
		wantCreated          bool
		wantErr              bool
		refreshConfig        bool
	}{
		"nil autoscaler config return error": {
			config:  common.NewTestRunnerConfig().RunnerConfig,
			wantErr: true,
		},
		"taskscaler exists and is not created": {
			config: common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().AutoscalerConfig,
				).
				RunnerConfig,
			scalers: map[string]taskscaler.Taskscaler{
				"": tokenTaskscaler,
			},
			wantTaskscaler: tokenTaskscaler,
			wantCreated:    false,
		},
		"taskscaler does not exist and is created": {
			config: common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().AutoscalerConfig,
				).
				RunnerConfig,
			wantTaskscaler: tokenTaskscaler,
			wantCreated:    true,
		},
		"error creating fleeting plugin": {
			config: common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().AutoscalerConfig,
				).
				RunnerConfig,
			fleetingRunPluginErr: true,
			wantErr:              true,
		},
		"error creating new taskscaler": {
			config: common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().AutoscalerConfig,
				).
				RunnerConfig,
			newTaskscalerErr: true,
			wantErr:          true,
		},
		"returns correct taskscaler": {
			config: common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().AutoscalerConfig,
				).
				WithToken("right").
				RunnerConfig,
			scalers: map[string]taskscaler.Taskscaler{
				"wrong": nil,
				"":      nil, // also wrong
				"right": tokenTaskscaler,
			},
			wantTaskscaler: tokenTaskscaler,
			wantCreated:    false,
		},
		"scaler lookup is keyed by URL+ID not token": {
			// The scaler is seeded under URL+ID. A config carrying a different
			// token still resolves to it, proving the key ignores the token.
			// (Rotation over time is covered by TestInitReusesScalerOnTokenRotation.)
			config: common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().AutoscalerConfig,
				).
				WithURL("https://gitlab.example.com").
				WithID(42).
				WithToken("some-token").
				RunnerConfig,
			scalers: map[string]taskscaler.Taskscaler{
				"https://gitlab.example.com|42": tokenTaskscaler,
			},
			wantTaskscaler: tokenTaskscaler,
			wantCreated:    false,
		},
		"scaler lookup falls back to token when ID is unset": {
			// Legacy/hand-written configs (pre-v15.3.0, before the id field
			// existed) default to with ID==0. Token rotation never backfills
			// the ID, so the key falls back to the token, preserving pre-fix
			// behavior rather than colliding every zero-ID runner under URL|0.
			config: common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().AutoscalerConfig,
				).
				WithURL("https://gitlab.example.com").
				WithToken("some-token").
				RunnerConfig,
			scalers: map[string]taskscaler.Taskscaler{
				"some-token": tokenTaskscaler,
			},
			wantTaskscaler: tokenTaskscaler,
			wantCreated:    false,
		},
		"detect refresh on config change": {
			config: common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().AutoscalerConfig,
				).
				WithToken("runner").
				RunnerConfig,
			scalers: map[string]taskscaler.Taskscaler{
				"runner": tokenTaskscaler,
			},
			wantTaskscaler: tokenTaskscaler,
			wantCreated:    false,
			refreshConfig:  true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ep := common.NewMockExecutorProvider(t)
			p := New(ep, Config{}).(*provider)
			p.taskscalerNew = mockTaskscalerNew(tokenTaskscaler, tt.newTaskscalerErr)
			p.fleetingRunPlugin = mockFleetingRunPlugin(tt.fleetingRunPluginErr)
			for k, v := range tt.scalers {
				p.scalers[k] = scaler{
					internal: v,
					shutdown: func(_ context.Context) {},
				}
			}

			ts, created, err := p.init(tt.config)

			assert.Equal(t, tt.wantTaskscaler, ts)
			assert.Equal(t, tt.wantCreated, created)
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			if tt.refreshConfig {
				_, created, _ = p.init(tt.config)
				assert.False(t, created)

				tt.config.ConfigLoadedAt = time.Now()

				_, created, _ = p.init(tt.config)
				assert.True(t, created)
			}
		})
	}
}

// TestInitReusesScalerOnTokenRotation models the bug from issue #39380: a single runner is
// initialized, then its authentication token is rotated in place (as
// resetOneRunnerToken does), and init is called again. The existing scaler must
// be reused rather than a second one created for the same instance group, since
// a second scaler would discover the running instances as pre-existing and prune
// them mid-job.
func TestInitReusesScalerOnTokenRotation(t *testing.T) {
	ts := mocks.NewTaskscaler(t)
	ep := common.NewMockExecutorProvider(t)

	p := New(ep, Config{}).(*provider)
	p.fleetingRunPlugin = mockFleetingRunPlugin(false)

	// Count how many taskscalers get created across both init calls.
	var created int
	newTaskscaler := mockTaskscalerNew(ts, false)
	p.taskscalerNew = func(ctx context.Context, ig fleetingprovider.InstanceGroup, opts ...taskscaler.Option) (taskscaler.Taskscaler, error) {
		created++
		return newTaskscaler(ctx, ig, opts...)
	}

	config := common.NewTestRunnerConfig().
		WithAutoscalerConfig(common.NewTestAutoscalerConfig().AutoscalerConfig).
		WithURL("https://gitlab.example.com").
		WithID(42).
		WithToken("token-before-rotation").
		RunnerConfig

	// First init: scaler is created.
	got, wasCreated, err := p.init(config)
	assert.NoError(t, err)
	assert.True(t, wasCreated)
	assert.Same(t, ts, got)
	assert.Equal(t, 1, created)

	// Rotate the token in place, exactly as resetOneRunnerToken mutates it.
	config.Token = "token-after-rotation"

	// Second init: the same scaler is reused, none created.
	got, wasCreated, err = p.init(config)
	assert.NoError(t, err)
	assert.False(t, wasCreated)
	assert.Same(t, ts, got)
	assert.Equal(t, 1, created)
}

func TestAcquire(t *testing.T) {
	const (
		afterInit      = "Init"
		afterConfigure = "Configure"
		afterReserve   = "Reserve"
	)

	tests := map[string]struct {
		idleCount          int
		PreemptiveMode     bool
		canReserve         bool
		wantEarlyReturn    string
		wantAcquisitionRef bool
		wantErr            bool
	}{
		"failed init": {
			wantEarlyReturn: afterInit,
			wantErr:         true,
		},
		"failed configure schedule": {
			wantEarlyReturn: afterConfigure,
			wantErr:         true,
		},
		"has capacity, no idle count": {
			idleCount:          0,
			canReserve:         true,
			wantAcquisitionRef: true,
		},
		"preemptiveMode false": {
			canReserve:         true,
			PreemptiveMode:     false,
			wantAcquisitionRef: true,
		},
		"preemptiveMode true": {
			canReserve:         true,
			PreemptiveMode:     true,
			wantAcquisitionRef: true,
		},
		"no capacity, no idle count": {
			idleCount:       0,
			canReserve:      false,
			wantEarlyReturn: afterReserve,
			wantErr:         true,
		},
		"has capacity, has idle count": {
			idleCount:          1,
			canReserve:         true,
			wantAcquisitionRef: true,
		},
		"no capacity, has idle count": {
			idleCount:       1,
			canReserve:      false,
			wantEarlyReturn: afterReserve,
			wantErr:         true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			const acqRefKey = "abcdefgh"

			config := common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().
						WithPolicies(
							common.AutoscalerPolicyConfig{
								IdleCount:      tt.idleCount,
								PreemptiveMode: &tt.PreemptiveMode,
							},
						).AutoscalerConfig,
				).RunnerConfig
			schedule := taskscaler.Schedule{
				IdleCount:      tt.idleCount,
				PreemptiveMode: tt.PreemptiveMode,
			}
			ts := mocks.NewTaskscaler(t)
			ep := common.NewMockExecutorProvider(t)
			p := New(ep, Config{}).(*provider)
			p.taskscalerNew = mockTaskscalerNew(ts /* wantErr */, false)
			p.fleetingRunPlugin = mockFleetingRunPlugin( /* wantErr */ false)
			p.generateUniqueID = func() (string, error) {
				return acqRefKey, nil
			}

			switch tt.wantEarlyReturn {
			case afterInit:
				// Init fails to create new taskscaler.
				p.taskscalerNew = mockTaskscalerNew(nil /* wantErr */, true)
			case afterConfigure:
				ts.EXPECT().ConfigureSchedule([]taskscaler.Schedule{schedule}).Return(fmt.Errorf("test error"))
			case afterReserve:
				ts.EXPECT().ConfigureSchedule([]taskscaler.Schedule{schedule}).Return(nil)
				ts.EXPECT().Reserve(acqRefKey).Return(taskscaler.ErrNoCapacity)
			default:
				ts.EXPECT().ConfigureSchedule([]taskscaler.Schedule{schedule}).Return(nil)
				ts.EXPECT().Reserve(acqRefKey).Return(nil)
			}

			ar, err := p.Acquire(config)

			if tt.wantAcquisitionRef {
				if assert.IsType(t, &acquisitionRef{}, ar) {
					assert.Equal(t, acqRefKey, ar.(*acquisitionRef).key)
				}
			} else {
				assert.Nil(t, ar)
			}
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func mockTaskscalerNew(
	newTaskscaler taskscaler.Taskscaler,
	newTaskscalerErr bool,
) func(context.Context, fleetingprovider.InstanceGroup, ...taskscaler.Option) (taskscaler.Taskscaler, error) {
	return func(context.Context, fleetingprovider.InstanceGroup, ...taskscaler.Option) (taskscaler.Taskscaler, error) {
		if newTaskscalerErr {
			return nil, fmt.Errorf("test error")
		}
		return newTaskscaler, nil
	}
}

func mockFleetingRunPlugin(wantErr bool) func(string, []byte, ...fleeting.PluginOption) (fleetingPlugin, error) {
	return func(string, []byte, ...fleeting.PluginOption) (fleetingPlugin, error) {
		if wantErr {
			return nil, fmt.Errorf("test error")
		}

		return new(fakeFleetingPlugin), nil
	}
}

type fakeFleetingPlugin struct{}

func (f *fakeFleetingPlugin) InstanceGroup() fleetingprovider.InstanceGroup { return nil }

func (f *fakeFleetingPlugin) Kill() {}

func TestProvider_ScalerFor_AppliesReloadedPolicyOnce(t *testing.T) {
	cfg := sweepTestConfig(true, false)
	ts := mocks.NewTaskscaler(t)

	p := New(common.NewMockExecutorProvider(t), Config{}).(*provider)
	p.scalers[runnerScalerKey(cfg)] = scaler{
		internal:       ts,
		shutdown:       func(context.Context) {},
		configLoadedAt: cfg.ConfigLoadedAt,
	}

	_, err := p.scalerFor(cfg)
	require.NoError(t, err, "config unchanged: policy not applied")

	cfg.ConfigLoadedAt = cfg.ConfigLoadedAt.Add(time.Minute)

	ts.EXPECT().ConfigureSchedule(mock.Anything).Return(assert.AnError).Once()
	_, err = p.scalerFor(cfg)
	require.ErrorIs(t, err, assert.AnError)

	ts.EXPECT().ConfigureSchedule(mock.Anything).Return(nil).Once()
	_, err = p.scalerFor(cfg)
	require.NoError(t, err, "a failed apply is retried")

	_, err = p.scalerFor(cfg)
	require.NoError(t, err, "an applied reload is not applied again")
}

func TestProvider_ScalerFor_RetriesFailedFirstApply(t *testing.T) {
	cfg := sweepTestConfig(true, false)
	cfg.Autoscaler.StateStorage.Dir = t.TempDir()
	cfg.ConfigLoadedAt = time.Now()

	ts := mocks.NewTaskscaler(t)
	ts.EXPECT().ConfigureSchedule(mock.Anything).Return(assert.AnError).Once()
	ts.EXPECT().ConfigureSchedule(mock.Anything).Return(nil).Once()

	p := New(common.NewMockExecutorProvider(t), Config{}).(*provider)
	p.taskscalerNew = mockTaskscalerNew(ts, false)
	p.fleetingRunPlugin = mockFleetingRunPlugin(false)

	_, err := p.scalerFor(cfg)
	require.ErrorIs(t, err, assert.AnError)

	_, err = p.scalerFor(cfg)
	require.NoError(t, err, "a failed first apply is retried")

	_, err = p.scalerFor(cfg)
	require.NoError(t, err, "an applied config is not applied again")
}

func TestProvider_CollectWhileAddingScalers(t *testing.T) {
	ts := mocks.NewTaskscaler(t)
	ts.EXPECT().ConfigureSchedule(mock.Anything).Return(nil).Maybe()
	ts.EXPECT().MetricsCollector().Return(nil).Maybe()
	ts.EXPECT().FleetingMetricsCollector().Return(nil).Maybe()

	p := New(common.NewMockExecutorProvider(t), Config{}).(*provider)
	p.taskscalerNew = mockTaskscalerNew(ts, false)
	p.fleetingRunPlugin = mockFleetingRunPlugin(false)
	dir := t.TempDir()

	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ch := make(chan prometheus.Metric, 1)
		for {
			select {
			case <-done:
				return
			default:
				p.Collect(ch)
			}
		}
	}()

	for i := 1; i <= 50; i++ {
		cfg := sweepTestConfig(true, false)
		cfg.ID = int64(i)
		cfg.Autoscaler.StateStorage.Dir = dir
		_, err := p.scalerFor(cfg)
		require.NoError(t, err)
	}

	close(done)
	<-stopped
}

func TestProvider_CollectDoesNotWaitForInit(t *testing.T) {
	ts := mocks.NewTaskscaler(t)
	ts.EXPECT().ConfigureSchedule(mock.Anything).Return(nil).Maybe()
	ts.EXPECT().MetricsCollector().Return(nil).Maybe()
	ts.EXPECT().FleetingMetricsCollector().Return(nil).Maybe()

	started := make(chan struct{})
	unblock := make(chan struct{})
	p := New(common.NewMockExecutorProvider(t), Config{}).(*provider)
	p.fleetingRunPlugin = mockFleetingRunPlugin(false)
	p.taskscalerNew = func(context.Context, fleetingprovider.InstanceGroup, ...taskscaler.Option) (taskscaler.Taskscaler, error) {
		close(started)
		<-unblock
		return ts, nil
	}

	cfg := sweepTestConfig(true, false)
	cfg.Autoscaler.StateStorage.Dir = t.TempDir()

	initDone := make(chan error, 1)
	go func() {
		_, err := p.scalerFor(cfg)
		initDone <- err
	}()
	<-started

	collected := make(chan struct{})
	go func() {
		defer close(collected)
		p.Collect(make(chan prometheus.Metric, 1))
		p.Describe(make(chan *prometheus.Desc, 1))
	}()

	select {
	case <-collected:
	case <-time.After(time.Second):
		t.Error("metrics collection waited for the taskscaler to start")
	}

	close(unblock)
	require.NoError(t, <-initDone)
	<-collected
}

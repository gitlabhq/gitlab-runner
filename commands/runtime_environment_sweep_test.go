//go:build !integration

package commands

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

type sweepingProvider struct {
	*common.MockExecutorProvider
	*common.MockRuntimeEnvironmentSweeper
}

func newSweepingProvider(t *testing.T) *sweepingProvider {
	return &sweepingProvider{
		MockExecutorProvider:          common.NewMockExecutorProvider(t),
		MockRuntimeEnvironmentSweeper: common.NewMockRuntimeEnvironmentSweeper(t),
	}
}

func sweepRunner(name, executor string, id int64, ffOn bool, ttl *time.Duration) *common.RunnerConfig {
	return &common.RunnerConfig{
		Name: name,
		RunnerCredentials: common.RunnerCredentials{
			ID:    id,
			Token: "glrt-" + name,
		},
		RunnerSettings: common.RunnerSettings{
			Executor:              executor,
			RuntimeEnvironmentTTL: ttl,
			FeatureFlags: map[string]bool{
				featureflags.SuspendableEnvironments: ffOn,
			},
		},
	}
}

// One sweep over a mix of runners: only eligible ones are swept, each with the
// same now whatever its TTL (expiry is stamped on the environment), a bounded
// context, and one runner's error does not stop the others.
func TestRuntimeEnvironmentSweepLoop_Sweep(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	customTTL := 48 * time.Hour
	logger, hook := test.NewNullLogger()

	sweeper := newSweepingProvider(t)
	expectSweep := func(name string, err error) {
		sweeper.MockRuntimeEnvironmentSweeper.EXPECT().
			SweepRuntimeEnvironments(mock.Anything, mock.MatchedBy(func(c *common.RunnerConfig) bool {
				return c.Name == name
			}), now).
			Run(func(ctx context.Context, _ *common.RunnerConfig, _ time.Time) {
				_, ok := ctx.Deadline()
				assert.True(t, ok, "each sweep call must be bounded")
			}).
			Return(err).
			Once()
	}
	expectSweep("default-ttl", errors.New("forbidden"))
	expectSweep("custom-ttl", nil)

	cfg := &common.Config{Runners: []*common.RunnerConfig{
		sweepRunner("default-ttl", "sweeper", 1, true, nil),
		sweepRunner("custom-ttl", "sweeper", 2, true, &customTTL),
		sweepRunner("ff-off", "sweeper", 3, false, nil),
		sweepRunner("no-id", "sweeper", 0, true, nil),
		sweepRunner("cannot-sweep", "plain", 4, true, nil),
		sweepRunner("unknown-executor", "missing", 5, true, nil),
	}}
	loop := &runtimeEnvironmentSweepLoop{
		providers: executors.NewProviderRegistry(map[string]common.ExecutorProvider{
			"sweeper": sweeper,
			"plain":   common.NewMockExecutorProvider(t),
		}),
		config: func() *common.Config { return cfg },
		now:    func() time.Time { return now },
		log:    logger,
	}

	loop.sweep(t.Context())

	require.Len(t, hook.AllEntries(), 1)
	entry := hook.LastEntry()
	assert.Equal(t, logrus.ErrorLevel, entry.Level)
	assert.EqualError(t, entry.Data[logrus.ErrorKey].(error), "forbidden")
}

func TestRuntimeEnvironmentSweepLoop_RunStopsOnCancel(t *testing.T) {
	sweeper := newSweepingProvider(t)
	swept := make(chan struct{}, 1)
	sweeper.MockRuntimeEnvironmentSweeper.EXPECT().
		SweepRuntimeEnvironments(mock.Anything, mock.Anything, mock.Anything).
		Run(func(context.Context, *common.RunnerConfig, time.Time) {
			select {
			case swept <- struct{}{}:
			default:
			}
		}).
		Return(nil)

	cfg := &common.Config{Runners: []*common.RunnerConfig{sweepRunner("a", "sweeper", 1, true, nil)}}
	loop := &runtimeEnvironmentSweepLoop{
		providers: executors.NewProviderRegistry(map[string]common.ExecutorProvider{"sweeper": sweeper}),
		config:    func() *common.Config { return cfg },
		now:       time.Now,
		log:       logrus.New(),
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		loop.run(ctx, time.Hour)
	}()

	select {
	case <-swept:
	case <-time.After(10 * time.Second):
		t.Fatal("first sweep did not run at start")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("loop did not stop after cancel")
	}
}

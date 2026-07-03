//go:build !integration

package commands

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/commands/internal/configfile"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

// observedMaxBuildID is a real max(p_ci_builds.id) sample.
const observedMaxBuildID int64 = 15_080_631_350

func TestNextBootVerifyJobID(t *testing.T) {
	bootVerifyJobSeq.Store(0)

	const n = 4096
	require.Less(t, int64(n), bootVerifyIDCounterMask, "n must stay under the counter range")

	seen := make(map[int64]struct{}, n)
	for range n {
		id := nextBootVerifyJobID()

		assert.Positive(t, id)
		assert.Greater(t, id, observedMaxBuildID)
		assert.GreaterOrEqual(t, id, bootVerifyIDBase)

		_, dup := seen[id]
		assert.Falsef(t, dup, "repeated id %d", id)
		seen[id] = struct{}{}
	}
}

func TestNextBootVerifyJobIDWrapsWithinRegion(t *testing.T) {
	// Park the counter at its max so the next Add wraps the low bits to zero.
	bootVerifyJobSeq.Store(bootVerifyIDCounterMask)

	id := nextBootVerifyJobID()

	assert.Positive(t, id)
	assert.Equal(t, bootVerifyIDBase, id)
	assert.Equal(t, bootVerifyIDBase, id&^bootVerifyIDCounterMask)
}

func TestBootVerifyJobSpec(t *testing.T) {
	t.Run("step timeout sits below the deadline", func(t *testing.T) {
		job := bootVerifyJobSpec(5 * time.Minute)

		require.Len(t, job.Steps, 1)
		assert.Equal(t, int((5*time.Minute - bootVerifyStepMargin).Seconds()), job.Steps[0].Timeout)
		assert.Equal(t, "none", job.Variables.Value("GIT_STRATEGY"))
		assert.GreaterOrEqual(t, job.ID, bootVerifyIDBase)
	})

	t.Run("timeout below the margin floors to the deadline", func(t *testing.T) {
		job := bootVerifyJobSpec(10 * time.Second)
		assert.Equal(t, 10, job.Steps[0].Timeout)
	})
}

func TestAcquireWithRetry(t *testing.T) {
	runner := &common.RunnerConfig{}
	const data = "executor-data"

	t.Run("returns on first success", func(t *testing.T) {
		p := common.NewMockExecutorProvider(t)
		p.On("Acquire", runner).Return(data, nil).Once()

		got, err := acquireWithRetry(t.Context(), p, runner, time.Millisecond, time.Millisecond)
		require.NoError(t, err)
		assert.Equal(t, data, got)
	})

	t.Run("retries a transient failure then succeeds", func(t *testing.T) {
		p := common.NewMockExecutorProvider(t)
		p.On("Acquire", runner).Return(nil, errors.New("no free machines")).Once()
		p.On("Acquire", runner).Return(data, nil).Once()

		got, err := acquireWithRetry(t.Context(), p, runner, time.Millisecond, time.Millisecond)
		require.NoError(t, err)
		assert.Equal(t, data, got)
	})

	t.Run("stops on a cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		p := common.NewMockExecutorProvider(t)
		// Long backoff so cancellation wins the select rather than the timer.
		p.On("Acquire", runner).Return(nil, errors.New("no free machines")).Once()

		_, err := acquireWithRetry(ctx, p, runner, time.Hour, time.Hour)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestRunBootVerify(t *testing.T) {
	newCmd := func(runners ...*common.RunnerConfig) *RunCommand {
		return &RunCommand{
			executorProviders: executors.NewProviderRegistry(map[string]common.ExecutorProvider{}),
			configfile: configfile.New("", configfile.WithExistingConfig(
				&common.Config{Runners: runners},
			), configfile.WithSystemID(common.UnknownSystemID)),
		}
	}

	t.Run("skips when no runner enables boot_verify", func(t *testing.T) {
		mr := newCmd(
			&common.RunnerConfig{RunnerCredentials: common.RunnerCredentials{Token: "a"}},
			&common.RunnerConfig{RunnerCredentials: common.RunnerCredentials{Token: "b"}, Experimental: &common.RunnerExperimental{BootVerify: &common.BootVerify{}}},
		)

		assert.NoError(t, mr.runBootVerify())
	})

	t.Run("fails when an enabled runner has no provider", func(t *testing.T) {
		mr := newCmd(&common.RunnerConfig{
			Name:              "canary",
			RunnerCredentials: common.RunnerCredentials{Token: "a"},
			RunnerSettings:    common.RunnerSettings{Executor: "nonexistent"},
			Experimental:      &common.RunnerExperimental{BootVerify: &common.BootVerify{Enabled: true}},
		})

		err := mr.runBootVerify()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "canary")
		assert.Contains(t, err.Error(), "no executor provider")
	})
}

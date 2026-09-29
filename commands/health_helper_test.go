//go:build !integration

package commands

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestHealthHelper_Backoff(t *testing.T) {
	interval := 2 * time.Minute
	runner := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			URL:   "https://gitlab.example.com/",
			Token: "token",
		},
	}
	runner.UnhealthyRequestsLimit = 2
	runner.UnhealthyInterval = &interval

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	h := newHealthHelper()
	h.now = func() time.Time { return now }
	healthyGauge := func() float64 { return testutil.ToFloat64(h.runnerHealthy(runner)) }

	h.markHealth(runner, false)
	assert.True(t, h.isHealthy(runner), "below the limit")

	h.markHealth(runner, false)
	assert.False(t, h.isHealthy(runner), "disabled after reaching the limit")
	assert.Equal(t, float64(0), healthyGauge())

	h.markHealth(runner, false)
	now = now.Add(29 * time.Second)
	assert.False(t, h.isHealthy(runner), "extra failures do not extend the first backoff")

	now = now.Add(time.Second)
	assert.True(t, h.isHealthy(runner), "probe allowed after 30s")

	h.markHealth(runner, false)
	assert.False(t, h.isHealthy(runner), "one failed probe re-disables")
	now = now.Add(29 * time.Second)
	assert.False(t, h.isHealthy(runner), "second backoff is at least 30s")
	now = now.Add(31 * time.Second)
	assert.True(t, h.isHealthy(runner), "second backoff is at most 60s")

	h.markHealth(runner, false)
	now = now.Add(29 * time.Second)
	assert.False(t, h.isHealthy(runner))
	now = now.Add(interval - 29*time.Second)
	assert.True(t, h.isHealthy(runner), "backoff capped at unhealthy_interval")

	h.markHealth(runner, true)
	assert.True(t, h.isHealthy(runner))
	assert.Equal(t, float64(1), healthyGauge())

	h.markHealth(runner, false)
	h.markHealth(runner, false)
	now = now.Add(29 * time.Second)
	assert.False(t, h.isHealthy(runner))
	now = now.Add(time.Second)
	assert.True(t, h.isHealthy(runner), "backoff restarts at 30s after a success")
}

func TestHealthHelper_IntervalBelowInitialBackoff(t *testing.T) {
	interval := 10 * time.Second
	runner := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			URL:   "https://gitlab.example.com/",
			Token: "token",
		},
	}
	runner.UnhealthyRequestsLimit = 1
	runner.UnhealthyInterval = &interval

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	h := newHealthHelper()
	h.now = func() time.Time { return now }

	h.markHealth(runner, false)
	now = now.Add(9 * time.Second)
	assert.False(t, h.isHealthy(runner))
	now = now.Add(time.Second)
	assert.True(t, h.isHealthy(runner))
}

func TestHealthHelper_ZeroIntervalNeverDisables(t *testing.T) {
	interval := time.Duration(0)
	runner := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			URL:   "https://gitlab.example.com/",
			Token: "token",
		},
	}
	runner.UnhealthyRequestsLimit = 1
	runner.UnhealthyInterval = &interval

	h := newHealthHelper()

	for range 5 {
		h.markHealth(runner, false)
		assert.True(t, h.isHealthy(runner))
	}
	assert.Equal(t, float64(1), testutil.ToFloat64(h.runnerHealthy(runner)))
}

//go:build !integration

package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

func TestRunnerSettings_RuntimeEnvironmentExpiresAt(t *testing.T) {
	createdAt := time.Date(2026, 10, 14, 0, 0, 0, 500, time.FixedZone("ICT", 7*60*60))
	customTTL := 48 * time.Hour

	tests := []struct {
		name string
		ttl  *time.Duration
		want time.Duration
	}{
		{name: "unset", ttl: nil, want: DefaultRuntimeEnvironmentTTL},
		{name: "zero never sweeps everything", ttl: new(time.Duration(0)), want: DefaultRuntimeEnvironmentTTL},
		{name: "custom", ttl: &customTTL, want: customTTL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := RunnerSettings{RuntimeEnvironmentTTL: tt.ttl}
			got := s.RuntimeEnvironmentExpiresAt(createdAt)
			assert.Equal(t, createdAt.Add(tt.want).Truncate(time.Second).UTC(), got)
			assert.Equal(t, time.UTC, got.Location())
		})
	}
}

func TestRuntimeEnvironmentExpiryStamp(t *testing.T) {
	expiresAt := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)

	stamp := FormatRuntimeEnvironmentExpiry(expiresAt)
	assert.Equal(t, "1792540800", stamp)
	got, err := ParseRuntimeEnvironmentExpiry(stamp)
	require.NoError(t, err)
	assert.Equal(t, expiresAt, got)

	for _, bad := range []string{"", "abc", "0", "-1", "1.5"} {
		_, err := ParseRuntimeEnvironmentExpiry(bad)
		assert.Error(t, err, "stamp %q", bad)
	}
}

func TestRunnerConfig_ValidateRuntimeEnvironmentTTL(t *testing.T) {
	tests := []struct {
		name    string
		ttl     *time.Duration
		wantErr bool
	}{
		{name: "unset", ttl: nil},
		{name: "exactly the maximum", ttl: new(MaxRuntimeEnvironmentTTL)},
		{name: "above the maximum", ttl: new(MaxRuntimeEnvironmentTTL + time.Second), wantErr: true},
		{name: "negative", ttl: new(-time.Hour), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &RunnerConfig{RunnerSettings: RunnerSettings{RuntimeEnvironmentTTL: tt.ttl}}

			err := c.Validate()
			if tt.wantErr {
				assert.ErrorContains(t, err, "runtime_environment_ttl")
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestCheckRuntimeEnvironmentExpiry(t *testing.T) {
	expiresAt := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)

	assert.NoError(t, CheckRuntimeEnvironmentExpiry(expiresAt, expiresAt), "exactly at the expiry")

	err := CheckRuntimeEnvironmentExpiry(expiresAt, expiresAt.Add(time.Second))
	assert.ErrorIs(t, err, ErrRuntimeEnvironmentExpired)
	assert.ErrorIs(t, err, ErrRuntimeEnvironmentUnavailable)
	var buildErr *BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Equal(t, RuntimeEnvironmentUnavailable, buildErr.FailureReason)
}

func TestRuntimeEnvironmentSweepable(t *testing.T) {
	expiresAt := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)

	assert.False(t, RuntimeEnvironmentSweepable(expiresAt, expiresAt.Add(time.Second)), "within the grace")
	assert.False(t, RuntimeEnvironmentSweepable(expiresAt, expiresAt.Add(RuntimeEnvironmentSweepGrace)), "exactly at the grace")
	assert.True(t, RuntimeEnvironmentSweepable(expiresAt, expiresAt.Add(RuntimeEnvironmentSweepGrace+time.Second)))
}

func TestRuntimeEnvironmentUnavailable_MapsToSupportedReason(t *testing.T) {
	assert.Contains(t, allFailureReasons, RuntimeEnvironmentUnavailable)

	m := newFailureReasonMapper(nil)
	assert.Equal(t, RunnerSystemFailure, m.Map(RuntimeEnvironmentUnavailable),
		"GitLab instances without the reason fall back to the previous resume failure reason")

	m = newFailureReasonMapper([]spec.JobFailureReason{RuntimeEnvironmentUnavailable})
	assert.Equal(t, RuntimeEnvironmentUnavailable, m.Map(RuntimeEnvironmentUnavailable))
}

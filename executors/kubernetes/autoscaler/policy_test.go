//go:build !integration

package autoscaler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyParse(t *testing.T) {
	tests := []struct {
		name    string
		policy  Policy
		wantErr bool
	}{
		{
			name:   "empty periods defaults to always",
			policy: Policy{IdleCount: 5},
		},
		{
			name: "single period",
			policy: Policy{
				Periods:   []string{"* 8-17 * * mon-fri"},
				IdleCount: 5,
			},
		},
		{
			name: "multiple periods",
			policy: Policy{
				Periods:   []string{"* 8-12 * * *", "* 14-18 * * *"},
				IdleCount: 5,
			},
		},
		{
			name: "invalid period",
			policy: Policy{
				Periods: []string{"invalid"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Parse()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPolicyIsActive(t *testing.T) {
	businessHours := Policy{
		Periods:   []string{"* 8-17 * * mon-fri"},
		Timezone:  "UTC",
		IdleCount: 5,
	}
	require.NoError(t, businessHours.Parse())

	tests := []struct {
		name     string
		policy   Policy
		time     time.Time
		isActive bool
	}{
		{
			name:     "business hours - monday morning",
			policy:   businessHours,
			time:     time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			isActive: true,
		},
		{
			name:     "business hours - saturday",
			policy:   businessHours,
			time:     time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC),
			isActive: false,
		},
		{
			name:     "business hours - monday evening",
			policy:   businessHours,
			time:     time.Date(2024, 1, 15, 20, 0, 0, 0, time.UTC),
			isActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isActive, tt.policy.IsActive(tt.time))
		})
	}
}

func TestPolicyListActive(t *testing.T) {
	// Default policy (always active, idle_count=0)
	defaultPolicy := Policy{
		Periods:   []string{"* * * * *"},
		Timezone:  "UTC",
		IdleCount: 0,
		IdleTime:  0,
	}
	require.NoError(t, defaultPolicy.Parse())

	// Business hours policy (idle_count=5)
	businessPolicy := Policy{
		Periods:   []string{"* 8-17 * * mon-fri"},
		Timezone:  "UTC",
		IdleCount: 5,
		IdleTime:  30 * time.Minute,
	}
	require.NoError(t, businessPolicy.Parse())

	policies := PolicyList{defaultPolicy, businessPolicy}

	tests := []struct {
		name              string
		time              time.Time
		expectedIdleCount int
	}{
		{
			name:              "during business hours - last matching wins",
			time:              time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), // Monday 10:00
			expectedIdleCount: 5,
		},
		{
			name:              "outside business hours - default",
			time:              time.Date(2024, 1, 15, 20, 0, 0, 0, time.UTC), // Monday 20:00
			expectedIdleCount: 0,
		},
		{
			name:              "weekend - default",
			time:              time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC), // Saturday 10:00
			expectedIdleCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			active := policies.Active(tt.time)
			assert.Equal(t, tt.expectedIdleCount, active.IdleCount)
		})
	}
}

func TestPolicyListActiveReturnsDefault(t *testing.T) {
	// Empty policy list should return DefaultPolicy
	var policies PolicyList
	active := policies.Active(time.Now())
	assert.Equal(t, DefaultPolicy.IdleCount, active.IdleCount)
	assert.Equal(t, DefaultPolicy.IdleTime, active.IdleTime)
}

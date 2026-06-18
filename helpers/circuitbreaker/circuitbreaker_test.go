//go:build !integration

package circuitbreaker

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testThreshold   = 3
	testOpenTimeout = 30 * time.Second
	testGrace       = 5 * time.Second
)

// gaugeState returns the value the breaker's state gauge reports.
func gaugeState(b *Breaker) State {
	return State(testutil.ToFloat64(b.metrics.state))
}

func TestBreaker_TripsAfterThreshold(t *testing.T) {
	b := New(testThreshold, testOpenTimeout, WithMetrics("test", "cb"))
	for i := 0; i < testThreshold-1; i++ {
		assert.False(t, b.RecordFailure(), "stays closed before the threshold")
		assert.True(t, b.Allow(), "closed breaker allows requests")
	}
	assert.Zero(t, testutil.ToFloat64(b.metrics.trips), "no trip before the threshold")
	assert.Equal(t, Closed, gaugeState(b))

	assert.True(t, b.RecordFailure(), "threshold failure trips the breaker")
	assert.False(t, b.Allow(), "open breaker blocks requests")
	assert.EqualValues(t, 1, testutil.ToFloat64(b.metrics.trips), "trip increments the counter")
	assert.Equal(t, Open, gaugeState(b))
}

func TestBreaker_SuccessResetsFailureCount(t *testing.T) {
	b := New(testThreshold, testOpenTimeout, WithMetrics("test", "cb"))
	for i := 0; i < testThreshold-1; i++ {
		b.RecordFailure()
	}
	b.RecordSuccess()
	// A reset counter means it takes another full threshold to trip.
	for i := 0; i < testThreshold-1; i++ {
		assert.False(t, b.RecordFailure())
	}
	assert.Zero(t, testutil.ToFloat64(b.metrics.trips), "no trip yet, the reset deferred it")
	assert.True(t, b.RecordFailure())
	assert.EqualValues(t, 1, testutil.ToFloat64(b.metrics.trips))
}

func TestBreaker_FailureGraceIgnoresInstantBurst(t *testing.T) {
	now := time.Now()
	b := New(testThreshold, testOpenTimeout,
		WithClock(func() time.Time { return now }),
		WithFailureGrace(testGrace))

	// A burst of well over the threshold at one instant stays closed: the
	// failures don't span the grace window.
	for i := 0; i < testThreshold+2; i++ {
		assert.False(t, b.RecordFailure(), "instantaneous burst stays within the grace window")
	}
	assert.True(t, b.Allow(), "breaker is still closed")

	// A success resets the streak.
	b.RecordSuccess()

	// Failures that persist past the grace window trip.
	for i := 0; i < testThreshold; i++ {
		assert.False(t, b.RecordFailure(), "within the grace window")
	}
	now = now.Add(testGrace + time.Second)
	assert.True(t, b.RecordFailure(), "sustained failure past the grace window trips")
}

func TestBreaker_AbortReleasesTrialWithoutVerdict(t *testing.T) {
	now := time.Now()
	b := New(testThreshold, testOpenTimeout,
		WithClock(func() time.Time { return now }),
		WithMetrics("test", "cb"))
	for i := 0; i < testThreshold; i++ {
		b.RecordFailure()
	}
	now = now.Add(testOpenTimeout + time.Second)
	require.True(t, b.Allow(), "half-open trial granted")

	b.Abort()
	assert.Equal(t, Open, gaugeState(b), "aborted trial reverts to open")
	assert.EqualValues(t, 1, testutil.ToFloat64(b.metrics.trips), "abort is not a trip")
	assert.False(t, b.Allow(), "must wait the cooldown again before the next trial")

	now = now.Add(testOpenTimeout + time.Second)
	assert.True(t, b.Allow(), "a fresh trial is granted after the cooldown")
}

func TestBreaker_HalfOpenTrialAfterTimeout(t *testing.T) {
	now := time.Now()
	b := New(testThreshold, testOpenTimeout, WithClock(func() time.Time { return now }), WithMetrics("test", "cb"))
	for i := 0; i < testThreshold; i++ {
		b.RecordFailure()
	}
	assert.EqualValues(t, 1, testutil.ToFloat64(b.metrics.trips))
	assert.Equal(t, Open, gaugeState(b))

	assert.False(t, b.Allow(), "blocks requests before the open timeout elapses")
	now = now.Add(testOpenTimeout + time.Second)
	assert.True(t, b.Allow(), "allows a single trial request after the timeout")
	assert.Equal(t, HalfOpen, gaugeState(b))
	assert.False(t, b.Allow(), "no second concurrent trial while half-open")
}

func TestBreaker_HalfOpenTrialSuccessCloses(t *testing.T) {
	now := time.Now()
	b := New(testThreshold, testOpenTimeout, WithClock(func() time.Time { return now }), WithMetrics("test", "cb"))
	for i := 0; i < testThreshold; i++ {
		b.RecordFailure()
	}
	now = now.Add(testOpenTimeout + time.Second)
	assert.True(t, b.Allow(), "trial request")

	assert.True(t, b.RecordSuccess(), "a successful trial request reports recovery")
	assert.True(t, b.Allow(), "closed after a successful trial request")
	assert.Equal(t, Closed, b.State())
	assert.Equal(t, Closed, gaugeState(b))
	assert.EqualValues(t, 1, testutil.ToFloat64(b.metrics.trips), "recovery does not add a trip")

	// A success while already closed is not a recovery.
	assert.False(t, b.RecordSuccess(), "a success in the closed state does not report recovery")
}

func TestBreaker_HalfOpenTrialFailureReopens(t *testing.T) {
	now := time.Now()
	b := New(testThreshold, testOpenTimeout, WithClock(func() time.Time { return now }), WithMetrics("test", "cb"))
	for i := 0; i < testThreshold; i++ {
		b.RecordFailure()
	}
	now = now.Add(testOpenTimeout + time.Second)
	assert.True(t, b.Allow(), "trial request")

	assert.True(t, b.RecordFailure(), "a failed trial request reopens the breaker")
	assert.EqualValues(t, 2, testutil.ToFloat64(b.metrics.trips), "reopening counts as another trip")
	assert.False(t, b.Allow(), "blocks requests again after reopening")
	// The cooldown restarts from the reopen, so another trial is allowed later.
	now = now.Add(testOpenTimeout + time.Second)
	assert.True(t, b.Allow())
}

//go:build !integration

package circuitbreaker

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
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

func gaugeState(b *Breaker) State {
	return State(testutil.ToFloat64(b.metrics.state))
}

func newTestBreaker(opts ...Option) (*Breaker, *time.Time) {
	now := time.Date(2026, 10, 13, 13, 17, 0, 0, time.UTC)
	opts = append([]Option{
		WithClock(func() time.Time { return now }),
		WithMetrics("test", "cb"),
	}, opts...)
	b := New(testThreshold, testOpenTimeout, opts...)
	b.rand = func() float64 { return 1 }
	return b, &now
}

func fail(t *testing.T, b *Breaker) bool {
	t.Helper()
	p, ok := b.Acquire()
	require.True(t, ok, "request admitted")
	return p.Failure()
}

func succeed(t *testing.T, b *Breaker) bool {
	t.Helper()
	p, ok := b.Acquire()
	require.True(t, ok, "request admitted")
	return p.Success()
}

func trip(t *testing.T, b *Breaker) {
	t.Helper()
	for range testThreshold - 1 {
		fail(t, b)
	}
	require.True(t, fail(t, b))
}

func TestBreaker_TripsAfterThreshold(t *testing.T) {
	b, _ := newTestBreaker()
	for range testThreshold - 1 {
		assert.False(t, fail(t, b), "stays closed before the threshold")
	}
	assert.Zero(t, testutil.ToFloat64(b.metrics.trips))
	assert.Equal(t, Closed, gaugeState(b))

	assert.True(t, fail(t, b), "threshold failure trips the breaker")
	_, ok := b.Acquire()
	assert.False(t, ok, "open breaker blocks requests")
	assert.EqualValues(t, 1, testutil.ToFloat64(b.metrics.trips))
	assert.Equal(t, Open, gaugeState(b))
}

func TestBreaker_SuccessResetsFailureCount(t *testing.T) {
	b, _ := newTestBreaker()
	for range testThreshold - 1 {
		fail(t, b)
	}
	succeed(t, b)
	for range testThreshold - 1 {
		assert.False(t, fail(t, b))
	}
	assert.True(t, fail(t, b))
}

func TestBreaker_FailureGraceIgnoresInstantBurst(t *testing.T) {
	b, now := newTestBreaker(WithFailureGrace(testGrace))

	for range testThreshold + 2 {
		assert.False(t, fail(t, b), "instantaneous burst stays within the grace window")
	}
	succeed(t, b)

	for range testThreshold {
		assert.False(t, fail(t, b), "within the grace window")
	}
	*now = now.Add(testGrace + time.Second)
	assert.True(t, fail(t, b), "sustained failure past the grace window trips")
}

func TestBreaker_HalfOpenTrialAfterTimeout(t *testing.T) {
	b, now := newTestBreaker()
	trip(t, b)

	_, ok := b.Acquire()
	assert.False(t, ok, "blocks requests before the open timeout elapses")
	*now = now.Add(testOpenTimeout + time.Second)
	_, ok = b.Acquire()
	assert.True(t, ok, "allows a trial request after the timeout")
	assert.Equal(t, HalfOpen, gaugeState(b))
	_, ok = b.Acquire()
	assert.False(t, ok, "no second concurrent trial while half-open")
}

func TestBreaker_HalfOpenTrialSuccessCloses(t *testing.T) {
	b, now := newTestBreaker()
	trip(t, b)
	*now = now.Add(testOpenTimeout + time.Second)

	assert.True(t, succeed(t, b), "a successful trial request reports recovery")
	assert.Equal(t, Closed, b.State())
	assert.EqualValues(t, 1, testutil.ToFloat64(b.metrics.trips), "recovery does not add a trip")
	assert.False(t, succeed(t, b), "a success in the closed state does not report recovery")
}

func TestBreaker_HalfOpenTrialFailureReopens(t *testing.T) {
	b, now := newTestBreaker()
	trip(t, b)
	*now = now.Add(testOpenTimeout + time.Second)

	assert.True(t, fail(t, b), "a failed trial request reopens the breaker")
	assert.EqualValues(t, 2, testutil.ToFloat64(b.metrics.trips))
	_, ok := b.Acquire()
	assert.False(t, ok, "blocks requests again after reopening")
	*now = now.Add(testOpenTimeout + time.Second)
	_, ok = b.Acquire()
	assert.True(t, ok)
}

func TestBreaker_AbortReleasesTrial(t *testing.T) {
	b, now := newTestBreaker()
	trip(t, b)
	*now = now.Add(testOpenTimeout + time.Second)
	p, ok := b.Acquire()
	require.True(t, ok)

	p.Abort()
	assert.Equal(t, HalfOpen, b.State())
	assert.EqualValues(t, 1, testutil.ToFloat64(b.metrics.trips), "abort is not a trip")
	_, ok = b.Acquire()
	assert.True(t, ok, "the trial slot is free again")
}

func TestBreaker_StalePermitsAreIgnored(t *testing.T) {
	b, now := newTestBreaker()
	inFlight, ok := b.Acquire()
	require.True(t, ok)
	trip(t, b)
	openUntil := b.openUntil

	assert.False(t, inFlight.Failure())
	assert.Equal(t, openUntil, b.openUntil, "a stale failure does not restart the open timeout")
	assert.EqualValues(t, 1, testutil.ToFloat64(b.metrics.trips))

	assert.False(t, inFlight.Success())
	assert.Equal(t, Open, b.State(), "a stale success does not close the breaker")

	*now = now.Add(testOpenTimeout + time.Second)
	probe, ok := b.Acquire()
	require.True(t, ok)
	inFlight.Abort()
	_, ok = b.Acquire()
	assert.False(t, ok, "a stale abort does not release the trial slot")
	probe.Success()
}

func ratePolicy(minRequests int, settle time.Duration) FailureRatePolicy {
	return FailureRatePolicy{Rate: 0.5, Window: time.Hour, Settle: settle, MinRequests: minRequests}
}

func TestBreaker_FailureRateTripsAtMinRequests(t *testing.T) {
	b, _ := newTestBreaker(WithFailureRate(ratePolicy(4, 0)))

	for range 3 {
		assert.False(t, fail(t, b), "fewer than MinRequests admitted")
	}
	assert.False(t, succeed(t, b), "a success is not a recovery")
	assert.Equal(t, Open, b.State(), "3 of 4 failed")
}

func TestBreaker_FailureRateStaysClosedBelowThreshold(t *testing.T) {
	b, _ := newTestBreaker(WithFailureRate(ratePolicy(4, 0)))

	for i := range 40 {
		if i%4 == 0 {
			fail(t, b)
		} else {
			succeed(t, b)
		}
	}
	assert.Equal(t, Closed, b.State())
}

func TestBreaker_FailureRateWindowClearsOnClose(t *testing.T) {
	b, now := newTestBreaker(WithFailureRate(ratePolicy(4, 0)))
	for range 4 {
		fail(t, b)
	}
	require.Equal(t, Open, b.State())
	*now = now.Add(testOpenTimeout + time.Second)
	succeed(t, b)
	require.Equal(t, Closed, b.State())

	for range 3 {
		fail(t, b)
	}
	assert.Equal(t, Closed, b.State(), "the window starts empty after closing")
}

func TestBreaker_FailureRateForgetsOldAdmissions(t *testing.T) {
	b, now := newTestBreaker(WithFailureRate(FailureRatePolicy{Rate: 0.5, Window: time.Minute, MinRequests: 2}))
	fail(t, b)
	*now = now.Add(2 * time.Minute)
	assert.False(t, fail(t, b), "the first failure is older than the window")
	assert.Equal(t, Closed, b.State())
}

func TestBreaker_FailureRateForgetsOldAdmissionsWithoutNewOnes(t *testing.T) {
	b, now := newTestBreaker(WithFailureRate(FailureRatePolicy{Rate: 0.5, Window: time.Minute, MinRequests: 4}))
	for range 3 {
		fail(t, b)
	}
	*now = now.Add(50 * time.Second)
	p, _ := b.Acquire()
	*now = now.Add(time.Minute)
	assert.False(t, p.Failure(), "the three failures are older than the window")
	assert.Equal(t, Closed, b.State())
}

func TestBreaker_SuccessCanTripAndReportsIt(t *testing.T) {
	var changes []State
	b, now := newTestBreaker(
		WithFailureRate(ratePolicy(2, time.Second)),
		WithStateChange(func(_, to State) { changes = append(changes, to) }),
	)
	p1, _ := b.Acquire()
	p2, _ := b.Acquire()
	p1.Failure()
	p2.Failure()
	require.Equal(t, Closed, b.State(), "too young to count")

	*now = now.Add(2 * time.Second)
	assert.False(t, succeed(t, b), "a success is not a recovery")
	assert.Equal(t, Open, b.State())
	assert.Equal(t, []State{Open}, changes)
}

func TestBreaker_AbortedRequestsAreNotCounted(t *testing.T) {
	b, _ := newTestBreaker(WithFailureRate(ratePolicy(2, 0)))
	p, _ := b.Acquire()
	p.Abort()
	assert.False(t, fail(t, b), "one counted request")
	assert.Equal(t, Closed, b.State())
}

func TestBreaker_StateChangeCallback(t *testing.T) {
	var changes [][2]State
	b, now := newTestBreaker(WithStateChange(func(from, to State) { changes = append(changes, [2]State{from, to}) }))
	trip(t, b)
	*now = now.Add(testOpenTimeout + time.Second)
	succeed(t, b)
	assert.Equal(t, [][2]State{{Closed, Open}, {Open, HalfOpen}, {HalfOpen, Closed}}, changes)
}

func TestBreaker_ResetClosesAndAdmitsWaiters(t *testing.T) {
	b, _ := newTestBreaker()
	trip(t, b)

	admitted := make(chan *Permit)
	go func() {
		p, _ := b.Wait(t.Context())
		admitted <- p
	}()
	select {
	case <-admitted:
		t.Fatal("admitted while open")
	case <-time.After(50 * time.Millisecond):
	}

	b.Reset()
	assert.Equal(t, Closed, b.State())
	select {
	case <-admitted:
	case <-time.After(time.Second):
		t.Fatal("waiter not admitted by Reset")
	}
}

func TestBreaker_FailureRateIgnoresRequestsYoungerThanSettle(t *testing.T) {
	b, now := newTestBreaker(WithFailureRate(ratePolicy(20, 5*time.Second)))

	permits := make([]*Permit, 90)
	for i := range permits {
		permits[i], _ = b.Acquire()
	}
	*now = now.Add(time.Second)
	for i, p := range permits {
		if i%5 == 0 {
			p.Failure()
		}
	}
	assert.Equal(t, Closed, b.State(), "fast failures are not judged before the settle time")

	*now = now.Add(5 * time.Second)
	assert.False(t, succeed(t, b), "18 of 90 failed once the burst is old enough to count")
	assert.Equal(t, Closed, b.State())
}

func TestBreaker_FailureRateCountsPendingAsNotFailed(t *testing.T) {
	b, now := newTestBreaker(WithFailureRate(ratePolicy(4, time.Second)))

	permits := make([]*Permit, 4)
	for i := range permits {
		permits[i], _ = b.Acquire()
	}
	*now = now.Add(2 * time.Second)
	assert.False(t, permits[0].Failure(), "1 of 4")
	assert.True(t, permits[1].Failure(), "2 of 4 failed, whatever the other 2 do")
	assert.Equal(t, Open, b.State())
}

func TestBreaker_FailureRateTripsOnlyFromRequestsOlderThanSettle(t *testing.T) {
	b, now := newTestBreaker(WithFailureRate(ratePolicy(4, time.Second)))

	old := make([]*Permit, 4)
	for i := range old {
		old[i], _ = b.Acquire()
	}
	*now = now.Add(2 * time.Second)
	young := make([]*Permit, 90)
	for i := range young {
		young[i], _ = b.Acquire()
	}
	for _, p := range old {
		p.Failure()
	}
	assert.Equal(t, Open, b.State(), "90 pending young requests do not dilute 4 old failures")
	for _, p := range young {
		p.Success()
	}
	assert.Equal(t, Open, b.State())
}

func TestBreaker_HalfOpenRoundReopensOnlyWhenAllProbesFail(t *testing.T) {
	b, now := newTestBreaker(WithProbes(3))
	trip(t, b)
	*now = now.Add(testOpenTimeout + time.Second)

	p1, _ := b.Acquire()
	p2, _ := b.Acquire()
	p3, _ := b.Acquire()
	assert.False(t, p1.Failure())
	assert.False(t, p2.Failure())
	assert.Equal(t, HalfOpen, b.State(), "two fast failures do not decide the round")
	assert.True(t, p3.Success())
	assert.Equal(t, Closed, b.State())

	trip(t, b)
	*now = now.Add(testOpenTimeout + time.Second)
	p1, _ = b.Acquire()
	p2, _ = b.Acquire()
	p3, _ = b.Acquire()
	p1.Failure()
	p2.Abort()
	assert.Equal(t, HalfOpen, b.State(), "an aborted probe is not a failed probe")
	p2, _ = b.Acquire()
	p2.Failure()
	assert.True(t, p3.Failure(), "the round ends when every probe has failed")
	assert.Equal(t, Open, b.State())
}

func TestBreaker_ProbesAdmittedWhileHalfOpen(t *testing.T) {
	b, now := newTestBreaker(WithProbes(2))
	trip(t, b)
	*now = now.Add(testOpenTimeout + time.Second)

	p1, ok := b.Acquire()
	require.True(t, ok)
	p2, ok := b.Acquire()
	require.True(t, ok)
	_, ok = b.Acquire()
	assert.False(t, ok, "only two probes at once")

	assert.True(t, p1.Success())
	assert.Equal(t, Closed, b.State())
	assert.False(t, p2.Failure(), "the other probe is from the previous generation")
	assert.Equal(t, Closed, b.State())
}

func TestBreaker_OpenBackoffDoublesAndCaps(t *testing.T) {
	b, now := newTestBreaker(WithOpenBackoff(4 * testOpenTimeout))
	trip(t, b)
	assert.Equal(t, testOpenTimeout, b.openUntil.Sub(*now))

	for _, want := range []time.Duration{2 * testOpenTimeout, 4 * testOpenTimeout, 4 * testOpenTimeout} {
		*now = b.openUntil
		require.True(t, fail(t, b), "failed probe reopens")
		assert.Equal(t, want, b.openUntil.Sub(*now))
	}

	*now = b.openUntil
	succeed(t, b)
	trip(t, b)
	assert.Equal(t, testOpenTimeout, b.openUntil.Sub(*now), "closing resets the backoff")
}

func TestBreaker_OpenBackoffJitter(t *testing.T) {
	b, now := newTestBreaker(WithOpenBackoff(testOpenTimeout))
	b.rand = func() float64 { return 0 }
	trip(t, b)
	assert.Equal(t, testOpenTimeout/2, b.openUntil.Sub(*now))
}

func TestBreaker_WaitBlocksUntilAdmitted(t *testing.T) {
	b, now := newTestBreaker(WithProbes(1))
	trip(t, b)
	*now = now.Add(testOpenTimeout + time.Second)
	probe, ok := b.Acquire()
	require.True(t, ok)

	admitted := make(chan *Permit)
	go func() {
		p, _ := b.Wait(t.Context())
		admitted <- p
	}()

	select {
	case <-admitted:
		t.Fatal("admitted while half-open with the probe slot taken")
	case <-time.After(50 * time.Millisecond):
	}

	probe.Success()
	select {
	case p := <-admitted:
		assert.Equal(t, b.generation, p.generation)
	case <-time.After(time.Second):
		t.Fatal("waiter not woken by close")
	}
}

func TestBreaker_WaitWokenByAbortedProbe(t *testing.T) {
	b, now := newTestBreaker(WithProbes(1))
	trip(t, b)
	*now = now.Add(testOpenTimeout + time.Second)
	probe, ok := b.Acquire()
	require.True(t, ok)

	admitted := make(chan *Permit)
	go func() {
		p, _ := b.Wait(t.Context())
		admitted <- p
	}()

	select {
	case <-admitted:
		t.Fatal("admitted while half-open with the probe slot taken")
	case <-time.After(50 * time.Millisecond):
	}

	probe.Abort()
	select {
	case p := <-admitted:
		assert.Equal(t, HalfOpen, b.State())
		assert.True(t, p.Success())
	case <-time.After(time.Second):
		t.Fatal("waiter not woken by the aborted probe")
	}
}

func TestBreaker_WaitRefusesWhenContextAlreadyDone(t *testing.T) {
	b, _ := newTestBreaker()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	p, ok := b.Wait(ctx)
	assert.False(t, ok, "closed breaker, cancelled context")
	assert.Nil(t, p)
}

func TestBreaker_WaitReturnsOnContextDone(t *testing.T) {
	b, _ := newTestBreaker()
	trip(t, b)

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	p, ok := b.Wait(ctx)
	assert.False(t, ok)
	assert.Nil(t, p)
}

// simulate runs workers through the breaker against a provider whose failures
// return much faster than its successes, and returns how many attempts the
// provider saw and how many of them failed.
func simulate(b *Breaker, workers int, d time.Duration, failureRate func() float64) (attempts, failures int64) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	rng := rand.New(rand.NewSource(1))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for ctx.Err() == nil {
				p, ok := b.Wait(ctx)
				if !ok {
					return
				}
				mu.Lock()
				fail := rng.Float64() < failureRate()
				mu.Unlock()
				atomic.AddInt64(&attempts, 1)
				if fail {
					atomic.AddInt64(&failures, 1)
					time.Sleep(time.Millisecond)
					p.Failure()
				} else {
					time.Sleep(50 * time.Millisecond)
					p.Success()
				}
			}
		})
	}
	wg.Wait()
	return atomic.LoadInt64(&attempts), atomic.LoadInt64(&failures)
}

func newSimulationBreaker() *Breaker {
	return New(0, 20*time.Millisecond,
		WithFailureRate(FailureRatePolicy{Rate: 0.5, Window: time.Second, Settle: 5 * time.Millisecond, MinRequests: 20}),
		WithProbes(9),
		WithOpenBackoff(200*time.Millisecond))
}

func TestBreaker_Simulation_PartialFailuresDoNotThrottle(t *testing.T) {
	b := newSimulationBreaker()
	var trips atomic.Int64
	go func() {
		was := Closed
		for range 400 {
			if s := b.State(); s == Open && was != Open {
				trips.Add(1)
			} else if s != Open {
				was = s
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	attempts, _ := simulate(b, 90, 2*time.Second, func() float64 { return 0.2 })

	assert.Zero(t, trips.Load(), "20% failures with fast failure completion must not trip")
	assert.Greater(t, attempts, int64(90*2000/50*6/10), "throughput close to the unthrottled rate")
}

func TestBreaker_Simulation_FullOutageBoundsAttempts(t *testing.T) {
	b := newSimulationBreaker()
	attempts, _ := simulate(b, 90, 2*time.Second, func() float64 { return 1 })

	// Unthrottled, 90 workers failing in 1ms each would make tens of
	// thousands of attempts. Open for 20ms doubling to 200ms with 9 probes
	// per round bounds it to a few hundred.
	assert.Less(t, attempts, int64(1500), "attempts during a full outage")
	assert.Equal(t, Open, b.State())
}

func TestBreaker_Simulation_RecoversAfterOutage(t *testing.T) {
	b := newSimulationBreaker()
	var failing atomic.Bool
	failing.Store(true)
	go func() {
		time.Sleep(time.Second)
		failing.Store(false)
	}()

	_, failures := simulate(b, 90, 2*time.Second, func() float64 {
		if failing.Load() {
			return 1
		}
		return 0
	})

	assert.Equal(t, Closed, b.State(), "closed once the provider recovered")
	assert.Less(t, failures, int64(1500))
}

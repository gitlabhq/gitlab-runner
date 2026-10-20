package circuitbreaker

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var _ prometheus.Collector = (*Breaker)(nil)

// State is the state of a Breaker.
type State int32

const (
	Closed   State = iota // requests flow normally
	Open                  // requests are blocked
	HalfOpen              // a limited number of trial requests are allowed through
)

func (s State) String() string {
	switch s {
	case Open:
		return "open"
	case HalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

// Breaker is a circuit breaker. It is safe for concurrent use.
//
// Requests are admitted with a Permit that records the outcome. Outcomes
// count only for the generation the Permit was issued in.
type Breaker struct {
	threshold    int
	failureGrace time.Duration
	rate         *FailureRatePolicy
	openTimeout  time.Duration
	maxOpen      time.Duration
	probes       int
	now          func() time.Time
	rand         func() float64
	onChange     func(from, to State)

	mu         sync.Mutex
	state      State
	generation uint64
	changed    chan struct{}
	openUntil  time.Time
	openCount  int

	failures       int
	firstFailureAt time.Time

	entries []*rateEntry

	probesInFlight int
	probesFailed   int

	metrics *breakerMetrics
}

type outcome int8

const (
	pending outcome = iota
	succeeded
	failed
	aborted
)

type rateEntry struct {
	admitted time.Time
	outcome  outcome
}

// Permit is one admitted request. Exactly one of Success, Failure or Abort
// must be called on it. Calls on a nil Permit do nothing.
type Permit struct {
	b          *Breaker
	generation uint64
	entry      *rateEntry
}

// FailureRatePolicy trips the breaker when at least Rate of the requests
// admitted between Window and Settle ago have failed, given at least
// MinRequests of them. Requests still in flight count as not failed.
type FailureRatePolicy struct {
	Rate        float64
	Window      time.Duration
	Settle      time.Duration
	MinRequests int
}

type breakerMetrics struct {
	state prometheus.GaugeFunc
	trips prometheus.Counter
}

// Option configures a Breaker.
type Option func(*Breaker)

// WithClock overrides the time source. It is primarily useful in tests.
func WithClock(now func() time.Time) Option {
	return func(b *Breaker) { b.now = now }
}

// WithFailureGrace makes the breaker trip only once it has been failing
// continuously for at least d, in addition to reaching the failure threshold,
// so a burst of simultaneous failures does not trip it while a sustained
// outage still does. Only applies to the consecutive-failure policy.
func WithFailureGrace(d time.Duration) Option {
	return func(b *Breaker) { b.failureGrace = d }
}

// WithFailureRate replaces the consecutive-failure policy.
func WithFailureRate(policy FailureRatePolicy) Option {
	return func(b *Breaker) { b.rate = &policy }
}

// WithProbes sets how many trial requests are admitted at once while
// half-open. The default is 1.
func WithProbes(n int) Option {
	return func(b *Breaker) { b.probes = n }
}

// WithOpenBackoff doubles the open timeout on each consecutive trip, up to
// max, and shortens each timeout by up to half as jitter.
func WithOpenBackoff(max time.Duration) Option {
	return func(b *Breaker) { b.maxOpen = max }
}

// WithStateChange calls f on every state change, with the breaker's lock held.
func WithStateChange(f func(from, to State)) Option {
	return func(b *Breaker) { b.onChange = f }
}

// WithMetrics enables Prometheus metrics for the breaker. The caller supplies
// the metric namespace and subsystem so the breaker stays generic, and it
// exposes <namespace>_<subsystem>_state (gauge: 0 closed, 1 open, 2 half-open)
// and <namespace>_<subsystem>_trips_total (counter). With this option the
// Breaker implements prometheus.Collector; register it to export the metrics.
func WithMetrics(namespace, subsystem string) Option {
	return func(b *Breaker) {
		b.metrics = &breakerMetrics{
			state: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "state",
				Help:      "Circuit breaker state (0 = closed, 1 = open, 2 = half-open).",
			}, func() float64 { return float64(b.State()) }),
			trips: prometheus.NewCounter(prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "trips_total",
				Help:      "Total number of times the circuit breaker has tripped open.",
			}),
		}
	}
}

// New returns a Breaker that opens after threshold consecutive failures and
// stays open for openTimeout before allowing trial requests.
func New(threshold int, openTimeout time.Duration, opts ...Option) *Breaker {
	b := &Breaker{
		threshold:   threshold,
		openTimeout: openTimeout,
		probes:      1,
		now:         time.Now,
		rand:        rand.Float64,
		changed:     make(chan struct{}),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Acquire admits a request if the breaker allows it.
func (b *Breaker) Acquire() (*Permit, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.acquire()
}

// Wait blocks until the breaker admits a request or ctx is done.
func (b *Breaker) Wait(ctx context.Context) (*Permit, bool) {
	for {
		if ctx.Err() != nil {
			return nil, false
		}
		b.mu.Lock()
		if permit, ok := b.acquire(); ok {
			b.mu.Unlock()
			return permit, true
		}
		changed := b.changed
		var wake <-chan time.Time
		var timer *time.Timer
		if b.state == Open {
			timer = time.NewTimer(b.openUntil.Sub(b.now()))
			wake = timer.C
		}
		b.mu.Unlock()

		select {
		case <-changed:
		case <-wake:
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return nil, false
		}
		if timer != nil {
			timer.Stop()
		}
	}
}

func (b *Breaker) acquire() (*Permit, bool) {
	if b.state == Open && !b.now().Before(b.openUntil) {
		b.transition(HalfOpen)
	}

	switch b.state {
	case Closed:
		return b.permit(), true
	case HalfOpen:
		if b.probesInFlight < b.probes {
			b.probesInFlight++
			return b.permit(), true
		}
	}
	return nil, false
}

func (b *Breaker) permit() *Permit {
	p := &Permit{b: b, generation: b.generation}
	if b.rate != nil {
		now := b.now()
		p.entry = &rateEntry{admitted: now}
		b.entries = append(b.entriesSince(now.Add(-b.rate.Window)), p.entry)
	}
	return p
}

func (b *Breaker) entriesSince(cutoff time.Time) []*rateEntry {
	i := 0
	for i < len(b.entries) && b.entries[i].admitted.Before(cutoff) {
		i++
	}
	return b.entries[i:]
}

// Success records a successful request and reports whether it closed the
// breaker from half-open.
func (p *Permit) Success() (recovered bool) {
	if p == nil {
		return false
	}
	b := p.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if p.generation != b.generation {
		return false
	}

	switch b.state {
	case HalfOpen:
		b.transition(Closed)
		return true
	case Closed:
		b.failures = 0
		if b.observe(p, succeeded) {
			b.open()
		}
	}
	return false
}

// Failure records a failed request and reports whether it tripped the
// breaker open.
func (p *Permit) Failure() (tripped bool) {
	if p == nil {
		return false
	}
	b := p.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if p.generation != b.generation {
		return false
	}

	switch b.state {
	case HalfOpen:
		b.probesFailed++
		if b.probesFailed >= b.probes {
			b.open()
			return true
		}
	case Closed:
		if b.rate != nil {
			if b.observe(p, failed) {
				b.open()
				return true
			}
			return false
		}
		t := b.now()
		if b.failures == 0 {
			b.firstFailureAt = t
		}
		b.failures++
		if b.failures >= b.threshold && t.Sub(b.firstFailureAt) >= b.failureGrace {
			b.open()
			return true
		}
	}
	return false
}

// Abort releases a permit for a request that was admitted but never attempted.
func (p *Permit) Abort() {
	if p == nil {
		return
	}
	b := p.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if p.generation != b.generation {
		return
	}
	switch b.state {
	case HalfOpen:
		b.probesInFlight--
		b.wake()
	case Closed:
		if p.entry != nil {
			p.entry.outcome = aborted
		}
	}
}

// Reset closes the breaker and admits any waiters.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state != Closed {
		b.transition(Closed)
	}
}

// State returns the current breaker state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// observe records the outcome of an admitted request and reports whether
// the failure rate policy says to trip.
func (b *Breaker) observe(p *Permit, o outcome) bool {
	if b.rate == nil || p.entry == nil {
		return false
	}
	p.entry.outcome = o

	now := b.now()
	b.entries = b.entriesSince(now.Add(-b.rate.Window))
	settled := now.Add(-b.rate.Settle)
	counted, failures := 0, 0
	for _, e := range b.entries {
		if e.admitted.After(settled) {
			break
		}
		if e.outcome == aborted {
			continue
		}
		counted++
		if e.outcome == failed {
			failures++
		}
	}
	return counted >= max(b.rate.MinRequests, 1) && float64(failures)/float64(counted) >= b.rate.Rate
}

func (b *Breaker) open() {
	b.openCount++
	timeout := b.openTimeout
	if b.maxOpen > 0 {
		timeout = b.maxOpen
		if b.openCount <= 20 {
			timeout = min(timeout, b.openTimeout<<(b.openCount-1))
		}
		timeout = timeout/2 + time.Duration(b.rand()*float64(timeout/2))
	}
	b.openUntil = b.now().Add(timeout)
	b.transition(Open)
	if b.metrics != nil {
		b.metrics.trips.Inc()
	}
}

func (b *Breaker) transition(to State) {
	from := b.state
	b.state = to
	b.generation++
	b.probesInFlight = 0
	b.probesFailed = 0
	if to == Closed {
		b.openCount = 0
		b.failures = 0
		b.entries = nil
	}
	b.wake()
	if b.onChange != nil {
		b.onChange(from, to)
	}
}

func (b *Breaker) wake() {
	close(b.changed)
	b.changed = make(chan struct{})
}

// Describe implements prometheus.Collector. It is a no-op unless WithMetrics
// was set.
func (b *Breaker) Describe(ch chan<- *prometheus.Desc) {
	if b.metrics == nil {
		return
	}
	b.metrics.state.Describe(ch)
	b.metrics.trips.Describe(ch)
}

// Collect implements prometheus.Collector. It is a no-op unless WithMetrics was
// set.
func (b *Breaker) Collect(ch chan<- prometheus.Metric) {
	if b.metrics == nil {
		return
	}
	b.metrics.state.Collect(ch)
	b.metrics.trips.Collect(ch)
}

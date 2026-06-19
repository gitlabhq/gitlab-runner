package circuitbreaker

import (
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
	HalfOpen              // a single trial request is allowed through
)

// Breaker is a circuit breaker. It is safe for concurrent use.
type Breaker struct {
	threshold    int
	openTimeout  time.Duration
	failureGrace time.Duration
	now          func() time.Time

	mu    sync.Mutex
	state State
	// failures counts consecutive failures and is only consulted in the Closed
	// state. The breaker is only ever re-entered into Closed via RecordSuccess,
	// which resets it to 0, so it is deliberately left untouched when opening.
	failures int
	// firstFailureAt is when the current consecutive-failure streak began, used
	// to enforce failureGrace.
	firstFailureAt time.Time
	openedAt       time.Time

	// metrics is non-nil only when WithMetrics is set.
	metrics *breakerMetrics
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
// continuously for at least d, in addition to reaching the failure threshold.
// This stops a burst of simultaneous failures - e.g. every in-flight request
// failing at once when a shared connection drops during a deploy - from tripping
// the breaker, while still tripping on a sustained outage. With the default (0)
// the grace check is a no-op (elapsed is always >= 0), so the breaker trips on
// the consecutive-failure count alone.
func WithFailureGrace(d time.Duration) Option {
	return func(b *Breaker) { b.failureGrace = d }
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
// stays open for openTimeout before allowing a trial request.
func New(threshold int, openTimeout time.Duration, opts ...Option) *Breaker {
	b := &Breaker{
		threshold:   threshold,
		openTimeout: openTimeout,
		now:         time.Now,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Allow reports whether the next request may proceed. While open it returns
// false, except that once the cooldown has elapsed it permits a single trial
// request and moves to half-open.
//
// When Allow grants a half-open trial (returns true while half-open), the caller
// must follow it with RecordSuccess or RecordFailure. If it never does - e.g. it
// panics before recording - the breaker stays half-open and Allow keeps blocking.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case Open:
		if b.now().Sub(b.openedAt) >= b.openTimeout {
			b.state = HalfOpen // hand out a single trial request
			return true
		}
		return false
	case HalfOpen:
		// A trial request is already in flight; block until it resolves.
		return false
	default: // Closed
		return true
	}
}

// RecordSuccess resets the breaker to the closed state and reports whether this
// success recovered it, i.e. closed it from the half-open state after a
// successful trial request.
func (b *Breaker) RecordSuccess() (recovered bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	recovered = b.state == HalfOpen
	b.failures = 0
	b.state = Closed
	return recovered
}

// RecordFailure registers a failure and reports whether it tripped the breaker
// open (either by reaching the threshold from closed, or by failing the
// half-open trial request).
func (b *Breaker) RecordFailure() (tripped bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case HalfOpen:
		b.open() // trial request failed: reopen and restart the cooldown
		return true
	case Open:
		return false
	default: // Closed
		t := b.now()
		if b.failures == 0 {
			b.firstFailureAt = t
		}
		b.failures++
		if b.failures >= b.threshold && t.Sub(b.firstFailureAt) >= b.failureGrace {
			b.open()
			return true
		}
		return false
	}
}

// Abort releases a request permitted by Allow without recording an outcome, for
// when the caller was let through but could not actually attempt the request
// (so the breaker observed nothing about it). A half-open trial reverts to open
// and waits the cooldown again; otherwise it is a no-op. It is not a trip.
func (b *Breaker) Abort() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == HalfOpen {
		b.state = Open
		b.openedAt = b.now()
	}
}

// State returns the current breaker state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// open moves the breaker to the open state and starts the cooldown. It does not
// reset failures (see the field comment): the counter is only read while Closed,
// and is reset there by RecordSuccess. The caller must hold b.mu.
func (b *Breaker) open() {
	b.state = Open
	b.openedAt = b.now()
	if b.metrics != nil {
		b.metrics.trips.Inc()
	}
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

package machine

import (
	"context"
	"errors"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

// A stale heartbeat label means no live manager tracks the instance,
// which lets external cleanup reap orphans without guessing from age.
const heartbeatLabel = "runner_manager_heartbeat"

// A hung label write would hold its semaphore slot forever.
const heartbeatTouchTimeout = 2 * time.Minute

const heartbeatTickPeriod = 15 * time.Second

// heartbeater refreshes the heartbeat label on one runner config's
// machines. The semaphore is shared across the provider's runner
// configs: the operation quota it protects is per process.
type heartbeater struct {
	touch func(ctx context.Context, name string, value string) error

	lock        sync.Mutex
	lastBeat    map[string]time.Time
	inFlight    map[string]context.CancelFunc
	sem         chan struct{}
	interval    time.Duration
	unsupported bool

	tickOnce   sync.Once
	tickPeriod time.Duration

	nowFn  func() time.Time
	randFn func(n int64) int64
}

func newHeartbeater(touch func(ctx context.Context, name string, value string) error, sem chan struct{}) *heartbeater {
	return &heartbeater{
		touch:      touch,
		lastBeat:   map[string]time.Time{},
		inFlight:   map[string]context.CancelFunc{},
		sem:        sem,
		tickPeriod: heartbeatTickPeriod,
		nowFn:      time.Now,
		randFn:     rand.Int63n,
	}
}

// beat registers the machine if it is new and refreshes its heartbeat
// label on a background goroutine if it is due. A failed attempt still
// counts as a beat, so unreachable machines retry next interval instead
// of hot-looping.
func (h *heartbeater) beat(name string, interval time.Duration) {
	h.refresh(name, interval, true)
}

func (h *heartbeater) refresh(name string, interval time.Duration, register bool) {
	if interval <= 0 {
		return
	}

	h.tickOnce.Do(func() { go h.tick() })

	ctx, due := h.schedule(name, interval, register)
	if due {
		go h.write(ctx, name)
	}
}

// schedule records the machine as due and returns the context for its
// write, or false if nothing is to be written now.
func (h *heartbeater) schedule(name string, interval time.Duration, register bool) (context.Context, bool) {
	now := h.nowFn()

	h.lock.Lock()
	defer h.lock.Unlock()

	if h.unsupported {
		return nil, false
	}

	h.interval = interval
	if _, busy := h.inFlight[name]; busy {
		return nil, false
	}

	last, seen := h.lastBeat[name]
	if !seen {
		if !register {
			// forgotten between the ticker's snapshot and this call
			return nil, false
		}

		// random offset, so a manager start doesn't burst one write per machine
		last = now.Add(-time.Duration(h.randFn(int64(interval))))
		h.lastBeat[name] = last
	}

	if now.Sub(last) < interval {
		return nil, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), heartbeatTouchTimeout)
	h.inFlight[name] = cancel

	return ctx, true
}

func (h *heartbeater) write(ctx context.Context, name string) {
	select {
	case h.sem <- struct{}{}:
		defer func() { <-h.sem }()
	case <-ctx.Done():
	}

	var err error
	if ctx.Err() == nil {
		err = h.touch(ctx, name, strconv.FormatInt(h.nowFn().Unix(), 10))
	}

	h.lock.Lock()
	// a machine forgotten mid-beat stays forgotten
	if _, tracked := h.lastBeat[name]; tracked {
		h.lastBeat[name] = h.nowFn()
	}
	if cancel, ok := h.inFlight[name]; ok {
		cancel()
		delete(h.inFlight, name)
	}

	if errors.Is(err, docker.ErrLabelsNotSupported) && !h.unsupported {
		// labeling is optional: disable instead of logging an error
		// per machine per interval on drivers without label support
		h.unsupported = true
		h.lock.Unlock()
		logrus.Warningln("Machine driver does not support labels, disabling heartbeat labels for this runner")

		return
	}
	h.lock.Unlock()

	if err != nil && !errors.Is(err, context.Canceled) {
		logrus.WithError(err).WithField("name", name).
			Warningln("Failed to refresh machine heartbeat label")
	}
}

// tick keeps beating while the acquire path is paused, e.g. when all
// worker slots are busy.
func (h *heartbeater) tick() {
	for range time.Tick(h.tickPeriod) {
		h.lock.Lock()
		interval := h.interval
		names := make([]string, 0, len(h.lastBeat))
		for name := range h.lastBeat {
			names = append(names, name)
		}
		h.lock.Unlock()

		for _, name := range names {
			h.refresh(name, interval, false)
		}
	}
}

// prune drops state for machines no longer in the store.
func (h *heartbeater) prune(active []string) {
	keep := make(map[string]struct{}, len(active))
	for _, name := range active {
		keep[name] = struct{}{}
	}

	h.lock.Lock()
	defer h.lock.Unlock()
	for name := range h.lastBeat {
		if _, ok := keep[name]; !ok {
			delete(h.lastBeat, name)
		}
	}
}

// forget drops one machine's state and cancels a beat that is queued or
// in flight for it, e.g. once the manager stops tracking it.
func (h *heartbeater) forget(name string) {
	h.lock.Lock()
	defer h.lock.Unlock()
	delete(h.lastBeat, name)
	if cancel, ok := h.inFlight[name]; ok {
		cancel()
	}
}

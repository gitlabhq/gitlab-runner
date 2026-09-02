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
	inFlight    map[string]struct{}
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
		inFlight:   map[string]struct{}{},
		sem:        sem,
		tickPeriod: heartbeatTickPeriod,
		nowFn:      time.Now,
		randFn:     rand.Int63n,
	}
}

// beat refreshes the machine's heartbeat label on a background
// goroutine if it is due. A failed attempt still counts as a beat, so
// unreachable machines retry next interval instead of hot-looping.
func (h *heartbeater) beat(name string, interval time.Duration) {
	if interval <= 0 {
		return
	}

	h.lock.Lock()
	if h.unsupported {
		h.lock.Unlock()
		return
	}
	h.lock.Unlock()

	h.tickOnce.Do(func() { go h.tick() })

	now := h.nowFn()

	h.lock.Lock()
	h.interval = interval
	if _, busy := h.inFlight[name]; busy {
		h.lock.Unlock()
		return
	}

	last, seen := h.lastBeat[name]
	if !seen {
		// random offset, so a manager start doesn't burst one write per machine
		last = now.Add(-time.Duration(h.randFn(int64(interval))))
		h.lastBeat[name] = last
	}

	if now.Sub(last) < interval {
		h.lock.Unlock()
		return
	}

	h.inFlight[name] = struct{}{}
	h.lock.Unlock()

	go func() {
		h.sem <- struct{}{}
		defer func() { <-h.sem }()

		ctx, cancel := context.WithTimeout(context.Background(), heartbeatTouchTimeout)
		defer cancel()

		err := h.touch(ctx, name, strconv.FormatInt(h.nowFn().Unix(), 10))

		h.lock.Lock()
		h.lastBeat[name] = h.nowFn()
		delete(h.inFlight, name)

		if errors.Is(err, docker.ErrLabelsNotSupported) && !h.unsupported {
			// labeling is optional: disable instead of logging an error
			// per machine per interval on drivers without label support
			h.unsupported = true
			h.lock.Unlock()
			logrus.Warningln("Machine driver does not support labels, disabling heartbeat labels for this runner")

			return
		}
		h.lock.Unlock()

		if err != nil {
			logrus.WithError(err).WithField("name", name).
				Warningln("Failed to refresh machine heartbeat label")
		}
	}()
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
			h.beat(name, interval)
		}
	}
}

// prune drops state for machines no longer in the store. A beat
// completing after its machine was pruned re-adds one entry; the next
// prune removes it.
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

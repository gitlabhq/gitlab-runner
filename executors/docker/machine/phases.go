package machine

import (
	"sync"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

// stdout and stderr are copied by separate goroutines, hence the lock.
type phaseTracker struct {
	mu        sync.Mutex
	current   string
	started   time.Time
	durations map[string]float64
}

func newPhaseTracker() *phaseTracker {
	return &phaseTracker{durations: map[string]float64{}}
}

func (p *phaseTracker) observe(entry docker.LogEntry) {
	phase, _ := entry.Fields["phase"].(string)
	if phase == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if phase == p.current || entry.Time.Before(p.started) {
		return
	}
	p.close(entry.Time)
	p.current = phase
	p.started = entry.Time
}

func (p *phaseTracker) close(at time.Time) {
	if p.current == "" {
		return
	}
	p.durations[p.current] += max(0, at.Sub(p.started).Seconds())
	p.current = ""
}

func (p *phaseTracker) finish(at time.Time) map[string]float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.close(at)
	return p.durations
}

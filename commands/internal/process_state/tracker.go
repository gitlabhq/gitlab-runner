package process_state

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type ProcessState uint

const (
	ProcessStateStarting = iota
	ProcessStateRunning
	ProcessStateGracefulShutdown
	ProcessStateForcefulShutdown
)

func (p ProcessState) String() string {
	names := map[ProcessState]string{
		ProcessStateStarting:         "starting",
		ProcessStateRunning:          "running",
		ProcessStateGracefulShutdown: "graceful-shutdown",
		ProcessStateForcefulShutdown: "forceful-shutdown",
	}

	name, ok := names[p]
	if !ok {
		return "unknown"
	}

	return name
}

var (
	_ prometheus.Collector = &Tracker{}
)

type Tracker struct {
	state       ProcessState
	stateMetric *prometheus.GaugeVec
	lock        sync.RWMutex
}

func NewTracker() *Tracker {
	t := &Tracker{
		stateMetric: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   "gitlab_runner",
				Subsystem:   "process_state",
				Name:        "info",
				Help:        "This metric provides info about the current state of the GitLab Runner process",
				ConstLabels: nil,
			},
			[]string{"state"},
		),
	}

	t.setState(ProcessStateStarting)

	return t
}

func (p *Tracker) CurrentState() ProcessState {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.state
}

func (p *Tracker) SetRunning() {
	p.setState(ProcessStateRunning)
}

func (p *Tracker) SetGracefulShutdown() {
	p.setState(ProcessStateGracefulShutdown)
}

func (p *Tracker) SetForcefulShutdown() {
	p.setState(ProcessStateForcefulShutdown)
}

func (p *Tracker) setState(state ProcessState) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.state = state

	p.stateMetric.Reset()
	p.stateMetric.WithLabelValues(state.String()).Set(1)
}

func (p *Tracker) Describe(ch chan<- *prometheus.Desc) {
	p.stateMetric.Describe(ch)
}

func (p *Tracker) Collect(ch chan<- prometheus.Metric) {
	p.stateMetric.Collect(ch)
}

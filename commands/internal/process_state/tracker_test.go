//go:build !integration

package process_state

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessState_String(t *testing.T) {
	tests := map[string]ProcessState{
		"starting":          ProcessStateStarting,
		"running":           ProcessStateRunning,
		"graceful-shutdown": ProcessStateGracefulShutdown,
		"forceful-shutdown": ProcessStateForcefulShutdown,
		"unknown":           ProcessState(99),
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.Equal(t, tn, tt.String())
		})
	}
}

func TestNewTracker(t *testing.T) {
	tracker := NewTracker()

	require.NotNil(t, tracker)
	assert.Equal(t, ProcessState(ProcessStateStarting), tracker.state)
	assert.NotNil(t, tracker.stateMetric)
}

func TestTracker_StateTransitions(t *testing.T) {
	tests := map[string]struct {
		setter   func(*Tracker)
		expected ProcessState
	}{
		"SetRunning": {
			setter:   (*Tracker).SetRunning,
			expected: ProcessStateRunning,
		},
		"SetGracefulShutdown": {
			setter:   (*Tracker).SetGracefulShutdown,
			expected: ProcessStateGracefulShutdown,
		},
		"SetForcefulShutdown": {
			setter:   (*Tracker).SetForcefulShutdown,
			expected: ProcessStateForcefulShutdown,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tracker := NewTracker()
			tt.setter(tracker)
			assert.Equal(t, tt.expected, tracker.state)
		})
	}
}

func TestTracker_Collect(t *testing.T) {
	t.Run("fresh tracker reports starting", func(t *testing.T) {
		tracker := NewTracker()

		ch := make(chan prometheus.Metric, 10)
		tracker.Collect(ch)
		close(ch)

		// Drain so the gauge series is populated for the label-based read below.
		var collected int
		for range ch {
			collected++
		}

		assert.Equal(t, 1, collected, "Collect should emit exactly one metric")
		assert.Equal(t, float64(1), testutil.ToFloat64(tracker.stateMetric.WithLabelValues("starting")))
	})

	t.Run("after SetRunning reports running", func(t *testing.T) {
		tracker := NewTracker()
		tracker.SetRunning()

		ch := make(chan prometheus.Metric, 10)
		tracker.Collect(ch)
		close(ch)

		var collected int
		for range ch {
			collected++
		}

		assert.Equal(t, 1, collected, "Collect should emit exactly one metric")
		assert.Equal(t, float64(1), testutil.ToFloat64(tracker.stateMetric.WithLabelValues("running")))
	})

	t.Run("end-to-end via prometheus.Registry", func(t *testing.T) {
		tracker := NewTracker()
		tracker.SetGracefulShutdown()

		registry := prometheus.NewRegistry()
		require.NoError(t, registry.Register(tracker))

		families, err := registry.Gather()
		require.NoError(t, err)
		require.Len(t, families, 1)
		assert.Equal(t, "gitlab_runner_process_state_info", families[0].GetName())
	})
}

func TestTracker_SetStateRace(t *testing.T) {
	const (
		writers       = 100
		iterations    = 1000
		readers       = 4
		readerRepeats = 1000
	)

	setters := []func(*Tracker){
		(*Tracker).SetRunning,
		(*Tracker).SetGracefulShutdown,
		(*Tracker).SetForcefulShutdown,
	}

	start := make(chan struct{})
	var wg sync.WaitGroup

	tracker := NewTracker()

	for i := 0; i < writers; i++ {
		wg.Go(func() {
			setter := setters[i%len(setters)]
			<-start
			for j := 0; j < iterations; j++ {
				setter(tracker)
			}
		})
	}

	for i := 0; i < readers; i++ {
		wg.Go(func() {
			<-start
			for j := 0; j < readerRepeats; j++ {
				tracker.CurrentState()
			}
		})
	}

	close(start)
	wg.Wait()
}

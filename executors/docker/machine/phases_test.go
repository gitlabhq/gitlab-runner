//go:build !integration

package machine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

func phaseEntry(at time.Time, phase string) docker.LogEntry {
	e := docker.LogEntry{Time: at, Msg: "x"}
	if phase != "" {
		e.Fields = map[string]any{"phase": phase}
	}
	return e
}

func TestPhaseTracker(t *testing.T) {
	t0 := time.Date(2026, 9, 8, 14, 38, 35, 0, time.UTC)
	at := func(s float64) time.Time { return t0.Add(time.Duration(s * float64(time.Second))) }

	tests := map[string]struct {
		entries []docker.LogEntry
		finish  time.Time
		want    map[string]float64
	}{
		"text mode: no phase fields, nothing recorded": {
			entries: []docker.LogEntry{phaseEntry(at(0), ""), phaseEntry(at(5), "")},
			finish:  at(10),
			want:    map[string]float64{},
		},
		"consecutive phases are diffed on their first line": {
			entries: []docker.LogEntry{
				phaseEntry(at(0), "driver_create"),
				phaseEntry(at(2), "driver_create"),
				phaseEntry(at(5), ""),
				phaseEntry(at(15), "wait_ssh"),
				phaseEntry(at(231), "detect_os"),
			},
			finish: at(233),
			want:   map[string]float64{"driver_create": 15, "wait_ssh": 216, "detect_os": 2},
		},
		"open phase is closed at finish (timeout / kill)": {
			entries: []docker.LogEntry{phaseEntry(at(0), "wait_ssh")},
			finish:  at(3600),
			want:    map[string]float64{"wait_ssh": 3600},
		},
		"late line from the previous phase is ignored": {
			entries: []docker.LogEntry{
				phaseEntry(at(0), "wait_ssh"),
				phaseEntry(at(10.001), "detect_os"),
				phaseEntry(at(10.000), "wait_ssh"),
				phaseEntry(at(200), "provision"),
			},
			finish: at(201),
			want:   map[string]float64{"wait_ssh": 10.001, "detect_os": 189.999, "provision": 1},
		},
		"repeated phase accumulates": {
			entries: []docker.LogEntry{
				phaseEntry(at(0), "wait_ssh"),
				phaseEntry(at(1), "detect_os"),
				phaseEntry(at(2), "wait_ssh"),
			},
			finish: at(4),
			want:   map[string]float64{"wait_ssh": 3, "detect_os": 1},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			p := newPhaseTracker()
			for _, e := range tc.entries {
				p.observe(e)
			}
			assert.Equal(t, tc.want, p.finish(tc.finish))
		})
	}
}

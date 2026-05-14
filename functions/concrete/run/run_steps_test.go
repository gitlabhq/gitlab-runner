//go:build !integration

package run

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/step-runner/pkg/api/client"
	"gitlab.com/gitlab-org/step-runner/schema/v1"
)

// TestInterpretRunResult pins the message/empty-message fallback and the
// runErr/flushErr precedence — the branches without a natural consumer test
// elsewhere. The cancellation-wrap contract is pinned in TestStatusFromError
// (runner_test.go) where the consumer of ErrJobCanceled actually lives.
func TestInterpretRunResult(t *testing.T) {
	errDial := errors.New("connection refused")
	errFlush := errors.New("flush broken pipe")

	tests := []struct {
		name string

		status   client.Status
		runErr   error
		flushErr error

		// At most one of these is set per case:
		wantNil         bool
		wantContains    string // substring that must appear in err.Error().
		wantNotContains string // substring that must NOT appear (used for clobber checks).
	}{
		{
			name:    "success returns nil",
			status:  client.Status{State: client.StateSuccess},
			wantNil: true,
		},
		{
			name:         "cancelled includes server message",
			status:       client.Status{State: client.StateCancelled, Message: "user hit cancel"},
			wantContains: "user hit cancel",
		},
		{
			name:         "failure with message surfaces message verbatim",
			status:       client.Status{State: client.StateFailure, Message: "step exited 2"},
			wantContains: "step exited 2",
		},
		{
			name:         "failure with empty message falls back to State.String()",
			status:       client.Status{State: client.StateFailure},
			wantContains: client.StateFailure.String(),
		},
		{
			name:         "flush error surfaces when runErr is nil",
			status:       client.Status{State: client.StateSuccess},
			flushErr:     errFlush,
			wantContains: errFlush.Error(),
		},
		{
			name:         "runErr wins over flushErr — flushErr must not clobber",
			status:       client.Status{State: client.StateSuccess},
			runErr:       errDial,
			flushErr:     errFlush,
			wantContains: errDial.Error(),
		},
		{
			name:            "runErr wins over flushErr — flushErr must not appear",
			status:          client.Status{State: client.StateSuccess},
			runErr:          errDial,
			flushErr:        errFlush,
			wantNotContains: errFlush.Error(),
		},
		{
			name:         "transport error is wrapped with context prefix",
			status:       client.Status{State: client.StateSuccess},
			runErr:       errDial,
			wantContains: "running user steps via step-runner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := interpretRunResult(tt.status, tt.runErr, tt.flushErr)

			switch {
			case tt.wantNil:
				assert.NoError(t, got)
			case tt.wantContains != "":
				require.Error(t, got)
				assert.Contains(t, got.Error(), tt.wantContains)
			case tt.wantNotContains != "":
				require.Error(t, got)
				assert.NotContains(t, got.Error(), tt.wantNotContains)
			}
		})
	}
}

// TestBuildStepsYAML_Preamble pins the "{}\n---\n" preamble step-runner's YAML
// parser requires. The idiomatic refactor (yaml.NewEncoder(...).Encode) would
// pass a round-trip test but drop the preamble — this assert is what catches
// that.
func TestBuildStepsYAML_Preamble(t *testing.T) {
	script := "echo hi"
	name := "greet"
	out, err := buildStepsYAML([]schema.Step{{Name: &name, Script: &script}})
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(out, "{}\n---\n"),
		"step-runner's parser requires an empty front-matter document; missing preamble silently breaks dispatch.\ngot: %q", out)
}

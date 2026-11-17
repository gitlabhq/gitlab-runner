//go:build !integration

package run

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	"google.golang.org/protobuf/types/known/structpb"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
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

// TestBuildRunRequest_Preamble pins the "{}\n---\n" preamble step-runner's YAML
// parser requires. The idiomatic refactor (yaml.NewEncoder(...).Encode) would
// pass a round-trip test but drop the preamble; this assert is what catches
// that.
func TestBuildRunRequest_Preamble(t *testing.T) {
	script := "echo hi"
	name := "greet"
	req, err := (&Runner{config: &Config{}, env: &env.Env{}}).buildRunRequest([]schema.Step{{Name: &name, Script: &script}})
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(req.Steps, "{}\n---\n"),
		"step-runner's parser requires an empty front-matter document; missing preamble silently breaks dispatch.\ngot: %q", req.Steps)
}

// TestBuildRunRequest_ScriptSurvivesStepRunnerDecoding guards the nested run:
// path against the multi-line string bug: a script that starts with a blank
// line or a tab failed to decode, and one indented on every line lost its
// indentation.
func TestBuildRunRequest_ScriptSurvivesStepRunnerDecoding(t *testing.T) {
	tests := map[string]string{
		"leading newline":           "\necho hi\necho there\n",
		"leading tab":               "\techo hi\n\techo there\n",
		"indented on every line":    "  echo hi\n  echo there\n",
		"leading newline and space": "\n  echo hi\n",
	}

	for name, script := range tests {
		t.Run(name, func(t *testing.T) {
			stepName := "user"
			r := &Runner{config: &Config{}, env: &env.Env{}}
			req, err := r.buildRunRequest([]schema.Step{{Name: &stepName, Script: &script}})
			require.NoError(t, err)

			var spec schema.Spec
			var got schema.Step
			decoder := yaml.NewDecoder(strings.NewReader(req.Steps))
			decoder.KnownFields(true)
			require.NoError(t, decoder.Decode(&spec))
			require.NoError(t, decoder.Decode(&got))

			require.Len(t, got.Run, 1)
			require.NotNil(t, got.Run[0].Script)
			assert.Equal(t, script, *got.Run[0].Script)
		})
	}
}

// TestBuildRunRequest_ForwardsJobVars guards the fix for nested run: steps
// failing with "attribute not found". The nested job resolves ${{ vars.* }} /
// ${{ job.* }} (including step/func references) from RunRequest.Variables, not
// Env, so the dispatch must forward the job variables.
func TestBuildRunRequest_ForwardsJobVars(t *testing.T) {
	name, script := "step", "true"
	r := &Runner{
		config: &Config{ID: 42},
		env: &env.Env{
			WorkingDir: "/work",
			Env:        map[string]string{"PATH": "/usr/bin"},
			JobVars: map[string]*structpb.Value{
				"CHANGELOG_FUNC_IMG": structpb.NewStringValue("registry.example/changelog:1"),
				"CI_COMMIT_REF_NAME": structpb.NewStringValue("main"),
			},
		},
	}

	req, err := r.buildRunRequest([]schema.Step{{Name: &name, Script: &script}})
	require.NoError(t, err)

	got := map[string]string{}
	for _, v := range req.Variables {
		got[v.Key] = v.Value
	}
	assert.Equal(t, "registry.example/changelog:1", got["CHANGELOG_FUNC_IMG"],
		"job variables must be forwarded as RunRequest.Variables for vars.*/job.* to resolve")
	assert.Equal(t, "main", got["CI_COMMIT_REF_NAME"])
}

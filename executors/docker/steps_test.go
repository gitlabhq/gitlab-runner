//go:build !integration

package docker

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadyWriter(t *testing.T) {
	tests := []struct {
		name        string
		writes      []string
		expectReady bool
	}{
		{
			name:        "single write with marker",
			writes:      []string{"step-runner is ready.\n"},
			expectReady: true,
		},
		{
			name:        "split write",
			writes:      []string{"step-runner is ", "ready.\n"},
			expectReady: true,
		},
		{
			name:        "marker in middle",
			writes:      []string{"prefix\nstep-runner is ready.\nsuffix"},
			expectReady: true,
		},
		{
			name:        "byte by byte",
			writes:      strings.Split("step-runner is ready.\n", ""),
			expectReady: true,
		},
		{
			name:        "false start then match",
			writes:      []string{"step-runner is not ready\nstep-runner is ready.\n"},
			expectReady: true,
		},
		{
			name:        "multiple markers",
			writes:      []string{"step-runner is ready.\nstep-runner is ready.\n"},
			expectReady: true,
		},
		{
			name:        "no match",
			writes:      []string{"no match here"},
			expectReady: false,
		},
		{
			name:        "partial match only",
			writes:      []string{"step-runner is"},
			expectReady: false,
		},
		{
			name:        "empty write",
			writes:      []string{""},
			expectReady: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w, ready := newReadyWriter(t.Context(), &buf)

			for _, s := range tt.writes {
				_, err := w.Write([]byte(s))
				require.NoError(t, err)
			}

			select {
			case <-ready:
				if !tt.expectReady {
					t.Fatal("expected channel to be closed")
				}
			default:
				if tt.expectReady {
					t.Fatal("expected channel to be closed")
				}
			}

			expected := strings.Join(tt.writes, "")
			assert.Equal(t, expected, buf.String(), "all data should be proxied")
		})
	}
}

func TestReadyWriter_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var buf bytes.Buffer
	_, ready := newReadyWriter(ctx, &buf)

	cancel()

	select {
	case <-ready:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected channel to close on context cancellation")
	}
}

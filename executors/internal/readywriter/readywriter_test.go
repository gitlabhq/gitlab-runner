//go:build !integration

package readywriter_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/executors/internal/readywriter"
)

func TestReadyWriter(t *testing.T) {
	tests := []struct {
		name           string
		writes         []string
		expectReady    bool
		expectedSocket string
	}{
		{
			name:           "single write with marker",
			writes:         []string{"step-runner is listening on socket /tmp/ready.sock\n"},
			expectReady:    true,
			expectedSocket: "/tmp/ready.sock",
		},
		{
			name:           "split write",
			writes:         []string{"step-runner is listening on socket ", "/tmp/split.sock", "\n"},
			expectReady:    true,
			expectedSocket: "/tmp/split.sock",
		},
		{
			name:           "marker in middle",
			writes:         []string{"prefix\nstep-runner is listening on socket /tmp/mid.sock\nsuffix"},
			expectReady:    true,
			expectedSocket: "/tmp/mid.sock",
		},
		{
			name:           "byte by byte",
			writes:         strings.Split("step-runner is listening on socket /tmp/bytes.sock\n", ""),
			expectReady:    true,
			expectedSocket: "/tmp/bytes.sock",
		},
		{
			name:           "false start then match",
			writes:         []string{"step-runner is not ready\nstep-runner is listening on socket /tmp/false.sock\n"},
			expectReady:    true,
			expectedSocket: "/tmp/false.sock",
		},
		{
			name:           "multiple markers",
			writes:         []string{"step-runner is listening on socket /tmp/first.sock\nstep-runner is listening on socket /tmp/second.sock\n"},
			expectReady:    true,
			expectedSocket: "/tmp/first.sock",
		},
		{
			name:        "no match",
			writes:      []string{"no match here"},
			expectReady: false,
		},
		{
			name:        "partial match only",
			writes:      []string{"step-runner is ready. socket:"},
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
			w, ready := readywriter.New(t.Context(), &buf)

			for _, s := range tt.writes {
				_, err := w.Write([]byte(s))
				require.NoError(t, err)
			}

			select {
			case socket, ok := <-ready:
				if !tt.expectReady {
					t.Fatal("expected channel to be open")
				}
				require.True(t, ok, "expected channel to be closed after write")
				assert.Equal(t, tt.expectedSocket, socket)
			default:
				if tt.expectReady {
					t.Fatal("expected channel to be closed")
				}
			}
			if tt.expectReady {
				_, ok := <-ready
				require.False(t, ok, "expected channel to be closed after first read")
			}

			expected := strings.Join(tt.writes, "")
			assert.Equal(t, expected, buf.String(), "all data should be proxied")
		})
	}
}

func TestReadyWriter_SocketLengthLimit(t *testing.T) {
	var buf bytes.Buffer
	w, ready := readywriter.New(t.Context(), &buf)

	marker := "step-runner is listening on socket "
	_, err := w.Write([]byte(marker))
	require.NoError(t, err)

	longPath := strings.Repeat("a", 4*1024+1)
	_, err = w.Write([]byte(longPath))
	require.NoError(t, err)

	select {
	case socket, ok := <-ready:
		assert.False(t, ok, "expected channel to be closed without sending")
		assert.Empty(t, socket)
	default:
		t.Fatal("expected channel to be closed after exceeding max socket length")
	}
}

func TestReadyWriter_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var buf bytes.Buffer
	_, ready := readywriter.New(ctx, &buf)

	cancel()

	select {
	case <-ready:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected channel to close on context cancellation")
	}
}

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
		// expectedOutput is what reaches the underlying writer once all
		// writes are done: everything except the ready line.
		expectedOutput string
		// heldBack is a trailing marker prefix (or an unfinished ready line)
		// that only Close can settle; it must reach the writer then.
		heldBack string
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
			expectedOutput: "prefix\nsuffix",
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
			expectedOutput: "step-runner is not ready\n",
		},
		{
			name:           "false start split across writes",
			writes:         []string{"step-run", "ner is not ready\n", "step-runner is listening on socket /tmp/split-false.sock\n"},
			expectReady:    true,
			expectedSocket: "/tmp/split-false.sock",
			expectedOutput: "step-runner is not ready\n",
		},
		{
			name:           "marker restarts inside a longer false start",
			writes:         []string{"step-runner is listep-runner is listening on socket /tmp/restart.sock\n"},
			expectReady:    true,
			expectedSocket: "/tmp/restart.sock",
			expectedOutput: "step-runner is li",
		},
		{
			name:           "multiple markers",
			writes:         []string{"step-runner is listening on socket /tmp/first.sock\nstep-runner is listening on socket /tmp/second.sock\n"},
			expectReady:    true,
			expectedSocket: "/tmp/first.sock",
			expectedOutput: "step-runner is listening on socket /tmp/second.sock\n",
		},
		{
			name:           "no match",
			writes:         []string{"no match here"},
			expectedOutput: "no match here",
		},
		{
			name:           "partial match only",
			writes:         []string{"step-runner is ready. socket:"},
			expectedOutput: "step-runner is ready. socket:",
		},
		{
			name:           "trailing marker prefix is held until Close",
			writes:         []string{"oops\nstep-runner is"},
			expectedOutput: "oops\n",
			heldBack:       "step-runner is",
		},
		{
			name:     "marker without a newline is forwarded on Close",
			writes:   []string{"step-runner is listening on socket /tmp/partial.sock"},
			heldBack: "step-runner is listening on socket /tmp/partial.sock",
		},
		{
			name:   "empty write",
			writes: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w, ready := readywriter.New(t.Context(), &buf)

			for _, s := range tt.writes {
				n, err := w.Write([]byte(s))
				require.NoError(t, err)
				assert.Equal(t, len(s), n, "held-back bytes count as written")
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

			assert.Equal(t, tt.expectedOutput, buf.String(), "everything but the ready line is proxied")

			require.NoError(t, w.Close())
			assert.Equal(t, tt.expectedOutput+tt.heldBack, buf.String(), "Close flushes what was held back")
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

	assert.Equal(t, marker+longPath, buf.String(), "an over-long line is not a handshake and is proxied as output")

	_, err = w.Write([]byte("\nmore output\n"))
	require.NoError(t, err)
	assert.Equal(t, marker+longPath+"\nmore output\n", buf.String(), "output after giving up is proxied untouched")
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

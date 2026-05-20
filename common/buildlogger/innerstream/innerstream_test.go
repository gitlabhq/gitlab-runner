//go:build !integration

package innerstream

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// header builds a fixed-prefix header in the format the inner step-runner's
// timestamper emits. Tests fabricate the wire format directly rather than
// spinning up a real inner step-runner.
func header(streamType byte, partial bool) []byte {
	h := make([]byte, headerLen)
	// Static separators in the timestamp field.
	copy(h, "2026-05-05T12:00:00.000000Z ")
	h[28] = '0'
	h[29] = '0'
	h[streamTypeOff] = streamType
	h[lineTypeOff] = lineFull
	if partial {
		h[lineTypeOff] = linePartial
	}
	return h
}

func wireLine(streamType byte, partial bool, body string) []byte {
	out := header(streamType, partial)
	out = append(out, []byte(body)...)
	out = append(out, '\n')
	return out
}

func TestSplitter_Parse(t *testing.T) {
	cases := []struct {
		name           string
		wire           []byte
		expectedStdout string
		expectedStderr string
	}{
		{
			name: "routes by stream type",
			wire: bytes.Join([][]byte{
				wireLine(streamStdout, false, "out-1"),
				wireLine(streamStderr, false, "err-1"),
				wireLine(streamStdout, false, "out-2"),
			}, nil),
			expectedStdout: "out-1\nout-2\n",
			expectedStderr: "err-1\n",
		},
		{
			// Inner buffer-overflow: a single logical line was split
			// across two physical lines on the wire. The outer masker
			// matches phrases byte-by-byte and resets on injected '\n',
			// so "secret-token" must land contiguous in the splitter
			// output.
			name: "partial marker merges buffer-overflow split",
			wire: bytes.Join([][]byte{
				wireLine(streamStdout, false, "lots of preamble... secre"),
				wireLine(streamStdout, true, "t-token continues"),
			}, nil),
			expectedStdout: "lots of preamble... secret-token continues\n",
		},
		{
			// stdout-Full then stderr-Full then stdout-Partial: the '+'
			// on the final line is a continuation of the FIRST stdout
			// line, even though an unrelated stderr line landed in
			// between. The stderr line must keep its '\n'.
			name: "partial marker only affects same stream",
			wire: bytes.Join([][]byte{
				wireLine(streamStdout, false, "stdout-A"),
				wireLine(streamStderr, false, "stderr-B"),
				wireLine(streamStdout, true, "-continued"),
			}, nil),
			expectedStdout: "stdout-A-continued\n",
			expectedStderr: "stderr-B\n",
		},
		{
			// A header followed by just '\n' is a legitimate empty line
			// (e.g. user code printed ""). The header bytes must be
			// stripped but the empty-line semantic preserved.
			name:           "header-only line emits empty line",
			wire:           append(header(streamStdout, false), '\n'),
			expectedStdout: "\n",
		},
		{
			// Truly malformed: shorter than the header. Must not surface
			// to the trace.
			name: "shorter than header is dropped",
			wire: []byte("garbage\n"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			s := New(&stdout, &stderr)

			_, err := s.Write(tc.wire)
			require.NoError(t, err)
			require.NoError(t, s.Flush())

			assert.Equal(t, tc.expectedStdout, stdout.String())
			assert.Equal(t, tc.expectedStderr, stderr.String())
		})
	}
}

// TestSplitter_HandlesPartialReads exercises the Write byte-streaming path
// rather than the parse logic, so it stays out of the table.
func TestSplitter_HandlesPartialReads(t *testing.T) {
	var stdout, stderr bytes.Buffer
	s := New(&stdout, &stderr)

	wire := wireLine(streamStdout, false, "hello world")

	// Feed one byte at a time to make sure inBuf accumulation works.
	for _, b := range wire {
		_, err := s.Write([]byte{b})
		require.NoError(t, err)
	}
	require.NoError(t, s.Flush())

	assert.Equal(t, "hello world\n", stdout.String())
	assert.Empty(t, stderr.String())
}

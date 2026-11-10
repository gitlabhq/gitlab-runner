//go:build !integration

package kubernetes

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	logPauseMarker  = `{"script": "some/script"}`
	logResumeMarker = `{"command_exit_code": 0}`
	someTimestamp   = "2024-07-25T09:50:54.008163908Z"
)

type fakeEntrypointForwarderSink struct {
	bytes.Buffer

	writeCallCount int
	closed         bool

	writeError error
	closeError error
}

func (s *fakeEntrypointForwarderSink) Write(p []byte) (int, error) {
	s.writeCallCount++
	if s.writeError != nil {
		return 0, s.writeError
	}

	return s.Buffer.Write(p)
}

func (s *fakeEntrypointForwarderSink) Close() error {
	s.closed = true
	return s.closeError
}

// containerLogTimestamps are timestamps in front of a log line. The kubelet writes the time the container runtime
// recorded, in the runtime's zone, so on a node whose time zone is not UTC it ends in an offset instead of Z. An
// explicit zero offset is valid RFC 3339 as well.
var containerLogTimestamps = []struct {
	name      string
	timestamp string
}{
	{name: "UTC", timestamp: someTimestamp},
	{name: "positive offset", timestamp: "2024-07-25T11:50:54.008163908+02:00"},
	{name: "zero offset", timestamp: "2024-07-25T09:50:54.008163908+00:00"},
	{name: "negative offset", timestamp: "2024-07-25T04:50:54.008163908-05:00"},
}

// timestampBuffer prefixes every write with timestamp and a space, the way the kubelet prefixes every log line.
// Without a timestamp it uses someTimestamp.
type timestampBuffer struct {
	bytes.Buffer

	timestamp string
}

func (b *timestampBuffer) Write(p []byte) (int, error) {
	timestamp := b.timestamp
	if timestamp == "" {
		timestamp = someTimestamp
	}

	return fmt.Fprintf(&b.Buffer, "%s %s", timestamp, p)
}

func TestEntrypointLogForwarder(t *testing.T) {
	t.Run("forward, pause, resume", func(t *testing.T) {
		for _, tc := range containerLogTimestamps {
			t.Run(tc.name, func(t *testing.T) {
				sink := &fakeEntrypointForwarderSink{}

				lf := &entrypointLogForwarder{
					Sink: sink,
				}

				buf := timestampBuffer{timestamp: tc.timestamp}

				fmt.Fprintln(&buf, "1")
				fmt.Fprintln(&buf, "2")

				fmt.Fprintln(&buf, logPauseMarker)

				fmt.Fprintln(&buf, "3")
				fmt.Fprintln(&buf, "4")

				fmt.Fprintln(&buf, logResumeMarker)

				fmt.Fprintln(&buf, "5")

				_, err := io.Copy(lf, &buf)
				require.NoError(t, err)

				expectedBuf := timestampBuffer{timestamp: tc.timestamp}
				fmt.Fprintln(&expectedBuf, "1")
				fmt.Fprintln(&expectedBuf, "2")
				fmt.Fprintln(&expectedBuf, "5")

				assert.Equal(t, expectedBuf.String(), sink.String())
			})
		}
	})

	t.Run("marker after something other than a timestamp", func(t *testing.T) {
		sink := &fakeEntrypointForwarderSink{}

		lf := &entrypointLogForwarder{
			Sink: sink,
		}

		// the first word is followed by a space, like a timestamp would be, but it is not one, so the marker is just
		// output of the build container
		input := "not-a-timestamp " + logPauseMarker + "\n" +
			"2024-07-25 09:50:54 " + logPauseMarker + "\n" +
			"after\n"

		_, err := io.Copy(lf, bytes.NewBufferString(input))
		require.NoError(t, err)

		assert.Equal(t, input, sink.String())
	})

	t.Run("lines too short for a timestamp and a space", func(t *testing.T) {
		sink := &fakeEntrypointForwarderSink{}

		lf := &entrypointLogForwarder{
			Sink: sink,
		}

		input := "\n" +
			"short\n" +
			// 30 bytes with the newline: as long as a UTC timestamp, one byte short of a timestamp and its space
			"2024-07-25T09:50:54.008163908\n"

		_, err := io.Copy(lf, bytes.NewBufferString(input))
		require.NoError(t, err)

		assert.Equal(t, input, sink.String())
	})

	t.Run("multiple writes, one line", func(t *testing.T) {
		sink := &fakeEntrypointForwarderSink{}

		lf := &entrypointLogForwarder{
			Sink: sink,
		}

		var buf timestampBuffer

		fmt.Fprint(&buf, "cat")
		fmt.Fprint(&buf, "badger")
		fmt.Fprintln(&buf, "axolotl")

		expected := buf.String()

		_, err := io.Copy(lf, &buf)
		require.NoError(t, err)

		assert.Equal(t, expected, sink.String())
		assert.Equal(t, 1, sink.writeCallCount, "expected write on the sink to be called once, got called %d times", sink.writeCallCount)
	})

	t.Run("one write, multiple lines", func(t *testing.T) {
		sink := &fakeEntrypointForwarderSink{}

		lf := &entrypointLogForwarder{
			Sink: sink,
		}

		var buf timestampBuffer

		fmt.Fprintln(&buf, "cat\nbadger\naxolotl")

		expected := buf.String()

		_, err := io.Copy(lf, &buf)
		require.NoError(t, err)

		assert.Equal(t, expected, sink.String())

		assert.Equal(t, 3, sink.writeCallCount, "expected write on the sink to be called 3 times, got called %d times", sink.writeCallCount)
	})

	t.Run("with timestamp", func(t *testing.T) {
		for _, tc := range containerLogTimestamps {
			t.Run(tc.name, func(t *testing.T) {
				sink := &fakeEntrypointForwarderSink{}

				lf := &entrypointLogForwarder{
					Sink: sink,
				}

				buf := timestampBuffer{timestamp: tc.timestamp}

				fmt.Fprintln(&buf, "cow")
				fmt.Fprintln(&buf, logPauseMarker)
				fmt.Fprintln(&buf, "no cow")
				// we are missing the space between the TS and the marker, thus the marker won't do anything
				// write directly to the underlying Buffer to avoid writing the timestamp correctly
				fmt.Fprintln(&buf.Buffer, tc.timestamp+logResumeMarker)
				fmt.Fprintln(&buf, "still no cow")
				fmt.Fprintln(&buf, logResumeMarker)
				fmt.Fprintln(&buf, "sheep")

				_, err := io.Copy(lf, &buf)
				require.NoError(t, err)

				expectedBuf := timestampBuffer{timestamp: tc.timestamp}
				fmt.Fprintln(&expectedBuf, "cow")
				fmt.Fprintln(&expectedBuf, "sheep")

				assert.Equal(t, expectedBuf.String(), sink.String())
			})
		}
	})

	t.Run("flushes on close", func(t *testing.T) {
		sink := &fakeEntrypointForwarderSink{}

		lf := &entrypointLogForwarder{
			Sink: sink,
		}

		var buf timestampBuffer

		fmt.Fprintln(&buf, "armadillo")
		fmt.Fprint(&buf, "cricket")

		_, err := io.Copy(lf, &buf)
		require.NoError(t, err)

		var expectedBuf timestampBuffer
		fmt.Fprintln(&expectedBuf, "armadillo")

		assert.Equal(t, expectedBuf.String(), sink.String())
		assert.Equal(t, 1, sink.writeCallCount, "expected write on the sink to be called once, got called %d times", sink.writeCallCount)

		require.NoError(t, lf.Close())

		fmt.Fprint(&expectedBuf, "cricket")

		assert.Equal(t, expectedBuf.String(), sink.String())
		assert.Equal(t, 2, sink.writeCallCount, "expected write on the sink to be called a second time, got called %d times", sink.writeCallCount)
	})

	t.Run("closes sink on close", func(t *testing.T) {
		sink := &fakeEntrypointForwarderSink{}

		lf := &entrypointLogForwarder{
			Sink: sink,
		}

		require.NoError(t, lf.Close())
		assert.True(t, sink.closed)
	})

	t.Run("closes sink on close when flush fails", func(t *testing.T) {
		writeErr := errors.New("write error")
		sink := &fakeEntrypointForwarderSink{
			writeError: writeErr,
		}

		lf := &entrypointLogForwarder{
			Sink: sink,
		}

		// just write some data we can flush after
		_, err := lf.Write([]byte("hello"))
		require.NoError(t, err)

		err = lf.Close()
		assert.ErrorIs(t, err, writeErr)
		assert.Equal(t, 1, sink.writeCallCount, "expected write on the sink to be called once, got called %d times", sink.writeCallCount)
		assert.True(t, sink.closed)
	})

	t.Run("flush doesn't fail close returns error", func(t *testing.T) {
		closeErr := errors.New("close error")
		sink := &fakeEntrypointForwarderSink{
			closeError: closeErr,
		}

		lf := &entrypointLogForwarder{
			Sink: sink,
		}

		require.Error(t, lf.Close())
		assert.True(t, sink.closed)
	})
}

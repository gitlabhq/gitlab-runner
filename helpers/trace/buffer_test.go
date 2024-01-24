//go:build !integration

package trace

import (
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTraceLimit(t *testing.T) {
	traceMessage := "This is the long message"

	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	buffer.SetLimit(10)
	assert.Equal(t, 0, buffer.Size())

	for i := 0; i < 100; i++ {
		n, err := buffer.Write([]byte(traceMessage))
		require.NoError(t, err)
		require.Greater(t, n, 0)
	}

	buffer.Finish()

	content, err := buffer.Bytes(0, 1000)
	require.NoError(t, err)

	expectedContent := "This is th\n" +
		"\x1b[33;1mJob's log exceeded limit of 10 bytes.\n" +
		"Job execution will continue but no more output will be collected.\x1b[0;m\n"
	assert.Equal(t, len(expectedContent), buffer.Size(), "unexpected buffer size")
	assert.Equal(t, "crc32:295921ca", buffer.Checksum())
	assert.Equal(t, expectedContent, string(content))
}

func TestTraceLimitEnsureValidUTF8(t *testing.T) {
	tests := map[string]struct {
		traceMessage     string
		limit            int
		expectedContent  string
		expectedChecksum string
	}{
		"1-byte UTF-8 characters (ASCII text)": {
			traceMessage: "0123456789",
			limit:        10,
			expectedContent: "0123456789\n" +
				"\x1b[33;1mJob's log exceeded limit of 10 bytes.\n" +
				"Job execution will continue but no more output will be collected.\x1b[0;m\n",
			expectedChecksum: "crc32:d4b99d81",
		},
		"2-byte UTF-8 characters": {
			traceMessage: "ǲ",
			limit:        5,
			expectedContent: "ǲǲ\n" +
				"\x1b[33;1mJob's log exceeded limit of 5 bytes.\n" +
				"Job execution will continue but no more output will be collected.\x1b[0;m\n",
			expectedChecksum: "crc32:318d2180",
		},
		"2-byte UTF-8 characters on even boundary": {
			traceMessage: "ǲ",
			limit:        6,
			expectedContent: "ǲǲǲ\n" +
				"\x1b[33;1mJob's log exceeded limit of 6 bytes.\n" +
				"Job execution will continue but no more output will be collected.\x1b[0;m\n",
			expectedChecksum: "crc32:8c2a1eda",
		},
		"3-byte UTF-8 characters": {
			traceMessage: "─",
			limit:        20,
			expectedContent: "──────\n" +
				"\x1b[33;1mJob's log exceeded limit of 20 bytes.\n" +
				"Job execution will continue but no more output will be collected.\x1b[0;m\n",
			expectedChecksum: "crc32:f187099c",
		},
		"3-byte UTF-8 characters with a limit of 1 byte": {
			traceMessage: "─",
			limit:        1,
			expectedContent: "\n" +
				"\x1b[33;1mJob's log exceeded limit of 1 bytes.\n" +
				"Job execution will continue but no more output will be collected.\x1b[0;m\n",
			expectedChecksum: "crc32:9e261b5f",
		},
		"4-byte UTF-8 characters": {
			traceMessage: "🐤",
			limit:        23,
			expectedContent: "🐤🐤🐤🐤🐤\n" +
				"\x1b[33;1mJob's log exceeded limit of 23 bytes.\n" +
				"Job execution will continue but no more output will be collected.\x1b[0;m\n",
			expectedChecksum: "crc32:10e32ecd",
		},
		"4-byte UTF-8 characters on even boundary": {
			traceMessage: "🐤",
			limit:        24,
			expectedContent: "🐤🐤🐤🐤🐤🐤\n" +
				"\x1b[33;1mJob's log exceeded limit of 24 bytes.\n" +
				"Job execution will continue but no more output will be collected.\x1b[0;m\n",
			expectedChecksum: "crc32:26e43372",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			buffer, err := New()
			require.NoError(t, err)
			defer buffer.Close()

			buffer.SetLimit(tc.limit)
			assert.Equal(t, 0, buffer.Size())

			for i := 0; i < 100; i++ {
				n, err := buffer.Write([]byte(tc.traceMessage))
				require.NoError(t, err)
				require.Greater(t, n, 0)
			}

			buffer.Finish()

			content, err := buffer.Bytes(0, 1000)
			require.NoError(t, err)

			assert.Equal(t, len(tc.expectedContent), buffer.Size(), "unexpected buffer size")
			assert.Equal(t, tc.expectedChecksum, buffer.Checksum())
			assert.Equal(t, tc.expectedContent, string(content))
		})
	}
}

func TestDelayedLimit(t *testing.T) {
	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	n, err := buffer.Write([]byte("data before limit\n"))
	assert.NoError(t, err)
	assert.Greater(t, n, 0)

	buffer.SetLimit(20)

	n, err = buffer.Write([]byte("data after limit\n"))
	assert.NoError(t, err)
	assert.Greater(t, n, 0)

	buffer.Finish()

	content, err := buffer.Bytes(0, 1000)
	require.NoError(t, err)

	expectedContent := "data before limit\nda\n\x1b[33;1mJob's log exceeded limit of 20 bytes.\n" +
		"Job execution will continue but no more output will be collected.\x1b[0;m\n"
	assert.Equal(t, len(expectedContent), buffer.Size(), "unexpected buffer size")
	assert.Equal(t, "crc32:559aa46f", buffer.Checksum())
	assert.Equal(t, expectedContent, string(content))
}

func TestTraceRace(t *testing.T) {
	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	buffer.SetLimit(1000)

	load := []func(){
		func() { _, _ = buffer.Write([]byte("x")) },
		func() { buffer.SetLimit(1000) },
		func() { buffer.Checksum() },
		func() { buffer.Size() },
	}

	var wg sync.WaitGroup
	for _, fn := range load {
		wg.Add(1)
		go func(fn func()) {
			defer wg.Done()

			for i := 0; i < 100; i++ {
				fn()
			}
		}(fn)
	}

	wg.Wait()

	buffer.Finish()

	_, err = buffer.Bytes(0, 1000)
	require.NoError(t, err)
}

func TestFlushOnError(t *testing.T) {
	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	require.False(t, buffer.failedFlush)

	n, err := buffer.Write([]byte("write to buffer"))
	require.Equal(t, 15, n)
	require.NoError(t, err)

	// close underlying writer
	buffer.logFile.Close()

	// consecutive flushes should now continue to error, as a closed file cannot
	// be recovered.
	_, err = buffer.Bytes(0, 15)
	require.Error(t, err)

	n, err = buffer.Write([]byte("..."))
	require.Equal(t, 0, n)
	require.Error(t, err)

	require.True(t, buffer.failedFlush)
}

func TestFixupInvalidUTF8(t *testing.T) {
	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	// \xfe and \xff are both invalid
	// \xff will be replaced by the "unicode replacement character" \ufffd
	_, err = buffer.Write([]byte("hello a\xfeb a\xffb\n"))
	require.NoError(t, err)

	content, err := buffer.Bytes(0, 1000)
	require.NoError(t, err)

	assert.True(t, utf8.ValidString(string(content)))
	assert.Equal(t, "hello a\ufffdb a\ufffdb\n", string(content))
}

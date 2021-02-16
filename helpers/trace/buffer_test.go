package trace

import (
	"math"
	"testing"

	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVariablesMasking(t *testing.T) {
	traceMessage := "This is the secret message cont@ining :secret duplicateValues"
	maskedValues := []string{
		"is",
		"duplicateValue",
		"duplicateValue",
		":secret",
		"cont@ining",
	}

	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	buffer.SetMasked(maskedValues)

	_, err = buffer.Write([]byte(traceMessage))
	require.NoError(t, err)

	buffer.Finish()

	content, err := buffer.Bytes(0, 1000)
	require.NoError(t, err)

	assert.Equal(t, "Th[MASKED] [MASKED] the secret message [MASKED] [MASKED] [MASKED]s", string(content))
}

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

func TestDelayedMask(t *testing.T) {
	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	n, err := buffer.Write([]byte("data before mask\n"))
	assert.NoError(t, err)
	assert.Greater(t, n, 0)

	buffer.SetMasked([]string{"mask_me"})

	n, err = buffer.Write([]byte("data mask_me masked\n"))
	assert.NoError(t, err)
	assert.Greater(t, n, 0)

	buffer.Finish()

	content, err := buffer.Bytes(0, 1000)
	require.NoError(t, err)

	expectedContent := "data before mask\ndata [MASKED] masked\n"
	assert.Equal(t, len(expectedContent), buffer.Size(), "unexpected buffer size")
	assert.Equal(t, "crc32:690f62e1", buffer.Checksum())
	assert.Equal(t, expectedContent, string(content))
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

const logLineStr = "hello world, this is a lengthy log line including secrets such as 'hello', and " +
	"https://example.com/?rss_token=foo&rss_token=bar and http://example.com/?authenticity_token=deadbeef and " +
	"https://example.com/?rss_token=foobar. it's longer than most log lines, but probably a good test for " +
	"anything that's benchmarking how fast it is to write log lines."

var logLineByte = []byte(logLineStr)

func BenchmarkBuffer10k(b *testing.B) {
	for i := 0; i < b.N; i++ {
		func() {
			buffer, err := New()
			require.NoError(b, err)
			defer buffer.Close()

			buffer.SetLimit(math.MaxInt64)
			buffer.SetMasked([]string{"hello"})

			const N = 10000
			b.ReportAllocs()
			b.SetBytes(int64(len(logLineByte) * N))
			for i := 0; i < N; i++ {
				_, _ = buffer.Write(logLineByte)
			}
			buffer.Finish()
		}()
	}
}

func BenchmarkBuffer10kWithURLScrub(b *testing.B) {
	for i := 0; i < b.N; i++ {
		func() {
			buffer, err := New()
			require.NoError(b, err)
			defer buffer.Close()

			buffer.SetLimit(math.MaxInt64)
			buffer.SetMasked([]string{"hello"})

			const N = 10000
			b.ReportAllocs()
			b.SetBytes(int64(len(logLineByte) * N))
			for i := 0; i < N; i++ {
				_, _ = buffer.Write([]byte(url_helpers.ScrubSecrets(logLineStr)))
			}
			buffer.Finish()
		}()
	}
}

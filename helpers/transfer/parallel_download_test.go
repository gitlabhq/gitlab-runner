//go:build !integration

package transfer

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParallelRangeDownload_WriteAt(t *testing.T) {
	t.Parallel()

	const total = int64(100)
	payload := bytes.Repeat([]byte("x"), int(total))

	fetchChunk := func(offset, length int64) (io.ReadCloser, error) {
		end := offset + length
		if end > total {
			end = total
		}
		return io.NopCloser(bytes.NewReader(payload[offset:end])), nil
	}

	f, err := os.CreateTemp(t.TempDir(), "parallel-range")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	err = ParallelRangeDownload(total, 7, 4, f, fetchChunk)
	require.NoError(t, err)

	got, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestParallelRangeDownload_InvalidChunkSize(t *testing.T) {
	t.Parallel()

	fetchChunk := func(_, _ int64) (io.ReadCloser, error) {
		t.Fatal("fetchChunk must not be called")
		return nil, nil
	}

	f, err := os.CreateTemp(t.TempDir(), "parallel-range")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	err = ParallelRangeDownload(100, 0, 4, f, fetchChunk)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chunk size must be positive")

	err = ParallelRangeDownload(100, -1, 4, f, fetchChunk)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chunk size must be positive")
}

// Regression: fetchChunk must not be able to drive unbounded allocation if the body is larger than the range.
func TestParallelRangeDownload_OversizedChunkBodyIgnored(t *testing.T) {
	t.Parallel()

	const total = int64(20)
	want := bytes.Repeat([]byte("a"), int(total))
	extra := bytes.Repeat([]byte("b"), 500)

	fetchChunk := func(offset, length int64) (io.ReadCloser, error) {
		slice := want[offset : offset+length]
		// Server ignores range length and appends junk; only the first length bytes may be read.
		return io.NopCloser(io.MultiReader(bytes.NewReader(slice), bytes.NewReader(extra))), nil
	}

	f, err := os.CreateTemp(t.TempDir(), "parallel-range")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	err = ParallelRangeDownload(total, 7, 4, f, fetchChunk)
	require.NoError(t, err)

	got, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestParallelRangeDownload_ShortChunkBody(t *testing.T) {
	t.Parallel()

	fetchChunk := func(offset, length int64) (io.ReadCloser, error) {
		if offset == 0 {
			return io.NopCloser(bytes.NewReader([]byte("short"))), nil
		}
		return io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), int(length)))), nil
	}

	f, err := os.CreateTemp(t.TempDir(), "parallel-range")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	err = ParallelRangeDownload(100, 50, 2, f, fetchChunk)
	require.Error(t, err)
	assert.True(t, errors.Is(err, io.ErrUnexpectedEOF), "got %v", err)
}

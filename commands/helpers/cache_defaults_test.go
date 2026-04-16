//go:build !integration

package helpers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCacheTransferTuning(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateCacheTransferTuning(
		defaultCacheTransferBufferSize,
		defaultCacheChunkSize,
		defaultCacheConcurrency,
	))

	require.NoError(t, validateCacheTransferTuning(1, 0, 0))

	err := validateCacheTransferTuning(0, defaultCacheChunkSize, defaultCacheConcurrency)
	require.Error(t, err)
	require.Contains(t, err.Error(), "transfer buffer size")

	err = validateCacheTransferTuning(-1, defaultCacheChunkSize, defaultCacheConcurrency)
	require.Error(t, err)
	require.Contains(t, err.Error(), "transfer buffer size")

	err = validateCacheTransferTuning(defaultCacheTransferBufferSize, -1, defaultCacheConcurrency)
	require.Error(t, err)
	require.Contains(t, err.Error(), "chunk size")

	err = validateCacheTransferTuning(defaultCacheTransferBufferSize, defaultCacheChunkSize, -1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "concurrency")
}

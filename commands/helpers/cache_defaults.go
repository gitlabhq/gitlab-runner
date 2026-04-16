package helpers

import "fmt"

// Default sizes for cache-extractor and cache-archiver transfer tuning (overridden by CLI / env).
const (
	defaultCacheTransferBufferSize = 4 * 1024 * 1024  // 4 MiB
	defaultCacheChunkSize          = 16 * 1024 * 1024 // 16 MiB
	defaultCacheConcurrency        = 16

	// logFieldHTTPETag is the structured log key for the HTTP ETag header (snake_case). Not defined in labkit/fields yet.
	logFieldHTTPETag = "etag"
)

// validateCacheTransferTuning checks values after normalize* maps 0 to defaults.
// Negative sizes bypass normalization and must be rejected so allocation and blob options do not panic or misbehave.
func validateCacheTransferTuning(transferBufferSize, chunkSize, concurrency int) error {
	if transferBufferSize <= 0 {
		return fmt.Errorf("invalid cache transfer buffer size %d (CACHE_TRANSFER_BUFFER_SIZE / --transfer-buffer-size): must be positive; use 0 for default %d bytes",
			transferBufferSize, defaultCacheTransferBufferSize)
	}
	if chunkSize < 0 {
		return fmt.Errorf("invalid cache chunk size %d (CACHE_CHUNK_SIZE / --chunk-size): must be non-negative; use 0 for default %d bytes",
			chunkSize, defaultCacheChunkSize)
	}
	if concurrency < 0 {
		return fmt.Errorf("invalid cache concurrency %d (CACHE_CONCURRENCY / --concurrency): must be non-negative", concurrency)
	}
	return nil
}

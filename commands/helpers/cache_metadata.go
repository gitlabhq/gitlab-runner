package helpers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
)

const (
	// cacheMetadataFileName is the basename of the local metadata file, to be dropped alongside the actual archive
	cacheMetadataFileName = "metadata.json"
)

// writeCacheMetadataFile dumps a file alongside the archive, holding all metadata. Before writing, the metadata keys
// are normalized with [normalizeMetadataKey].
func writeCacheMetadataFile(archiveFilePath string, metadata map[string]string) error {
	normalized := map[string]string{}
	for k, v := range metadata {
		if k == "" {
			continue
		}
		normalized[normalizeCacheMetadataKey(k)] = v
	}

	// json.Marshal won't ever fail for map[string]string
	data, _ := json.Marshal(normalized)

	file := filepath.Join(filepath.Dir(archiveFilePath), cacheMetadataFileName)
	if err := os.WriteFile(file, data, 0640); err != nil {
		return fmt.Errorf("writing metadata file: %w", err)
	}

	return nil
}

// normalizeCacheMetadataKey normalizes a metadata key. This is done to be consistent, regardless where the metadata
// actually came from (e.g.: user defined for uploads, from http headers for downloads) or which cloud providers are at
// play.
func normalizeCacheMetadataKey(key string) string {
	return strings.ToLower(textproto.CanonicalMIMEHeaderKey(key))
}

// headersToCacheMetadata pulls out metadata from well-known http response headers.
func headersToCacheMetadata(headers http.Header) map[string]string {
	metadata := map[string]string{}

	for headerKey := range headers {
		metaKey, ok := extractCacheMetadataKey(headerKey)
		if !ok {
			continue
		}
		metadata[metaKey] = headers.Get(headerKey)
	}

	return metadata
}

// extractCacheMetadataKey checks if headerKey looks like a http response header key for metadata, ie. something like
// the headers below. If so, the actual metadata key is returned.
// This is best-effort, at the time we pull caches with a pre-signed URL, we don't have any other information, and we
// don't have creds to actually ask the cloud provider for metadata.
//
// http headers for metadata look something like:
//   - X-Goog-Meta-something...
//   - X-Amz-Meta-something...
func extractCacheMetadataKey(headerKey string) (string, bool) {
	parts := strings.Split(headerKey, "-")
	if len(parts) < 4 {
		return "", false
	}
	isMetadataHeader := (strings.EqualFold(parts[0], "x") && strings.EqualFold(parts[2], "meta"))
	if isMetadataHeader {
		return strings.Join(parts[3:], "-"), true
	}
	return "", false
}

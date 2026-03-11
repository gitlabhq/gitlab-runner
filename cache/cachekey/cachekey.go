package cachekey

import (
	"fmt"
	"path"
	"strings"
	"unicode"
)

// normaliser decodes URL-encoded slashes and dots, and converts backslashes to
// forward slashes in a single pass.
var normaliser = strings.NewReplacer(
	"%2f", "/",
	"%2F", "/",
	"%2e", ".",
	"%2E", ".",
	`\`, "/",
)

// Sanitize validates and normalises a cache key.
// Cache keys may contain path separators. The function:
//   - decodes URL-encoded '/' (%2f) and '.' (%2e) characters
//   - replaces all '\' with '/'
//   - resolves path traversals (., ..) within a virtual root
//   - strips trailing whitespace from the rightmost path segments,
//     removing any that become empty after trimming
func Sanitize(cacheKey string) (string, error) {
	if cacheKey == "" {
		return "", nil
	}

	// Decode percent-encoded chars and normalise separators, then
	// resolve traversals against a virtual root so ".." can never
	// escape beyond the root.
	cleaned := path.Clean("/" + normaliser.Replace(cacheKey))

	// Strip the leading "/" we added, split into segments, then walk
	// backwards trimming trailing whitespace from the rightmost
	// segments—dropping any that become empty.
	parts := strings.Split(cleaned[1:], "/")
	n := len(parts)
	for n > 0 {
		parts[n-1] = strings.TrimRightFunc(parts[n-1], unicode.IsSpace)
		if parts[n-1] != "" {
			break
		}
		n--
	}

	key := strings.Join(parts[:n], "/")

	if key == "" {
		return "", fmt.Errorf("cache key %q could not be sanitized", cacheKey)
	}

	return key, nil
}

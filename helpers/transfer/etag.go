package transfer

import "strings"

// StrongETag reports whether etag is a strong entity tag (RFC 9110, section 8.8.3): a quoted
// opaque tag without the W/ prefix of a weak one. If-Match compares entity tags strongly, so only
// a strong one can make a chunk request conditional on the version of the object that an earlier
// response named. A weak one never matches, and would fail every chunk request with 412.
func StrongETag(etag string) bool {
	return len(etag) >= 2 && etag[0] == '"' && etag[len(etag)-1] == '"' &&
		!strings.Contains(etag[1:len(etag)-1], `"`)
}

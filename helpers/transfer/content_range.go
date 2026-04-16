package transfer

import (
	"strconv"
	"strings"
)

// RangeProbeBodyMaxDiscard is how many bytes to read from the body of a Range GET (e.g. bytes=0-0) before Close.
// A 206 response body is 1 byte; reading slightly more helps HTTP connection reuse.
const RangeProbeBodyMaxDiscard int64 = 2

// ParseContentRangeTotal returns the full representation length N from an HTTP Content-Range field value
// (RFC 9110), for example "bytes 0-0/N" or "bytes */N". It returns ok false if the value is malformed,
// the complete length is unknown ("*"), or N <= 0.
func ParseContentRangeTotal(contentRange string) (n int64, ok bool) {
	const prefix = "bytes "
	contentRange = strings.TrimSpace(contentRange)
	if !strings.HasPrefix(contentRange, prefix) {
		return 0, false
	}
	rest := strings.TrimSpace(contentRange[len(prefix):])
	slash := strings.LastIndex(rest, "/")
	if slash < 0 {
		return 0, false
	}
	totalStr := strings.TrimSpace(rest[slash+1:])
	if totalStr == "*" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(totalStr, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

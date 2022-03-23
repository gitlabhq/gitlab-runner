package helpers

import (
	"strings"
)

func ShortenToken(token string) string {
	if len(token) < 8 {
		return token
	}

	if token[:2] == "GR" && len(token) >= 17 {
		// Token is prefixed with RUNNERS_TOKEN_PREFIX: GR (for Gitlab Runner) combined with the rotation
		// date decimal-to-hex-encoded. Let's add some more characters in order to compensate.
		if strings.IndexFunc(token[2:9], isInvalidPrefixRune) == -1 {
			return token[:17]
		}
	}
	return token[:8]
}

func isInvalidPrefixRune(r rune) bool {
	return (r < '0' || r > '9') && (r < 'A' || r > 'F') && (r < 'a' || r > 'f')
}

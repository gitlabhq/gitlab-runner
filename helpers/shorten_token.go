package helpers

import (
	"strings"
)

func ShortenToken(token string) string {
	switch {
	case len(token) < 8:
		return token
	case token[:5] == "glrt-":
		// Token is prefixed with CREATED_RUNNER_TOKEN_PREFIX: glrt- (for GitLab Runner Token).
		// Let's add some more characters in order to compensate.
		return token[5:14]
	case token[:6] == "glrtr-":
		// Token is prefixed with REGISTRATION_RUNNER_TOKEN_PREFIX: glrtr- (for GitLab Runner Token).
		// This prefix will only be used for runners registered via the registration token.
		// Let's add some more characters in order to compensate.
		return token[6:15]
	case token[:6] == "glcbt-":
		return token[6:15]
	case token[:2] == "GR" && len(token) >= 17 && strings.IndexFunc(token[2:9], isInvalidPrefixRune) == -1:
		// Token is prefixed with RUNNERS_TOKEN_PREFIX: GR (for GitLab Runner) combined with the rotation
		// date decimal-to-hex-encoded. Let's add some more characters in order to compensate.
		return token[:17]
	default:
		return token[:8]
	}
}

func isInvalidPrefixRune(r rune) bool {
	return (r < '0' || r > '9') && (r < 'A' || r > 'F') && (r < 'a' || r > 'f')
}

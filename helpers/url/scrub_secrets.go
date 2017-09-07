package url_helpers

import (
	"regexp"
)

var scrubRegexp = regexp.MustCompile(`(?im)([\?&]((?:private|authenticity|rss)[\-_]token)|X-AMZ-Signature)=[^&]*`)

// ScrubSecrets replaces the content of any sensitive query string parameters
// in an URL with `[FILTERED]`
func ScrubSecrets(url string) string {
	return scrubRegexp.ReplaceAllString(url, "$1=[FILTERED]")
}

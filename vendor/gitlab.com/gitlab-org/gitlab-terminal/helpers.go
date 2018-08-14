package terminal

import (
	"net"
	"net/http"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

var scrubRegexp = regexp.MustCompile(`(?i)([\?&]((?:private|authenticity|rss)[\-_]token)|(?:X-AMZ-)?Signature)=[^&]*`)

func fail500(w http.ResponseWriter, r *http.Request, err error) {
	http.Error(w, "Internal server error", 500)
	printError(r, err)
}

func printError(r *http.Request, err error) {
	if r != nil {
		log.WithFields(log.Fields{
			"method": r.Method,
			"uri":    scrubURLParams(r.RequestURI),
		}).WithError(err).Error("error")
	} else {
		log.WithError(err).Error("unknown error")
	}
}

func headerClone(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}

func setForwardedFor(newHeaders *http.Header, originalRequest *http.Request) {
	if clientIP, _, err := net.SplitHostPort(originalRequest.RemoteAddr); err == nil {
		var header string

		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := originalRequest.Header["X-Forwarded-For"]; ok {
			header = strings.Join(prior, ", ") + ", " + clientIP
		} else {
			header = clientIP
		}
		newHeaders.Set("X-Forwarded-For", header)
	}
}

// ScrubURLParams replaces the content of any sensitive query string parameters
// in an URL with `[FILTERED]`
func scrubURLParams(url string) string {
	return scrubRegexp.ReplaceAllString(url, "$1=[FILTERED]")
}

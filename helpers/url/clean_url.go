package url_helpers

import "net/url"

func CleanURL(value string) (ret string) {
	u, err := url.Parse(value)
	if err != nil {
		return
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// RedactIfEnabled returns url unchanged when redact is false. When redact is
// true it returns the placeholder string "remote storage" so callers can use
// it unconditionally in log statements without branching.
func RedactIfEnabled(url string, redact bool) string {
	if redact {
		return "remote storage"
	}
	return url
}

// OnlySchemeAndHost strips everything from an URL, except the host (including port) and the scheme; in other words, it
// removes path, fragment, query & userinfo.
// The original URL won't be mutated.
func OnlySchemeAndHost(u *url.URL) *url.URL {
	return &url.URL{
		Host:   u.Host,
		Scheme: u.Scheme,
	}
}

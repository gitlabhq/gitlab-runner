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

// OnlySchemeAndHost strips everything from an URL, except the host (including port) and the scheme; in other words, it
// removes path, fragment, query & userinfo.
// The original URL won't be mutated.
func OnlySchemeAndHost(u *url.URL) *url.URL {
	return &url.URL{
		Host:   u.Host,
		Scheme: u.Scheme,
	}
}

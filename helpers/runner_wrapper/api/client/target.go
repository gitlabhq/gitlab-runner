package client

import (
	"net/url"
	"strings"
)

func parseDialTarget(target string) (string, string) {
	network := "tcp"

	// unix://absolute
	if strings.Contains(target, ":/") {
		uri, err := url.Parse(target)
		if err != nil {
			return network, target
		}

		if uri.Path == "" {
			return uri.Scheme, uri.Host
		}

		return uri.Scheme, uri.Path
	}

	// unix:relative-path
	network, path, found := strings.Cut(target, ":")
	if found {
		return network, path
	}

	// tcp://target
	return network, target
}

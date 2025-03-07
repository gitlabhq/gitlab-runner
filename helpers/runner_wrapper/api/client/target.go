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
	scheme, addr, found := strings.Cut(target, ":")
	if found && scheme == "unix" {
		return scheme, addr
	}

	// tcp://target
	return network, target
}

func formatGRPCCompatible(target string) string {
	network, address := parseDialTarget(target)
	if network == "unix" {
		return target
	}

	return address
}

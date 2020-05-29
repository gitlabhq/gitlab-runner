package docker

import (
	"net"
	"net/http"
	"time"

	"github.com/docker/go-connections/sockets"
)

const defaultTimeout = 300 * time.Second
const defaultKeepAlive = 10 * time.Second
const defaultTLSHandshakeTimeout = 60 * time.Second
const defaultResponseHeaderTimeout = 120 * time.Second
const defaultExpectContinueTimeout = 120 * time.Second
const defaultIdleConnTimeout = 10 * time.Second

// configureTransport configures the specified Transport according to the
// specified proto and addr.
// If the proto is unix (using a unix socket to communicate) or npipe the
// compression is disabled.
func configureTransport(tr *http.Transport, proto, addr string) error {
	err := sockets.ConfigureTransport(tr, proto, addr)
	if err != nil {
		return err
	}

	tr.TLSHandshakeTimeout = defaultTLSHandshakeTimeout
	tr.ResponseHeaderTimeout = defaultResponseHeaderTimeout
	tr.ExpectContinueTimeout = defaultExpectContinueTimeout
	tr.IdleConnTimeout = defaultIdleConnTimeout

	// for network protocols set custom sockets with keep-alive
	if proto == "tcp" || proto == "http" || proto == "https" {
		dialer, err := sockets.DialerFromEnvironment(&net.Dialer{
			Timeout:   defaultTimeout,
			KeepAlive: defaultKeepAlive,
		})
		if err != nil {
			return err
		}
		tr.Dial = dialer.Dial // nolint:staticcheck
	}
	return nil
}

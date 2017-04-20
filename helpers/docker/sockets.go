package docker_helpers

import (
	"net"
	"net/http"
	"time"

	"github.com/docker/go-connections/sockets"
)

// Why 32? See https://github.com/docker/docker/pull/8035.
const defaultTimeout = 32 * time.Second
const defaultKeepAlive = 10 * time.Second
const defaultTLSHandshakeTimeout = 10 * time.Second
const defaultResponseHeaderTimeout = 30 * time.Second
const defaultExpectContinueTimeout = 30 * time.Second
const defaultIdleConnTimeout = time.Minute

// configureTransport configures the specified Transport according to the
// specified proto and addr.
// If the proto is unix (using a unix socket to communicate) or npipe the
// compression is disabled.
func configureTransport(tr *http.Transport, proto, addr string) error {
	tr.TLSHandshakeTimeout = defaultTLSHandshakeTimeout
	tr.ResponseHeaderTimeout = defaultResponseHeaderTimeout
	tr.ExpectContinueTimeout = defaultExpectContinueTimeout
	tr.IdleConnTimeout = defaultIdleConnTimeout

	switch proto {
	case "unix":
		// No need for compression in local communications.
		tr.DisableCompression = true
		tr.Dial = func(_, _ string) (net.Conn, error) {
			return net.DialTimeout(proto, addr, defaultTimeout)
		}
	case "npipe":
		// No need for compression in local communications.
		tr.DisableCompression = true
		tr.Dial = func(_, _ string) (net.Conn, error) {
			return sockets.DialPipe(addr, defaultTimeout)
		}
	default:
		tr.Proxy = http.ProxyFromEnvironment
		dialer, err := sockets.DialerFromEnvironment(&net.Dialer{
			Timeout:   defaultTimeout,
			KeepAlive: defaultKeepAlive,
		})
		if err != nil {
			return err
		}
		tr.Dial = dialer.Dial
	}
	return nil
}

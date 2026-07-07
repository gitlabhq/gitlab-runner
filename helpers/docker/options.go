package docker

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/moby/moby/client"
)

const (
	defaultTimeout               = 300 * time.Second
	defaultKeepAlive             = 10 * time.Second
	defaultTLSHandshakeTimeout   = 60 * time.Second
	defaultResponseHeaderTimeout = 120 * time.Second
	defaultExpectContinueTimeout = 120 * time.Second
	defaultIdleConnTimeout       = 10 * time.Second
)

var (
	useTestDialerFunc = false
	errDialerTest     = errors.New("custom dialer error")
	testDialerFunc    = func(context.Context, string, string) (net.Conn, error) {
		return nil, errDialerTest
	}
)

// newCustomHTTPClient builds the *http.Client the SDK client uses from an
// already-configured transport (see configureTransport) and disallows
// redirects to prevent redirection to malicious docker daemons.
//
// The transport is configured up-front and handed to the SDK as a fully-formed
// client, so the cached transport remains authoritative for tests that inspect
// it (client.WithHTTPClient clones the transport it is given).
func newCustomHTTPClient(transport *http.Transport) *http.Client {
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return ErrRedirectNotAllowed
		},
	}
}

// configureTransport builds the transport for the docker client's
// *http.Client (see newCustomHTTPClient). client.Opt operates on an
// unexported config, so callers can't reach these settings through it;
// we set sockets, timeouts, the dialer, and TLS on our own transport instead.
func configureTransport(transport *http.Transport, c Credentials) error {
	url, err := client.ParseHostURL(c.Host)
	if err != nil {
		return err
	}

	if err := sockets.ConfigureTransport(transport, url.Scheme, url.Host); err != nil {
		return err
	}

	switch url.Scheme {
	case "tcp", "http", "https":
		// only set timeouts and a dialer for remote schemes; for unix/npipe
		// sockets.ConfigureTransport already installs the appropriate dialer.
		transport.TLSHandshakeTimeout = defaultTLSHandshakeTimeout
		transport.ResponseHeaderTimeout = defaultResponseHeaderTimeout
		transport.ExpectContinueTimeout = defaultExpectContinueTimeout
		transport.IdleConnTimeout = defaultIdleConnTimeout

		dialer := &net.Dialer{
			Timeout:   defaultTimeout,
			KeepAlive: defaultKeepAlive,
		}

		if !useTestDialerFunc {
			transport.DialContext = dialer.DialContext
		} else {
			// set the test dialer function, so we can test that
			// our client setup works in the expected order
			transport.DialContext = testDialerFunc
		}
	}

	if c.TLSVerify {
		var cacertPath, certPath, keyPath string
		if c.CertPath != "" {
			cacertPath = filepath.Join(c.CertPath, "ca.pem")
			certPath = filepath.Join(c.CertPath, "cert.pem")
			keyPath = filepath.Join(c.CertPath, "key.pem")
		}

		tlsConfig, err := tlsconfig.Client(tlsconfig.Options{
			CAFile:             cacertPath,
			CertFile:           certPath,
			KeyFile:            keyPath,
			ExclusiveRootPools: true,
			MinVersion:         tls.VersionTLS12,
		})
		if err != nil {
			return err
		}

		transport.TLSClientConfig = tlsConfig
	}

	return nil
}

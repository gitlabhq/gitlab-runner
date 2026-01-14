package docker

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
	"github.com/docker/go-connections/sockets"
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

func WithCustomTLSClientConfig(c Credentials) client.Opt {
	return func(cli *client.Client) error {
		var cacertPath, certPath, keyPath string
		if c.CertPath != "" {
			cacertPath = filepath.Join(c.CertPath, "ca.pem")
			certPath = filepath.Join(c.CertPath, "cert.pem")
			keyPath = filepath.Join(c.CertPath, "key.pem")
		}

		if c.TLSVerify {
			return client.WithTLSClientConfig(
				cacertPath,
				certPath,
				keyPath,
			)(cli)
		}

		return nil
	}
}

func WithCustomHTTPClient(transport *http.Transport) client.Opt {
	return func(c *client.Client) error {
		url, err := client.ParseHostURL(c.DaemonHost())
		if err != nil {
			return err
		}

		err = sockets.ConfigureTransport(transport, url.Scheme, url.Host)
		if err != nil {
			return err
		}

		// customize http client
		if err := client.WithHTTPClient(&http.Client{
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return ErrRedirectNotAllowed
			},
		})(c); err != nil {
			return err
		}

		switch url.Scheme {
		case "tcp", "http", "https":
			// only set timeouts for remote schemes
			transport.TLSHandshakeTimeout = defaultTLSHandshakeTimeout
			transport.ResponseHeaderTimeout = defaultResponseHeaderTimeout
			transport.ExpectContinueTimeout = defaultExpectContinueTimeout
			transport.IdleConnTimeout = defaultIdleConnTimeout
		default:
			return nil
		}

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

		return nil
	}
}

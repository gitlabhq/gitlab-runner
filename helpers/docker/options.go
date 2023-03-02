package docker

import (
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
	testDialerFunc    = func(string, string) (net.Conn, error) {
		return nil, errDialerTest
	}
)

func WithCustomHTTPClient() client.Opt {
	return client.WithHTTPClient(&http.Client{
		Transport: &http.Transport{
			TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
			ResponseHeaderTimeout: defaultResponseHeaderTimeout,
			ExpectContinueTimeout: defaultExpectContinueTimeout,
			IdleConnTimeout:       defaultIdleConnTimeout,
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return ErrRedirectNotAllowed
		},
	})
}

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

func WithCustomKeepalive() client.Opt {
	return func(c *client.Client) error {
		url, err := client.ParseHostURL(c.DaemonHost())
		if err != nil {
			return err
		}

		switch url.Scheme {
		case "tcp", "http", "https":
		default:
			return nil
		}

		transport, ok := c.HTTPClient().Transport.(*http.Transport)
		if !ok {
			return nil
		}

		dialer, err := sockets.DialerFromEnvironment(&net.Dialer{
			Timeout:   defaultTimeout,
			KeepAlive: defaultKeepAlive,
		})
		if err != nil {
			return err
		}

		// copy same behaviour as docker's client, and use Dial rather
		// than DialContext
		//nolint:staticcheck
		if !useTestDialerFunc {
			transport.Dial = dialer.Dial
		} else {
			// set the test dialer function, so we can test that
			// our client setup works in the expected order
			transport.Dial = testDialerFunc
		}

		return nil
	}
}

package terminal

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

type TerminalSettings struct {
	// The terminal provider may require use of a particular subprotocol. If so,
	// it must be specified here, and Workhorse must have a matching codec.
	Subprotocols []string

	// The websocket URL to connect to.
	Url string

	// Any headers (e.g., Authorization) to send with the websocket request
	Header http.Header

	// The CA roots to validate the remote endpoint with, for wss:// URLs. The
	// system-provided CA pool will be used if this is blank. PEM-encoded data.
	CAPem string

	// The value is specified in seconds. It is converted to time.Duration
	// later.
	MaxSessionTime int
}

func (t *TerminalSettings) URL() (*url.URL, error) {
	return url.Parse(t.Url)
}

func (t *TerminalSettings) Dialer() *websocket.Dialer {
	dialer := &websocket.Dialer{
		Subprotocols: t.Subprotocols,
	}

	if len(t.CAPem) > 0 {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM([]byte(t.CAPem))
		dialer.TLSClientConfig = &tls.Config{RootCAs: pool}
	}

	return dialer
}

func (t *TerminalSettings) Clone() *TerminalSettings {
	// Doesn't clone the strings, but that's OK as strings are immutable in go
	cloned := *t
	cloned.Header = headerClone(t.Header)
	return &cloned
}

func (t *TerminalSettings) Dial() (*websocket.Conn, *http.Response, error) {
	return t.Dialer().Dial(t.Url, t.Header)
}

func (t *TerminalSettings) Validate() error {
	if t == nil {
		return fmt.Errorf("Terminal details not specified")
	}

	if len(t.Subprotocols) == 0 {
		return fmt.Errorf("No subprotocol specified")
	}

	parsedURL, err := t.URL()
	if err != nil {
		return fmt.Errorf("Invalid URL")
	}

	if parsedURL.Scheme != "ws" && parsedURL.Scheme != "wss" {
		return fmt.Errorf("Invalid websocket scheme: %q", parsedURL.Scheme)
	}

	return nil
}

func (t *TerminalSettings) IsEqual(other *TerminalSettings) bool {
	if t == nil && other == nil {
		return true
	}

	if t == nil || other == nil {
		return false
	}

	if len(t.Subprotocols) != len(other.Subprotocols) {
		return false
	}

	for i, subprotocol := range t.Subprotocols {
		if other.Subprotocols[i] != subprotocol {
			return false
		}
	}

	if len(t.Header) != len(other.Header) {
		return false
	}

	for header, values := range t.Header {
		if len(values) != len(other.Header[header]) {
			return false
		}
		for i, value := range values {
			if other.Header[header][i] != value {
				return false
			}
		}
	}

	return t.Url == other.Url &&
		t.CAPem == other.CAPem &&
		t.MaxSessionTime == other.MaxSessionTime
}

package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

const requestTimeout = 30 * time.Second

// maxErrorBodyLen caps how much of a response body APIError carries, since
// its Error() text ends up in the job log and appliance_url is job-controlled.
const maxErrorBodyLen = 256

// APIError wraps a non-2xx response from the Conjur appliance. StatusCode is
// exposed so callers can classify configuration failures (4xx) separately
// from external dependency failures (5xx).
type APIError struct {
	Status int
	Body   string
}

func newAPIError(status int, body string) *APIError {
	if len(body) > maxErrorBodyLen {
		body = strings.ToValidUTF8(body[:maxErrorBodyLen], "") + "...(truncated)"
	}

	return &APIError{Status: status, Body: body}
}

func (e *APIError) Error() string {
	return fmt.Sprintf("conjur API error: status %d: %s", e.Status, e.Body)
}

func (e *APIError) StatusCode() int {
	return e.Status
}

type Client struct {
	httpClient *http.Client
}

// ClientOption configures a Client built by NewClient.
type ClientOption func(*Client)

// WithRootCAs trusts only the certificate authorities in pool, in place of the
// system trust store. It is for tests against an httptest.NewTLSServer
// certificate.
func WithRootCAs(pool *x509.CertPool) ClientOption {
	return func(c *Client) {
		t := &http.Transport{}
		if d, ok := http.DefaultTransport.(*http.Transport); ok {
			t = d.Clone()
		}
		t.TLSClientConfig = &tls.Config{RootCAs: pool}
		c.httpClient.Transport = t
	}
}

// NewClient returns a Client that verifies the appliance's TLS certificate
// against the system trust store, like any other resolver's HTTP client. To
// test against an appliance signed by an internal CA, add that CA to the
// trust store of the machine running the runner, or set SSL_CERT_FILE /
// SSL_CERT_DIR, which Go's default transport reads on its own.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) GetSecret(ctx context.Context, secret *spec.ConjurSecret) (string, error) {
	baseURL, err := applianceBaseURL(secret.Server.ApplianceURL)
	if err != nil {
		return "", fmt.Errorf("invalid conjur appliance url: %w", err)
	}

	token, err := c.authenticate(ctx, baseURL, &secret.Server)
	if err != nil {
		return "", fmt.Errorf("authenticating with conjur: %w", err)
	}

	value, err := c.fetchSecret(ctx, baseURL, &secret.Server, secret.Path, token)
	if err != nil {
		return "", fmt.Errorf("fetching secret from conjur: %w", err)
	}

	return value, nil
}

// applianceBaseURL validates appliance_url and returns it as a base for the API
// paths below. A query or fragment has to be rejected rather than escaped: Go
// drops a fragment on the wire and a query turns the appended path into a
// parameter value, so either one lets a job decide which path the runner
// manager actually requests. https is required unconditionally, since the CI
// JWT and the secret value both cross this connection, and user info is
// rejected since Go sends it as an automatic Authorization: Basic header.
func applianceBaseURL(raw string) (string, error) {
	// Checked against the raw string, because url.Parse records a bare "#" or
	// "?" the same way as none at all while both still change which path the
	// appended API path lands on.
	switch {
	case strings.Contains(raw, "#"):
		return "", errors.New("must not contain a fragment")
	case strings.Contains(raw, "?"):
		return "", errors.New("must not contain a query string")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	switch {
	case u.User != nil:
		return "", errors.New("must not contain user info")
	case u.Host == "":
		return "", errors.New("no host")
	case u.Scheme != "https":
		return "", fmt.Errorf("scheme %q is not supported, use https", u.Scheme)
	}

	return strings.TrimRight(u.String(), "/"), nil
}

// authenticate exchanges the CI JWT for a short-lived Conjur access token via
// authn-jwt. The response body is the token as plain, unencoded JSON
// (protected/payload/signature); it must be base64-encoded by the caller
// before use in the Authorization header of the following request, since
// Conjur base64-decodes that header value server-side and expects to find
// the raw JSON token underneath.
func (c *Client) authenticate(ctx context.Context, baseURL string, server *spec.ConjurServer) (string, error) {
	authURL := fmt.Sprintf(
		"%s/authn-jwt/%s/%s/authenticate",
		baseURL,
		url.PathEscape(server.AuthnJWTServiceID),
		url.PathEscape(server.Account),
	)

	body := url.Values{"jwt": {server.JWT}}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("building authenticate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	respBody, statusCode, err := c.do(req)
	if err != nil {
		return "", err
	}

	if statusCode != http.StatusOK {
		return "", newAPIError(statusCode, respBody)
	}

	return encodeAccessToken(respBody)
}

var errNotAnAccessToken = errors.New("response is not a conjur access token")

// encodeAccessToken checks that body is shaped like a Conjur access token
// before encoding it for the Authorization header. Any host answering 200 would
// otherwise pass as an appliance, and since fetchSecret hands back a 200 body
// verbatim, a job could point appliance_url at a service only the runner
// manager can reach and receive that service's response as the secret value.
// Only the structure is checked, not the claims, which is enough to rule out
// responses that were never meant to be Conjur tokens.
func encodeAccessToken(body string) (string, error) {
	var token struct {
		Protected string `json:"protected"`
		Payload   string `json:"payload"`
		Signature string `json:"signature"`
	}

	if err := json.Unmarshal([]byte(body), &token); err != nil {
		return "", errNotAnAccessToken
	}

	if token.Protected == "" || token.Payload == "" || token.Signature == "" {
		return "", errNotAnAccessToken
	}

	return base64.StdEncoding.EncodeToString([]byte(body)), nil
}

func (c *Client) fetchSecret(ctx context.Context, baseURL string, server *spec.ConjurServer, path, token string) (string, error) {
	fetchURL := fmt.Sprintf(
		"%s/secrets/%s/variable/%s",
		baseURL,
		url.PathEscape(server.Account),
		encodeVariableID(path),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL, nil)
	if err != nil {
		return "", fmt.Errorf("building fetch request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf(`Token token="%s"`, token))

	respBody, statusCode, err := c.do(req)
	if err != nil {
		return "", err
	}

	switch statusCode {
	case http.StatusOK:
		if respBody == "" {
			return "", errors.New("conjur returned an empty value for the variable")
		}
		return respBody, nil
	case http.StatusNotFound:
		return "", common.ErrSecretNotFound
	default:
		return "", newAPIError(statusCode, respBody)
	}
}

// maxResponseBodySize caps how much of a response body do() buffers, since
// appliance_url is job-controlled and an attacker-pointed host could otherwise
// exhaust the runner manager's memory with an arbitrarily large response.
const maxResponseBodySize = 1 << 20 // 1 MiB; ample for an access token or secret value

func (c *Client) do(req *http.Request) (string, int, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize+1))
	if err != nil {
		return "", 0, fmt.Errorf("reading response body: %w", err)
	}
	if len(data) > maxResponseBodySize {
		return "", 0, fmt.Errorf("response body exceeds %d bytes", maxResponseBodySize)
	}

	return string(data), resp.StatusCode, nil
}

// encodeVariableID percent-encodes a Conjur variable identifier the same way
// the official Conjur clients do: slashes (which separate policy segments in
// the identifier) must become %2F rather than being left as path separators,
// and spaces must become %20 rather than the "+" that url.QueryEscape uses.
func encodeVariableID(id string) string {
	return strings.ReplaceAll(url.QueryEscape(id), "+", "%20")
}

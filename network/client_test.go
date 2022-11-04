//go:build !integration

package network

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/jpillora/backoff"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	. "gitlab.com/gitlab-org/gitlab-runner/common"
)

func clientHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	logrus.Debugln(
		r.Method, r.URL.String(),
		"Content-Type:", r.Header.Get("Content-Type"),
		"Accept:", r.Header.Get("Accept"),
		"Body:", string(body),
	)

	switch r.URL.Path {
	case "/api/v4/test/ok":
	case "/api/v4/test/auth":
		w.WriteHeader(http.StatusForbidden)
	case "/api/v4/test/json":
		if r.Header.Get("Content-Type") != "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"message":{"some-key":["some error"]}}`)
			return
		}
		if r.Header.Get("Accept") != "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotAcceptable)
			fmt.Fprint(w, `{"message":"406 Not Acceptable"}`)
			return
		}

		switch r.Header.Get("PRIVATE-TOKEN") {
		case "":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"key":"value"}`)
		case "my-pat":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"key":"value","pat":"my-pat"}`)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"message":"403 Forbidden"}`)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func writeTLSCertificate(s *httptest.Server, file string) error {
	c := s.TLS.Certificates[0]
	if len(c.Certificate) == 0 || c.Certificate[0] == nil {
		return errors.New("no predefined certificate")
	}

	encoded := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: c.Certificate[0],
	})

	return os.WriteFile(file, encoded, 0o600)
}

func writeTLSKeyPair(s *httptest.Server, certFile, keyFile string) error {
	c := s.TLS.Certificates[0]
	if len(c.Certificate) == 0 || c.Certificate[0] == nil {
		return errors.New("no predefined certificate")
	}

	encodedCert := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: c.Certificate[0],
	})

	if err := os.WriteFile(certFile, encodedCert, 0o600); err != nil {
		return err
	}

	switch k := c.PrivateKey.(type) {
	case *rsa.PrivateKey:
		encodedKey := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(k),
		})
		return os.WriteFile(keyFile, encodedKey, 0o600)
	default:
		return errors.New("unexpected private key type")
	}
}

func TestNewClient(t *testing.T) {
	c, err := newClient(&RunnerCredentials{
		URL: "http://test.example.com/ci///",
	})
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, "http://test.example.com/api/v4/", c.url.String())
}

func TestInvalidUrl(t *testing.T) {
	_, err := newClient(&RunnerCredentials{
		URL: "address.com/ci///",
	})
	assert.Error(t, err)
}

func TestClientDo(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(clientHandler))
	defer s.Close()

	c, err := newClient(&RunnerCredentials{
		URL: s.URL,
	})
	assert.NoError(t, err)
	assert.NotNil(t, c)

	statusCode, statusText, _ := c.doJSONWithPAT(
		context.Background(),
		"test/auth",
		http.MethodGet,
		http.StatusOK,
		"",
		nil,
		nil,
	)
	assert.Equal(t, http.StatusForbidden, statusCode, statusText)

	req := struct {
		Query bool `json:"query"`
	}{
		true,
	}

	res := struct {
		Key string  `json:"key"`
		PAT *string `json:"pat,omitempty"`
	}{}

	statusCode, statusText, _ = c.doJSONWithPAT(
		context.Background(),
		"test/json",
		http.MethodGet,
		http.StatusOK,
		"",
		nil,
		&res,
	)
	assert.Equal(t, http.StatusBadRequest, statusCode, statusText)
	assert.Contains(t, statusText, `test/json: 400 Bad Request (some-key: some error)`)

	statusCode, statusText, _ = c.doJSONWithPAT(
		context.Background(),
		"test/json",
		http.MethodGet,
		http.StatusOK,
		"",
		&req,
		nil,
	)
	assert.Equal(t, http.StatusNotAcceptable, statusCode, statusText)
	assert.True(
		t,
		strings.HasSuffix(statusText, "test/json: 406 Not Acceptable"),
		"%q should contain %q suffix",
		statusText,
		"test/json: 406 Not Acceptable",
	)

	statusCode, statusText, _ = c.doJSONWithPAT(
		context.Background(),
		"test/json",
		http.MethodGet,
		http.StatusOK,
		"",
		nil,
		nil,
	)
	assert.Equal(t, http.StatusBadRequest, statusCode, statusText)
	assert.Contains(t, statusText, `test/json: 400 Bad Request (some-key: some error)`)

	statusCode, statusText, _ = c.doJSONWithPAT(
		context.Background(),
		"test/json",
		http.MethodGet,
		http.StatusOK,
		"",
		&req,
		&res,
	)
	assert.Equal(t, http.StatusOK, statusCode, statusText)
	assert.Equal(t, "value", res.Key, statusText)
	assert.Equal(t, (*string)(nil), res.PAT, statusText)

	statusCode, statusText, _ = c.doJSONWithPAT(
		context.Background(),
		"test/json",
		http.MethodGet,
		http.StatusCreated,
		"my-pat",
		&req,
		&res,
	)
	assert.Equal(t, http.StatusCreated, statusCode, statusText)
	assert.Equal(t, "value", res.Key, statusText)
	assert.Equal(t, "my-pat", *res.PAT, statusText)

	statusCode, statusText, _ = c.doJSONWithPAT(
		context.Background(),
		"test/json",
		http.MethodGet,
		http.StatusCreated,
		"invalid-pat",
		&req,
		&res,
	)
	assert.Equal(t, http.StatusForbidden, statusCode, statusText)
	assert.Contains(t, statusText, `test/json: 403 Forbidden`)
}

type testContextKey int

func TestClientDo_Context(t *testing.T) {
	buf := bytes.NewBufferString("test")
	ctx := context.WithValue(context.Background(), testContextKey(0), "test")
	response := &http.Response{
		Status:     "Not found",
		StatusCode: http.StatusNotFound,
	}

	c, err := newClient(&RunnerCredentials{
		URL: "http://gitlab.example.com",
	})
	require.NoError(t, err)
	require.NotNil(t, c)

	requesterMock := new(mockRequester)
	defer requesterMock.AssertExpectations(t)
	c.requester = requesterMock

	requesterMock.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return assert.Equal(t, ctx, req.Context())
	})).Return(response, nil).Once()

	res, err := c.do(ctx, "/test", http.MethodPost, buf, "plain/text", nil)

	assert.NoError(t, err)
	assert.Equal(t, response, res)
}

func TestClientInvalidSSL(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(clientHandler))
	defer s.Close()

	c, _ := newClient(&RunnerCredentials{
		URL: s.URL,
	})
	statusCode, statusText, _ := c.doJSONWithPAT(
		context.Background(),
		"test/ok",
		http.MethodGet,
		http.StatusOK,
		"",
		nil,
		nil,
	)
	assert.Equal(t, -1, statusCode, statusText)
	// Error messages provided by Linux and MacOS respectively.
	const want = "certificate signed by unknown authority|certificate is not trusted"
	assert.Regexp(t, regexp.MustCompile(want), statusText)
}

func TestClientTLSCAFile(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(clientHandler))
	defer s.Close()

	file, err := os.CreateTemp("", "cert_")
	assert.NoError(t, err)
	file.Close()
	defer os.Remove(file.Name())

	err = writeTLSCertificate(s, file.Name())
	assert.NoError(t, err)

	c, _ := newClient(&RunnerCredentials{
		URL:       s.URL,
		TLSCAFile: file.Name(),
	})
	statusCode, statusText, resp := c.doJSONWithPAT(
		context.Background(),
		"test/ok",
		http.MethodGet,
		http.StatusOK,
		"",
		nil,
		nil,
	)
	assert.Equal(t, http.StatusOK, statusCode, statusText)

	tlsData, err := c.getResponseTLSData(resp.TLS, true)
	assert.NoError(t, err)
	assert.NotEmpty(t, tlsData.CAChain)
}

func TestClientCertificateInPredefinedDirectory(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(clientHandler))
	defer s.Close()

	serverURL, err := url.Parse(s.URL)
	require.NoError(t, err)
	hostname, _, err := net.SplitHostPort(serverURL.Host)
	require.NoError(t, err)

	tempDir := t.TempDir()
	CertificateDirectory = tempDir

	err = writeTLSCertificate(s, filepath.Join(tempDir, hostname+".crt"))
	assert.NoError(t, err)

	c, _ := newClient(&RunnerCredentials{
		URL: s.URL,
	})
	statusCode, statusText, resp := c.doJSONWithPAT(
		context.Background(),
		"test/ok",
		http.MethodGet,
		http.StatusOK,
		"",
		nil,
		nil,
	)
	assert.Equal(t, http.StatusOK, statusCode, statusText)

	tlsData, err := c.getResponseTLSData(resp.TLS, true)
	assert.NoError(t, err)
	assert.NotEmpty(t, tlsData.CAChain)
}

func TestClientInvalidTLSAuth(t *testing.T) {
	s := httptest.NewUnstartedServer(http.HandlerFunc(clientHandler))
	s.TLS = new(tls.Config)
	s.TLS.ClientAuth = tls.RequireAnyClientCert
	s.StartTLS()
	defer s.Close()

	ca, err := os.CreateTemp("", "cert_")
	assert.NoError(t, err)
	ca.Close()
	defer os.Remove(ca.Name())

	err = writeTLSCertificate(s, ca.Name())
	assert.NoError(t, err)

	c, _ := newClient(&RunnerCredentials{
		URL:       s.URL,
		TLSCAFile: ca.Name(),
	})
	statusCode, statusText, _ := c.doJSONWithPAT(
		context.Background(),
		"test/ok",
		http.MethodGet,
		http.StatusOK,
		"",
		nil,
		nil,
	)
	assert.Equal(t, -1, statusCode, statusText)
	assert.Contains(t, statusText, "tls: bad certificate")
}

func TestClientTLSAuth(t *testing.T) {
	s := httptest.NewUnstartedServer(http.HandlerFunc(clientHandler))
	s.TLS = new(tls.Config)
	s.TLS.ClientAuth = tls.RequireAnyClientCert
	s.StartTLS()
	defer s.Close()

	ca, err := os.CreateTemp("", "cert_")
	assert.NoError(t, err)
	ca.Close()
	defer os.Remove(ca.Name())

	err = writeTLSCertificate(s, ca.Name())
	assert.NoError(t, err)

	cert, err := os.CreateTemp("", "cert_")
	assert.NoError(t, err)
	cert.Close()
	defer os.Remove(cert.Name())

	key, err := os.CreateTemp("", "key_")
	assert.NoError(t, err)
	key.Close()
	defer os.Remove(key.Name())

	err = writeTLSKeyPair(s, cert.Name(), key.Name())
	assert.NoError(t, err)

	c, _ := newClient(&RunnerCredentials{
		URL:         s.URL,
		TLSCAFile:   ca.Name(),
		TLSCertFile: cert.Name(),
		TLSKeyFile:  key.Name(),
	})

	statusCode, statusText, resp := c.doJSONWithPAT(
		context.Background(),
		"test/ok",
		http.MethodGet,
		http.StatusOK,
		"",
		nil,
		nil,
	)
	assert.Equal(t, http.StatusOK, statusCode, statusText)

	tlsData, err := c.getResponseTLSData(resp.TLS, true)
	assert.NoError(t, err)
	assert.NotEmpty(t, tlsData.CAChain)
	assert.Equal(t, cert.Name(), tlsData.CertFile)
	assert.Equal(t, key.Name(), tlsData.KeyFile)
}

func TestClientTLSAuthCertificatesInPredefinedDirectory(t *testing.T) {
	s := httptest.NewUnstartedServer(http.HandlerFunc(clientHandler))
	s.TLS = new(tls.Config)
	s.TLS.ClientAuth = tls.RequireAnyClientCert
	s.StartTLS()
	defer s.Close()

	tempDir := t.TempDir()
	CertificateDirectory = tempDir

	serverURL, err := url.Parse(s.URL)
	require.NoError(t, err)
	hostname, _, err := net.SplitHostPort(serverURL.Host)
	require.NoError(t, err)

	err = writeTLSCertificate(s, filepath.Join(tempDir, hostname+".crt"))
	assert.NoError(t, err)

	err = writeTLSKeyPair(
		s,
		filepath.Join(tempDir, hostname+".auth.crt"),
		filepath.Join(tempDir, hostname+".auth.key"),
	)
	assert.NoError(t, err)

	c, _ := newClient(&RunnerCredentials{
		URL: s.URL,
	})
	statusCode, statusText, resp := c.doJSONWithPAT(
		context.Background(),
		"test/ok",
		http.MethodGet,
		http.StatusOK,
		"",
		nil,
		nil,
	)
	assert.Equal(t, http.StatusOK, statusCode, statusText)

	tlsData, err := c.getResponseTLSData(resp.TLS, true)
	assert.NoError(t, err)
	assert.NotEmpty(t, tlsData.CAChain)
	assert.NotEmpty(t, tlsData.CertFile)
	assert.NotEmpty(t, tlsData.KeyFile)
}

func TestUrlFixing(t *testing.T) {
	assert.Equal(t, "https://gitlab.example.com", fixCIURL("https://gitlab.example.com/ci///"))
	assert.Equal(t, "https://gitlab.example.com", fixCIURL("https://gitlab.example.com/ci/"))
	assert.Equal(t, "https://gitlab.example.com", fixCIURL("https://gitlab.example.com/ci"))
	assert.Equal(t, "https://gitlab.example.com", fixCIURL("https://gitlab.example.com/"))
	assert.Equal(t, "https://gitlab.example.com", fixCIURL("https://gitlab.example.com///"))
	assert.Equal(t, "https://gitlab.example.com", fixCIURL("https://gitlab.example.com"))
	assert.Equal(t, "https://example.com/gitlab", fixCIURL("https://example.com/gitlab/ci/"))
	assert.Equal(t, "https://example.com/gitlab", fixCIURL("https://example.com/gitlab/ci///"))
	assert.Equal(t, "https://example.com/gitlab", fixCIURL("https://example.com/gitlab/ci"))
	assert.Equal(t, "https://example.com/gitlab", fixCIURL("https://example.com/gitlab/"))
	assert.Equal(t, "https://example.com/gitlab", fixCIURL("https://example.com/gitlab///"))
	assert.Equal(t, "https://example.com/gitlab", fixCIURL("https://example.com/gitlab"))
}

func charsetTestClientHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/v4/with-charset":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"key\":\"value\"}")
	case "/api/v4/without-charset":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"key\":\"value\"}")
	case "/api/v4/without-json":
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"key\":\"value\"}")
	case "/api/v4/invalid-header":
		w.Header().Set("Content-Type", "application/octet-stream, test, a=b")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"key\":\"value\"}")
	}
}

func TestClientHandleCharsetInContentType(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(charsetTestClientHandler))
	defer s.Close()

	c, _ := newClient(&RunnerCredentials{
		URL: s.URL,
	})

	res := struct {
		Key string `json:"key"`
	}{}

	statusCode, statusText, _ := c.doJSONWithPAT(
		context.Background(),
		"with-charset",
		http.MethodGet,
		http.StatusOK,
		"",
		nil,
		&res,
	)
	assert.Equal(t, http.StatusOK, statusCode, statusText)

	statusCode, statusText, _ = c.doJSONWithPAT(
		context.Background(),
		"without-charset",
		http.MethodGet,
		http.StatusOK,
		"",
		nil,
		&res,
	)
	assert.Equal(t, http.StatusOK, statusCode, statusText)

	statusCode, statusText, _ = c.doJSONWithPAT(
		context.Background(),
		"without-json",
		http.MethodGet,
		http.StatusOK,
		"",
		nil,
		&res,
	)
	assert.Equal(t, -1, statusCode, statusText)

	statusCode, statusText, _ = c.doJSONWithPAT(
		context.Background(),
		"invalid-header",
		http.MethodGet,
		http.StatusOK,
		"",
		nil,
		&res,
	)
	assert.Equal(t, -1, statusCode, statusText)
}

type backoffTestCase struct {
	responseStatus int
	mustBackoff    bool
}

func tooManyRequestsHandler(w http.ResponseWriter, r *http.Request) {
	status, err := strconv.Atoi(r.Header.Get("responseStatus"))
	if err != nil {
		w.WriteHeader(599)
	} else {
		w.WriteHeader(status)
	}
}

func TestRequestsBackOff(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(tooManyRequestsHandler))
	defer s.Close()

	c, _ := newClient(&RunnerCredentials{
		URL: s.URL,
	})

	testCases := []backoffTestCase{
		{http.StatusCreated, false},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusOK, false},
		{http.StatusConflict, true},
		{http.StatusTooManyRequests, true},
		{http.StatusCreated, false},
		{http.StatusInternalServerError, true},
		{http.StatusTooManyRequests, true},
		{599, true},
		{499, true},
	}

	backoff := c.ensureBackoff(http.MethodPost, "")
	for id, testCase := range testCases {
		t.Run(fmt.Sprintf("%d-%d", id, testCase.responseStatus), func(t *testing.T) {
			backoff.Reset()
			assert.Zero(t, backoff.Attempt())

			var body io.Reader
			headers := make(http.Header)
			headers.Add("responseStatus", strconv.Itoa(testCase.responseStatus))

			res, err := c.do(context.Background(), "/", http.MethodPost, body, "application/json", headers)

			assert.NoError(t, err)
			assert.Equal(t, testCase.responseStatus, res.StatusCode)

			var expected float64
			if testCase.mustBackoff {
				expected = 1.0
			}
			assert.Equal(t, expected, backoff.Attempt())
		})
	}
}

func TestRequesterCalled(t *testing.T) {
	c, _ := newClient(&RunnerCredentials{
		URL: "http://localhost:1000/",
	})

	rl := &mockRequester{}
	defer rl.AssertExpectations(t)

	resReturn := &http.Response{
		StatusCode: http.StatusOK,
	}
	rl.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.String() == "http://mockURL" && req.Method == http.MethodGet
	})).Return(resReturn, nil)
	c.requester = rl

	res, _ := c.do(context.Background(), "http://mockURL", http.MethodGet, nil, "", nil)
	assert.Equal(t, resReturn, res)
}

func Test307and308Redirections(t *testing.T) {
	testPayload := []byte("test payload")

	type codes struct {
		sent     int
		expected int
	}

	defaultCodes := []codes{
		{sent: http.StatusTemporaryRedirect, expected: http.StatusOK},
		{sent: http.StatusPermanentRedirect, expected: http.StatusOK},
	}

	tests := map[string]struct {
		getRequest   func(t *testing.T) io.Reader
		expectedBody []byte
		codes        []codes
	}{
		"nil body": {
			getRequest: func(t *testing.T) io.Reader {
				return nil
			},
			expectedBody: []byte{},
			codes:        defaultCodes,
		},
		"bytes buffer": {
			getRequest: func(t *testing.T) io.Reader {
				return bytes.NewReader(testPayload)
			},
			expectedBody: testPayload,
			codes:        defaultCodes,
		},
		"piped data": {
			getRequest: func(t *testing.T) io.Reader {
				pr, pw := io.Pipe()

				go func() {
					defer func() {
						_ = pw.Close()
					}()

					_, err := io.Copy(pw, bytes.NewReader(testPayload))
					assert.NoError(t, err)
				}()

				return pr
			},
			codes: []codes{
				{sent: http.StatusTemporaryRedirect, expected: http.StatusTemporaryRedirect},
				{sent: http.StatusPermanentRedirect, expected: http.StatusPermanentRedirect},
			},
		},
	}

	for tn, tt := range tests {
		for _, code := range tt.codes {
			t.Run(fmt.Sprintf("code-%d-%s", code.sent, tn), func(t *testing.T) {
				s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
					const redirectionURI = "/redirected"

					if r.RequestURI == redirectionURI {
						body, err := io.ReadAll(r.Body)
						assert.NoError(t, err)

						if !assert.Equal(t, tt.expectedBody, body) {
							rw.WriteHeader(http.StatusInternalServerError)
							return
						}

						rw.WriteHeader(http.StatusOK)
						return
					}

					_, err := io.Copy(io.Discard, r.Body)
					assert.NoError(t, err)

					_ = r.Body.Close()

					rw.Header().Set("Location", redirectionURI)
					rw.WriteHeader(code.sent)
				}))

				u, err := url.Parse(s.URL)
				require.NoError(t, err)

				c := &client{
					url:             u,
					requestBackOffs: make(map[string]*backoff.Backoff),
				}
				c.requester = &c.Client

				response, err := c.do(context.Background(), "/", http.MethodPatch, tt.getRequest(t), "", nil)
				assert.NoError(t, err)
				if assert.NotNil(t, response) {
					assert.Equal(t, code.expected, response.StatusCode)
				}
			})
		}
	}
}

func TestEnsureUserAgentAlwaysSent(t *testing.T) {
	tests := map[string]struct {
		r io.Reader
	}{
		"request reader is present": {
			r: bytes.NewBufferString("test"),
		},
		"request reader is empty": {
			r: nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				assert.Equal(t, AppVersion.UserAgent(), r.UserAgent())
				rw.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			url, err := url.Parse(server.URL)
			require.NoError(t, err)

			c := &client{
				url:             url,
				requestBackOffs: make(map[string]*backoff.Backoff),
			}
			c.requester = &c.Client

			headers := http.Header{}
			headers.Set("Test", "test")

			response, err := c.do(context.Background(), "/", http.MethodGet, tt.r, "", headers)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, response.StatusCode)
		})
	}
}

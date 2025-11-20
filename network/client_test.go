//go:build !integration

package network

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	. "gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/certificate"
)

func clientHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	logrus.Debugln(
		r.Method, r.URL.String(),
		"Content-Type:", r.Header.Get(ContentType),
		"Accept:", r.Header.Get(Accept),
		"Body:", string(body),
	)

	switch r.URL.Path {
	case "/api/v4/test/ok":
	case "/api/v4/test/auth":
		w.WriteHeader(http.StatusForbidden)
	case "/api/v4/test/json":
		if r.Header.Get(ContentType) != "application/json" {
			w.Header().Set(ContentType, "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"message":{"some-key":["some error"]}}`)
			return
		}
		if r.Header.Get(Accept) != "application/json" {
			w.Header().Set(ContentType, "application/json")
			w.WriteHeader(http.StatusNotAcceptable)
			fmt.Fprint(w, `{"message":"406 Not Acceptable"}`)
			return
		}

		switch r.Header.Get(PrivateToken) {
		case "":
			w.Header().Set(ContentType, "application/json")
			fmt.Fprint(w, `{"key":"value"}`)
		case "my-pat":
			w.Header().Set(ContentType, "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"key":"value","pat":"my-pat"}`)
		default:
			w.Header().Set(ContentType, "application/json")
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
	t.Parallel()

	testCases := []struct {
		name            string
		creds           *RunnerCredentials
		expectedErr     string
		expectedBaseURL string
	}{
		{
			name: "success",
			creds: &RunnerCredentials{
				URL: "http://test.example.com/ci///",
			},
			expectedBaseURL: "http://test.example.com/api/v4/",
		},
		{
			name: "failed to parse url",
			creds: &RunnerCredentials{
				URL: "\n",
			},
			expectedErr: "parse URL",
		},
		{
			name: "not http or https",
			creds: &RunnerCredentials{
				URL: "example.com",
			},
			expectedErr: "only http or https scheme supported",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newClient(tc.creds, NewAPIRequestsCollector())

			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.expectedErr)
				assert.Nil(t, c)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, c)
				assert.Equal(t, c.url.String(), tc.expectedBaseURL)
			}
		})
	}
}

func TestServerCertificateChange(t *testing.T) {
	gen := certificate.X509Generator{}

	// we use net.Listen and tls.Listener to build our own "httptest"-esque TLS server here,
	// because the httptest package doesn't give you enough control over the TLS certificate
	// setup.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	// create a very impractical TLS server that changes the TLS certificate on every connection.
	srv := &http.Server{Addr: ln.Addr().String(), Handler: http.HandlerFunc(clientHandler)}
	srv.TLSConfig = &tls.Config{
		GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, _, err := gen.Generate("127.0.0.1")
			return &cert, err
		},
	}

	// serve TLS
	tlsListener := tls.NewListener(ln, srv.TLSConfig)
	go func() {
		errServe := srv.Serve(tlsListener)
		require.EqualError(t, errServe, "http: Server closed")
	}()
	defer srv.Close()

	// create runner client
	c, err := newClient(&RunnerCredentials{
		URL: "https://" + ln.Addr().String(),
	}, NewAPIRequestsCollector())
	require.NoError(t, err)
	require.NotNil(t, c)

	// we cheat here and skip verification so that we don't need a bunch of
	// valid certificates from the client's perspective.
	c.createTransport()
	c.Client.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = true

	//
	var cachedCA []byte
	for i := 0; i < 10; i++ {
		statusCode, statusText, resp := c.doJSON(
			t.Context(),
			"test/ok",
			http.MethodGet,
			http.StatusOK,
			nil,
			nil,
			nil,
		)
		assert.Equal(t, http.StatusOK, statusCode, statusText)

		// force a client transport refresh, without this, the
		// PeerCertificates will not change.
		c.connectionMaxAge = 1
		c.lastIdleRefresh = time.Now().Add(-10 * time.Second)
		c.ensureTransportMaxAge()

		sum := sha256.Sum256(resp.TLS.PeerCertificates[0].Raw)
		if cachedCA != nil {
			require.NotEqual(t, sum[:], cachedCA, "ca was cached and should not have been")
		}
		cachedCA = sum[:]
	}
}

func TestClient_Do(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		ctx         context.Context
		uri         string
		url         string
		method      string
		setup       func(tb testing.TB) (ContentProvider, requester)
		requestType string
		headers     http.Header
		expectedErr string
		expectedRes *http.Response
	}{
		{
			name: "failed to parse url",
			ctx:  t.Context(),
			uri:  "\n",
			setup: func(tb testing.TB) (ContentProvider, requester) {
				return NewMockContentProvider(t), newMockRequester(tb)
			},
			expectedErr: "parse URL",
		},
		{
			name: "get reader error",
			ctx:  t.Context(),
			uri:  "/test",
			setup: func(tb testing.TB) (ContentProvider, requester) {
				mcp := NewMockContentProvider(t)
				mcp.On("GetReader").Return(nil, errors.New("computer said no"))
				return mcp, newMockRequester(tb)
			},
			expectedErr: "get reader",
		},
		{
			name: "create request error",
			ctx:  nil,
			uri:  "/test",
			setup: func(tb testing.TB) (ContentProvider, requester) {
				mcp := NewMockContentProvider(t)
				mcp.On("GetReader").Return(io.NopCloser(strings.NewReader("test")), nil)
				return mcp, newMockRequester(tb)
			},
			expectedErr: "create NewRequest",
		},
		{
			name:        "execute request error",
			ctx:         t.Context(),
			uri:         "/test",
			url:         "http://invalid.com",
			method:      http.MethodPost,
			requestType: "application/json",
			headers: http.Header{
				"Custom-Header": {"test-custom-header"},
			},
			setup: func(tb testing.TB) (ContentProvider, requester) {
				mcp := NewMockContentProvider(t)
				testRequestBody := "test"
				mcp.On("GetReader").Return(io.NopCloser(strings.NewReader(testRequestBody)), nil)
				mcp.On("GetContentLength").Return(int64(len(testRequestBody)), true)

				mr := newMockRequester(tb)
				mr.On("Do", mock.Anything).Return(nil, errors.New("request error")).Once()

				return mcp, mr
			},
			expectedErr: "execute request",
		},
		{
			name:        "success",
			ctx:         t.Context(),
			uri:         "/test",
			method:      http.MethodPost,
			requestType: "application/json",
			headers: http.Header{
				"Custom-Header": {"test-custom-header"},
			},
			setup: func(tb testing.TB) (ContentProvider, requester) {
				mcp := NewMockContentProvider(t)
				testRequestBody := "test"
				mcp.On("GetReader").Return(io.NopCloser(strings.NewReader(testRequestBody)), nil)
				mcp.On("GetContentLength").Return(int64(len(testRequestBody)), true)

				mr := newMockRequester(tb)
				mr.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					require.Equal(tb, t.Context(), req.Context())
					require.Equal(tb, req.Method, http.MethodPost)
					require.Equal(tb, req.Header.Get("Custom-Header"), "test-custom-header")
					require.Equal(tb, req.Header.Get("Content-Type"), "application/json")
					require.Equal(tb, req.ContentLength, int64(4))
					return true
				})).Return(&http.Response{
					StatusCode: http.StatusOK,
				}, nil).Once()

				return mcp, mr
			},
			expectedRes: &http.Response{StatusCode: http.StatusOK},
		},
		{
			name:        "success nil body",
			ctx:         t.Context(),
			uri:         "/test",
			method:      http.MethodPost,
			requestType: "application/json",
			headers: http.Header{
				"Custom-Header": {"test-custom-header"},
			},
			setup: func(tb testing.TB) (ContentProvider, requester) {
				mcp := NewMockContentProvider(t)
				testRequestBody := "test"
				mcp.On("GetReader").Return(io.NopCloser(strings.NewReader(testRequestBody)), nil)
				mcp.On("GetContentLength").Return(int64(len(testRequestBody)), true)

				mr := newMockRequester(tb)
				mr.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					require.Equal(tb, t.Context(), req.Context())
					require.Equal(tb, req.Method, http.MethodPost)
					require.Equal(tb, req.Header.Get("Custom-Header"), "test-custom-header")
					require.Equal(tb, req.Header.Get("Content-Type"), "application/json")
					require.Equal(tb, req.ContentLength, int64(4))
					return true
				})).Return(&http.Response{
					StatusCode: http.StatusOK,
				}, nil).Once()

				return mcp, mr
			},
			expectedRes: &http.Response{StatusCode: http.StatusOK},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newClient(&RunnerCredentials{
				URL: "http://example.com",
			}, NewAPIRequestsCollector())
			require.NoError(t, err)
			require.NotNil(t, c)

			mcp, mr := tc.setup(t)

			c.requester = mr

			res, err := c.do(tc.ctx, tc.uri, tc.method, mcp, tc.requestType, tc.headers)

			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.Equal(t, tc.expectedRes, res)
			}
		})
	}
}

func TestClient_DoJSON(t *testing.T) {
	t.Parallel()

	type (
		Request struct {
			FirstName string `json:"firstName"`
		}
		Response struct {
			LastName string `json:"lastName"`
		}
	)
	testCases := []struct {
		name               string
		uri                string
		method             string
		statusCode         int
		headers            http.Header
		request            any
		response           *Response
		success            bool
		mockHandler        func(tb testing.TB) func(w http.ResponseWriter, r *http.Request)
		expectedStatusCode int
		expectedStatusText string
	}{
		{
			name:    "failed to marshal request",
			request: math.NaN(),
			mockHandler: func(tb testing.TB) func(w http.ResponseWriter, r *http.Request) {
				tb.Helper()
				return func(w http.ResponseWriter, r *http.Request) {}
			},
			expectedStatusCode: -1,
			expectedStatusText: "marshal request object: json: unsupported value: NaN",
		},
		{
			name:   "execute json request",
			uri:    "\n",
			method: http.MethodPost,
			mockHandler: func(tb testing.TB) func(w http.ResponseWriter, r *http.Request) {
				tb.Helper()
				return func(w http.ResponseWriter, r *http.Request) {}
			},
			expectedStatusCode: -1,
			expectedStatusText: "execute JSON request",
		},
		{
			name:       "response is not application/json",
			uri:        "/test/uri",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			mockHandler: func(tb testing.TB) func(w http.ResponseWriter, r *http.Request) {
				tb.Helper()
				return func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/test")
				}
			},
			response:           &Response{},
			expectedStatusCode: -1,
			expectedStatusText: "response is not application/json",
		},
		{
			name:       "error decoding json payload",
			uri:        "test/uri",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			mockHandler: func(tb testing.TB) func(w http.ResponseWriter, r *http.Request) {
				tb.Helper()
				return func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, err := w.Write([]byte("\n"))
					require.NoError(t, err)
				}
			},
			response:           &Response{},
			expectedStatusCode: -1,
			expectedStatusText: "decoding json payload",
		},
		{
			name:       "status forbidden",
			uri:        "test/uri",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			headers: http.Header{
				"Content-Type": {"application/json"},
				"Custom":       {"custom/header"},
			},
			request: &Request{FirstName: "test-first-name"},
			mockHandler: func(tb testing.TB) func(w http.ResponseWriter, r *http.Request) {
				tb.Helper()
				return func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(tb, r.Method, http.MethodPost)
					assert.Equal(tb, r.Header.Get("Content-Type"), "application/json")
					assert.Equal(tb, r.Header.Get("Custom"), "custom/header")

					var reqBody Request
					err := json.NewDecoder(r.Body).Decode(&reqBody)
					require.NoError(t, err)

					assert.Equal(tb, reqBody.FirstName, "test-first-name")

					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-GitLab-Last-Update", "gitlab-last-update")
					w.WriteHeader(http.StatusBadRequest)

					err = json.NewEncoder(w).Encode(Response{LastName: "test-last-name"})
					require.NoError(tb, err)
				}
			},
			response:           &Response{},
			expectedStatusCode: http.StatusBadRequest,
			expectedStatusText: http.StatusText(http.StatusBadRequest),
		},
		{
			name:       "success status ok",
			uri:        "test/uri",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			headers: http.Header{
				"Content-Type": {"application/json"},
				"Custom":       {"custom/header"},
			},
			request: &Request{FirstName: "test-first-name"},
			mockHandler: func(tb testing.TB) func(w http.ResponseWriter, r *http.Request) {
				tb.Helper()
				return func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(tb, r.Method, http.MethodPost)
					assert.Equal(tb, r.Header.Get("Content-Type"), "application/json")
					assert.Equal(tb, r.Header.Get("Custom"), "custom/header")

					var reqBody Request
					err := json.NewDecoder(r.Body).Decode(&reqBody)
					require.NoError(t, err)

					assert.Equal(tb, reqBody.FirstName, "test-first-name")

					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-GitLab-Last-Update", "gitlab-last-update")

					err = json.NewEncoder(w).Encode(Response{LastName: "test-last-name"})
					require.NoError(tb, err)
				}
			},
			response:           &Response{},
			success:            true,
			expectedStatusCode: http.StatusOK,
			expectedStatusText: http.StatusText(http.StatusOK),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(tc.mockHandler(t)))
			defer s.Close()

			c, err := newClient(&RunnerCredentials{
				URL: s.URL,
			}, NewAPIRequestsCollector())
			require.NoError(t, err)
			require.NotNil(t, c)

			statusCode, statusText, _ := c.doJSON(t.Context(), tc.uri, tc.method, tc.statusCode, tc.headers, tc.request, tc.response)

			assert.Equal(t, tc.expectedStatusCode, statusCode)
			assert.Contains(t, statusText, tc.expectedStatusText)

			if tc.success {
				assert.NotEmpty(t, tc.response.LastName)
				assert.Equal(t, c.getLastUpdate(), "gitlab-last-update")
			}
		})
	}
}

func TestClientInvalidSSL(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(clientHandler))
	defer s.Close()

	c, _ := newClient(&RunnerCredentials{
		URL: s.URL,
	}, NewAPIRequestsCollector())
	statusCode, statusText, _ := c.doJSON(
		t.Context(),
		"test/ok",
		http.MethodGet,
		http.StatusOK,
		nil,
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
	}, NewAPIRequestsCollector())
	statusCode, statusText, resp := c.doJSON(
		t.Context(),
		"test/ok",
		http.MethodGet,
		http.StatusOK,
		nil,
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

	err = writeTLSCertificate(s, filepath.Join(tempDir, hostname+".crt"))
	assert.NoError(t, err)

	c, _ := newClient(&RunnerCredentials{
		URL: s.URL,
	}, NewAPIRequestsCollector(), withCertificateDirectory(tempDir))
	statusCode, statusText, resp := c.doJSON(
		t.Context(),
		"test/ok",
		http.MethodGet,
		http.StatusOK,
		nil,
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
	}, NewAPIRequestsCollector())
	statusCode, statusText, _ := c.doJSON(
		t.Context(),
		"test/ok",
		http.MethodGet,
		http.StatusOK,
		nil,
		nil,
		nil,
	)
	assert.Equal(t, -1, statusCode, statusText)
	assert.Contains(t, statusText, "tls: certificate required")
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
	}, NewAPIRequestsCollector())

	statusCode, statusText, resp := c.doJSON(
		t.Context(),
		"test/ok",
		http.MethodGet,
		http.StatusOK,
		nil,
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
	}, NewAPIRequestsCollector(), withCertificateDirectory(tempDir))
	statusCode, statusText, resp := c.doJSON(
		t.Context(),
		"test/ok",
		http.MethodGet,
		http.StatusOK,
		nil,
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
		w.Header().Set(ContentType, "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"key\":\"value\"}")
	case "/api/v4/without-charset":
		w.Header().Set(ContentType, "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"key\":\"value\"}")
	case "/api/v4/without-json":
		w.Header().Set(ContentType, "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"key\":\"value\"}")
	case "/api/v4/invalid-header":
		w.Header().Set(ContentType, "application/octet-stream, test, a=b")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"key\":\"value\"}")
	}
}

func TestClientHandleCharsetInContentType(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(charsetTestClientHandler))
	defer s.Close()

	c, _ := newClient(&RunnerCredentials{
		URL: s.URL,
	}, NewAPIRequestsCollector())

	res := struct {
		Key string `json:"key"`
	}{}

	statusCode, statusText, _ := c.doJSON(
		t.Context(),
		"with-charset",
		http.MethodGet,
		http.StatusOK,
		nil,
		nil,
		&res,
	)
	assert.Equal(t, http.StatusOK, statusCode, statusText)

	statusCode, statusText, _ = c.doJSON(
		t.Context(),
		"without-charset",
		http.MethodGet,
		http.StatusOK,
		nil,
		nil,
		&res,
	)
	assert.Equal(t, http.StatusOK, statusCode, statusText)

	statusCode, statusText, _ = c.doJSON(
		t.Context(),
		"without-json",
		http.MethodGet,
		http.StatusOK,
		nil,
		nil,
		&res,
	)
	assert.Equal(t, -1, statusCode, statusText)

	statusCode, statusText, _ = c.doJSON(
		t.Context(),
		"invalid-header",
		http.MethodGet,
		http.StatusOK,
		nil,
		nil,
		&res,
	)
	assert.Equal(t, -1, statusCode, statusText)
}

func TestRequesterCalled(t *testing.T) {
	c, _ := newClient(&RunnerCredentials{
		URL: "http://localhost:1000/",
	}, NewAPIRequestsCollector())

	rl := newMockRequester(t)

	resReturn := &http.Response{
		StatusCode: http.StatusOK,
	}
	rl.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.String() == "http://mockURL" && req.Method == http.MethodGet
	})).Return(resReturn, nil)
	c.requester = rl

	res, _ := c.do(t.Context(), "http://mockURL", http.MethodGet, nil, "", nil)
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
		bodyProvider func(t *testing.T) ContentProvider
		expectedBody []byte
		codes        []codes
	}{
		"nil body": {
			bodyProvider: func(t *testing.T) ContentProvider {
				return nil
			},
			expectedBody: []byte{},
			codes:        defaultCodes,
		},
		"bytes buffer": {
			bodyProvider: func(t *testing.T) ContentProvider {
				return BytesProvider{
					Data: testPayload,
				}
			},
			expectedBody: testPayload,
			codes:        defaultCodes,
		},
		"piped data": {
			bodyProvider: func(t *testing.T) ContentProvider {
				return StreamProvider{
					ReaderFactory: func() (io.ReadCloser, error) {
						pr, pw := io.Pipe()

						go func() {
							defer pw.Close()
							_, err := pw.Write(testPayload)
							assert.NoError(t, err)
						}()

						return pr, nil
					},
				}
			},
			expectedBody: testPayload,
			codes:        defaultCodes,
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
					url: u,
				}
				c.requester = &c.Client

				response, err := c.do(t.Context(), "/", http.MethodPatch, tt.bodyProvider(t), "", nil)
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
		b ContentProvider
	}{
		"request reader is present": {
			b: BytesProvider{
				Data: []byte("test"),
			},
		},
		"request reader is empty": {
			b: nil,
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
				url: url,
			}
			c.requester = &c.Client

			headers := http.Header{}
			headers.Set("Test", "test")

			response, err := c.do(t.Context(), "/", http.MethodGet, tt.b, "", headers)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, response.StatusCode)
		})
	}
}

func TestWithMaxAge(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		age  time.Duration
	}{
		{
			name: "set age",
			age:  10 * time.Second,
		},
		{
			name: "no value",
		},
	}

	for _, tc := range testCases {
		c := &client{}

		withMaxAge(tc.age)(c)

		assert.Equal(t, c.connectionMaxAge, tc.age)
	}
}

//go:build !integration

package network

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
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
	})
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
			context.Background(),
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

func TestClientDo(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(clientHandler))
	defer s.Close()

	c, err := newClient(&RunnerCredentials{
		URL: s.URL,
	})
	assert.NoError(t, err)
	assert.NotNil(t, c)

	statusCode, statusText, _ := c.doJSON(
		context.Background(),
		"test/auth",
		http.MethodGet,
		http.StatusOK,
		nil,
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

	statusCode, statusText, _ = c.doJSON(
		context.Background(),
		"test/json",
		http.MethodGet,
		http.StatusOK,
		nil,
		nil,
		&res,
	)
	assert.Equal(t, http.StatusBadRequest, statusCode, statusText)
	assert.Contains(t, statusText, `test/json: 400 Bad Request (some-key: some error)`)

	statusCode, statusText, _ = c.doJSON(
		context.Background(),
		"test/json",
		http.MethodGet,
		http.StatusOK,
		nil,
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

	statusCode, statusText, _ = c.doJSON(
		context.Background(),
		"test/json",
		http.MethodGet,
		http.StatusOK,
		nil,
		nil,
		nil,
	)
	assert.Equal(t, http.StatusBadRequest, statusCode, statusText)
	assert.Contains(t, statusText, `test/json: 400 Bad Request (some-key: some error)`)

	statusCode, statusText, _ = c.doJSON(
		context.Background(),
		"test/json",
		http.MethodGet,
		http.StatusOK,
		nil,
		&req,
		&res,
	)
	assert.Equal(t, http.StatusOK, statusCode, statusText)
	assert.Equal(t, "value", res.Key, statusText)
	assert.Equal(t, (*string)(nil), res.PAT, statusText)

	statusCode, statusText, _ = c.doJSON(
		context.Background(),
		"test/json",
		http.MethodGet,
		http.StatusCreated,
		PrivateTokenHeader("my-pat"),
		&req,
		&res,
	)
	assert.Equal(t, http.StatusCreated, statusCode, statusText)
	assert.Equal(t, "value", res.Key, statusText)
	assert.Equal(t, "my-pat", *res.PAT, statusText)

	statusCode, statusText, _ = c.doJSON(
		context.Background(),
		"test/json",
		http.MethodGet,
		http.StatusCreated,
		PrivateTokenHeader("invalid-pat"),
		&req,
		&res,
	)
	assert.Equal(t, http.StatusForbidden, statusCode, statusText)
	assert.Contains(t, statusText, `test/json: 403 Forbidden`)
}

func TestClientNilBody(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(clientHandler))
	defer s.Close()

	c, err := newClient(&RunnerCredentials{
		URL: s.URL,
	})
	assert.NoError(t, err)
	assert.NotNil(t, c)

	headers := make(http.Header)
	headers.Set(PrivateToken, "test-me")
	headers.Set(ContentType, "application/json")
	headers.Set(Accept, "application/json")

	resp, err := c.do(context.Background(), "/api/v4/test/json", http.MethodGet, nil, "", headers)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

type testContextKey int

func TestClientDo_Context(t *testing.T) {
	bodyProvider := BytesProvider{
		Data: []byte("test"),
	}

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

	requesterMock := newMockRequester(t)
	c.requester = requesterMock

	requesterMock.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return assert.Equal(t, ctx, req.Context())
	})).Return(response, nil).Once()

	res, err := c.do(ctx, "/test", http.MethodPost, bodyProvider, "plain/text", nil)

	assert.NoError(t, err)
	assert.Equal(t, response, res)
}

func TestClientInvalidSSL(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(clientHandler))
	defer s.Close()

	c, _ := newClient(&RunnerCredentials{
		URL: s.URL,
	})
	statusCode, statusText, _ := c.doJSON(
		context.Background(),
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
	})
	statusCode, statusText, resp := c.doJSON(
		context.Background(),
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
	CertificateDirectory = tempDir

	err = writeTLSCertificate(s, filepath.Join(tempDir, hostname+".crt"))
	assert.NoError(t, err)

	c, _ := newClient(&RunnerCredentials{
		URL: s.URL,
	})
	statusCode, statusText, resp := c.doJSON(
		context.Background(),
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
	})
	statusCode, statusText, _ := c.doJSON(
		context.Background(),
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
	})

	statusCode, statusText, resp := c.doJSON(
		context.Background(),
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
	statusCode, statusText, resp := c.doJSON(
		context.Background(),
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
	})

	res := struct {
		Key string `json:"key"`
	}{}

	statusCode, statusText, _ := c.doJSON(
		context.Background(),
		"with-charset",
		http.MethodGet,
		http.StatusOK,
		nil,
		nil,
		&res,
	)
	assert.Equal(t, http.StatusOK, statusCode, statusText)

	statusCode, statusText, _ = c.doJSON(
		context.Background(),
		"without-charset",
		http.MethodGet,
		http.StatusOK,
		nil,
		nil,
		&res,
	)
	assert.Equal(t, http.StatusOK, statusCode, statusText)

	statusCode, statusText, _ = c.doJSON(
		context.Background(),
		"without-json",
		http.MethodGet,
		http.StatusOK,
		nil,
		nil,
		&res,
	)
	assert.Equal(t, -1, statusCode, statusText)

	statusCode, statusText, _ = c.doJSON(
		context.Background(),
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
	})

	rl := newMockRequester(t)

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

				response, err := c.do(context.Background(), "/", http.MethodPatch, tt.bodyProvider(t), "", nil)
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

			response, err := c.do(context.Background(), "/", http.MethodGet, tt.b, "", headers)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, response.StatusCode)
		})
	}
}

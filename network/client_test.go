package network

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

func clientHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	logrus.Debugln(r.Method, r.URL.String(),
		"Content-Type:", r.Header.Get("Content-Type"),
		"Accept:", r.Header.Get("Accept"),
		"Body:", string(body))

	switch r.URL.Path {
	case "/api/v4/test/ok":
	case "/api/v4/test/auth":
		w.WriteHeader(403)
	case "/api/v4/test/json":
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(400)
		} else if r.Header.Get("Accept") != "application/json" {
			w.WriteHeader(406)
		} else {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, "{\"key\":\"value\"}")
		}
	default:
		w.WriteHeader(404)
	}
}

func writeTLSCertificate(s *httptest.Server, file string) error {
	c := s.TLS.Certificates[0]
	if c.Certificate == nil || c.Certificate[0] == nil {
		return errors.New("no predefined certificate")
	}

	encoded := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: c.Certificate[0],
	})

	return ioutil.WriteFile(file, encoded, 0600)
}

func writeTLSKeyPair(s *httptest.Server, certFile string, keyFile string) error {
	c := s.TLS.Certificates[0]
	if c.Certificate == nil || c.Certificate[0] == nil {
		return errors.New("no predefined certificate")
	}

	encodedCert := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: c.Certificate[0],
	})

	if err := ioutil.WriteFile(certFile, encodedCert, 0600); err != nil {
		return err
	}

	switch k := c.PrivateKey.(type) {
	case *rsa.PrivateKey:
		encodedKey := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(k),
		})
		return ioutil.WriteFile(keyFile, encodedKey, 0600)
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

	statusCode, statusText, _ := c.doJSON("test/auth", "GET", 200, nil, nil)
	assert.Equal(t, 403, statusCode, statusText)

	req := struct {
		Query bool `json:"query"`
	}{
		true,
	}

	res := struct {
		Key string `json:"key"`
	}{}

	statusCode, statusText, _ = c.doJSON("test/json", "GET", 200, nil, &res)
	assert.Equal(t, 400, statusCode, statusText)

	statusCode, statusText, _ = c.doJSON("test/json", "GET", 200, &req, nil)
	assert.Equal(t, 406, statusCode, statusText)

	statusCode, statusText, _ = c.doJSON("test/json", "GET", 200, nil, nil)
	assert.Equal(t, 400, statusCode, statusText)

	statusCode, statusText, _ = c.doJSON("test/json", "GET", 200, &req, &res)
	assert.Equal(t, 200, statusCode, statusText)
	assert.Equal(t, "value", res.Key, statusText)
}

func TestClientInvalidSSL(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(clientHandler))
	defer s.Close()

	c, _ := newClient(&RunnerCredentials{
		URL: s.URL,
	})
	statusCode, statusText, _ := c.doJSON("test/ok", "GET", 200, nil, nil)
	assert.Equal(t, -1, statusCode, statusText)
	assert.Contains(t, statusText, "certificate signed by unknown authority")
}

func TestClientTLSCAFile(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(clientHandler))
	defer s.Close()

	file, err := ioutil.TempFile("", "cert_")
	assert.NoError(t, err)
	file.Close()
	defer os.Remove(file.Name())

	err = writeTLSCertificate(s, file.Name())
	assert.NoError(t, err)

	c, _ := newClient(&RunnerCredentials{
		URL:       s.URL,
		TLSCAFile: file.Name(),
	})
	statusCode, statusText, tlsData := c.doJSON("test/ok", "GET", 200, nil, nil)
	assert.Equal(t, 200, statusCode, statusText)
	assert.NotEmpty(t, tlsData.CAChain)
}

func TestClientCertificateInPredefinedDirectory(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(clientHandler))
	defer s.Close()

	serverURL, err := url.Parse(s.URL)
	require.NoError(t, err)
	hostname, _, err := net.SplitHostPort(serverURL.Host)
	require.NoError(t, err)

	tempDir, err := ioutil.TempDir("", "certs")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)
	CertificateDirectory = tempDir

	err = writeTLSCertificate(s, filepath.Join(tempDir, hostname+".crt"))
	assert.NoError(t, err)

	c, _ := newClient(&RunnerCredentials{
		URL: s.URL,
	})
	statusCode, statusText, tlsData := c.doJSON("test/ok", "GET", 200, nil, nil)
	assert.Equal(t, 200, statusCode, statusText)
	assert.NotEmpty(t, tlsData.CAChain)
}

func TestClientInvalidTLSAuth(t *testing.T) {
	s := httptest.NewUnstartedServer(http.HandlerFunc(clientHandler))
	s.TLS = new(tls.Config)
	s.TLS.ClientAuth = tls.RequireAnyClientCert
	s.StartTLS()
	defer s.Close()

	ca, err := ioutil.TempFile("", "cert_")
	assert.NoError(t, err)
	ca.Close()
	defer os.Remove(ca.Name())

	err = writeTLSCertificate(s, ca.Name())
	assert.NoError(t, err)

	c, _ := newClient(&RunnerCredentials{
		URL:       s.URL,
		TLSCAFile: ca.Name(),
	})
	statusCode, statusText, _ := c.doJSON("test/ok", "GET", 200, nil, nil)
	assert.Equal(t, -1, statusCode, statusText)
	assert.Contains(t, statusText, "tls: bad certificate")
}

func TestClientTLSAuth(t *testing.T) {
	s := httptest.NewUnstartedServer(http.HandlerFunc(clientHandler))
	s.TLS = new(tls.Config)
	s.TLS.ClientAuth = tls.RequireAnyClientCert
	s.StartTLS()
	defer s.Close()

	ca, err := ioutil.TempFile("", "cert_")
	assert.NoError(t, err)
	ca.Close()
	defer os.Remove(ca.Name())

	err = writeTLSCertificate(s, ca.Name())
	assert.NoError(t, err)

	cert, err := ioutil.TempFile("", "cert_")
	assert.NoError(t, err)
	cert.Close()
	defer os.Remove(cert.Name())

	key, err := ioutil.TempFile("", "key_")
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
	statusCode, statusText, tlsData := c.doJSON("test/ok", "GET", 200, nil, nil)
	assert.Equal(t, 200, statusCode, statusText)
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

	tempDir, err := ioutil.TempDir("", "certs")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)
	CertificateDirectory = tempDir

	serverURL, err := url.Parse(s.URL)
	require.NoError(t, err)
	hostname, _, err := net.SplitHostPort(serverURL.Host)
	require.NoError(t, err)

	err = writeTLSCertificate(s, filepath.Join(tempDir, hostname+".crt"))
	assert.NoError(t, err)

	err = writeTLSKeyPair(s,
		filepath.Join(tempDir, hostname+".auth.crt"),
		filepath.Join(tempDir, hostname+".auth.key"))
	assert.NoError(t, err)

	c, _ := newClient(&RunnerCredentials{
		URL: s.URL,
	})
	statusCode, statusText, tlsData := c.doJSON("test/ok", "GET", 200, nil, nil)
	assert.Equal(t, 200, statusCode, statusText)
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
		w.WriteHeader(200)
		fmt.Fprint(w, "{\"key\":\"value\"}")
	case "/api/v4/without-charset":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, "{\"key\":\"value\"}")
	case "/api/v4/without-json":
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "{\"key\":\"value\"}")
	case "/api/v4/invalid-header":
		w.Header().Set("Content-Type", "application/octet-stream, test, a=b")
		w.WriteHeader(200)
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

	statusCode, statusText, _ := c.doJSON("with-charset", "GET", 200, nil, &res)
	assert.Equal(t, 200, statusCode, statusText)

	statusCode, statusText, _ = c.doJSON("without-charset", "GET", 200, nil, &res)
	assert.Equal(t, 200, statusCode, statusText)

	statusCode, statusText, _ = c.doJSON("without-json", "GET", 200, nil, &res)
	assert.Equal(t, -1, statusCode, statusText)

	statusCode, statusText, _ = c.doJSON("invalid-header", "GET", 200, nil, &res)
	assert.Equal(t, -1, statusCode, statusText)
}

type backoffTestCase struct {
	responseStatus int
	minDelayFactor float64
	maxDelayFactor float64
}

func compareDurations(t *testing.T, testCase backoffTestCase, previousDuration, duration time.Duration) {
	previous := previousDuration.Seconds()
	if previous == 0 {
		return
	}

	durationsFactor := duration.Seconds() / previousDuration.Seconds()
	t.Logf("previous=%-20s current=%-20s factor=%5.3f", previousDuration, duration, durationsFactor)

	factorsComparison := testCase.minDelayFactor < durationsFactor && durationsFactor < testCase.maxDelayFactor

	message := "Previous and current duration factor should be between %.3f and %.3f, got %.3f"
	assert.True(t, factorsComparison, message, testCase.minDelayFactor, testCase.maxDelayFactor, durationsFactor)
}

func doTestCall(t *testing.T, c *client, testCase backoffTestCase, previousDuration time.Duration) (duration time.Duration) {
	var body io.Reader
	headers := make(http.Header)
	headers.Add("responseStatus", strconv.Itoa(testCase.responseStatus))

	started := time.Now()
	res, err := c.do("/", "POST", body, "application/json", headers)
	duration = time.Since(started)

	assert.NoError(t, err)
	assert.Equal(t, testCase.responseStatus, res.StatusCode)
	compareDurations(t, testCase, previousDuration, duration)

	return
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

	backOffDelayMin = 1 * time.Millisecond
	backOffDelayJitter = false
	switchFactor := float64((backOffDelayMin / (1 * time.Microsecond)))

	delayFactorMin := backOffDelayFactor - 0.5
	delayFactorMax := backOffDelayFactor + 0.5

	testCases := []backoffTestCase{
		{http.StatusCreated, 0, switchFactor},
		{http.StatusInternalServerError, 0, switchFactor},
		{http.StatusBadGateway, delayFactorMin, delayFactorMax},
		{http.StatusServiceUnavailable, delayFactorMin, delayFactorMax},
		{http.StatusOK, 0, switchFactor},
		{http.StatusConflict, 0, switchFactor},
		{http.StatusTooManyRequests, delayFactorMin, delayFactorMax},
		{http.StatusCreated, 0, switchFactor},
		{http.StatusInternalServerError, 0, switchFactor},
		{http.StatusTooManyRequests, delayFactorMin, delayFactorMax},
		{599, delayFactorMin, delayFactorMax},
		{499, delayFactorMin, delayFactorMax},
	}

	var duration time.Duration
	for id, testCase := range testCases {
		t.Run(fmt.Sprintf("%d-%d", id, testCase.responseStatus), func(t *testing.T) {
			duration = doTestCall(t, c, testCase, duration)
		})
	}
}

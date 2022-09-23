//go:build !integration

package ca_chain

import (
	"bytes"
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type fetcherMockFactory func(t *testing.T) fetcher

//nolint:unparam
func newFetcherMock(expectedURL string, data []byte, err error) fetcherMockFactory {
	return func(t *testing.T) fetcher {
		m := mockFetcher{}
		m.On("Fetch", expectedURL).Return(data, err)

		return &m
	}
}

type decoderMockFactory func(t *testing.T) decoder

func newDecoderMock(inputData []byte, cert *x509.Certificate, err error) decoderMockFactory {
	return func(t *testing.T) decoder {
		return func(data []byte) (*x509.Certificate, error) {
			assert.Equal(t, inputData, data)

			return cert, err
		}
	}
}

func TestUrlResolver_Resolve(t *testing.T) {
	testError := errors.New("test-error")
	url1 := "url1"

	testCACertificate := loadCertificate(t, testCACert)
	testCertificate := loadCertificate(t, testCert)
	testCertificateWithURL := loadCertificate(t, testCert)
	testCertificateWithURL.IssuingCertificateURL = []string{url1, "url2"}

	tests := map[string]struct {
		certs          []*x509.Certificate
		mockLoopLimit  int
		mockFetcher    fetcherMockFactory
		mockDecoder    decoderMockFactory
		expectedError  string
		expectedCerts  []*x509.Certificate
		expectedOutput []string
	}{
		"empty input chain": {
			certs:          nil,
			mockLoopLimit:  defaultURLResolverLoopLimit,
			expectedError:  "",
			expectedCerts:  nil,
			expectedOutput: nil,
		},
		"last certificate without URL": {
			certs:         []*x509.Certificate{testCertificate},
			mockLoopLimit: defaultURLResolverLoopLimit,
			expectedError: "",
			expectedCerts: []*x509.Certificate{testCertificate},
			expectedOutput: []string{
				"Certificate doesn't provide parent URL: exiting the loop",
			},
		},
		"last certificate with URL and fetcher error": {
			certs:         []*x509.Certificate{testCertificateWithURL},
			mockLoopLimit: defaultURLResolverLoopLimit,
			mockFetcher:   newFetcherMock(url1, nil, testError),
			expectedError: "error while fetching issuer certificate: remote fetch failure: test-error",
			expectedCerts: nil,
			expectedOutput: []string{
				"Remote certificate fetching error",
			},
		},
		"last certificate with URL and decoder error": {
			certs:         []*x509.Certificate{testCertificateWithURL},
			mockLoopLimit: defaultURLResolverLoopLimit,
			mockFetcher:   newFetcherMock(url1, []byte("test"), nil),
			mockDecoder:   newDecoderMock([]byte("test"), nil, testError),
			expectedError: "error while fetching issuer certificate: decoding failure: test-error",
			expectedCerts: nil,
			expectedOutput: []string{
				"Certificate decoding error",
			},
		},
		"last certificate with URL with not self signed": {
			certs:         []*x509.Certificate{testCertificateWithURL},
			mockLoopLimit: defaultURLResolverLoopLimit,
			mockFetcher:   newFetcherMock(url1, []byte("test"), nil),
			mockDecoder:   newDecoderMock([]byte("test"), testCertificate, nil),
			expectedError: "",
			expectedCerts: []*x509.Certificate{testCertificateWithURL, testCertificate},
			expectedOutput: []string{
				"Appending the certificate to the chain",
			},
		},
		"last certificate with URL with self signed": {
			certs:         []*x509.Certificate{testCertificateWithURL},
			mockLoopLimit: defaultURLResolverLoopLimit,
			mockFetcher:   newFetcherMock(url1, []byte("test"), nil),
			mockDecoder:   newDecoderMock([]byte("test"), testCACertificate, nil),
			expectedError: "",
			expectedCerts: []*x509.Certificate{testCertificateWithURL, testCACertificate},
			expectedOutput: []string{
				"Fetched issuer certificate is a ROOT certificate so exiting the loop",
			},
		},
		"last certificate with URL but no issue certificate": {
			certs:         []*x509.Certificate{testCertificateWithURL},
			mockLoopLimit: defaultURLResolverLoopLimit,
			mockFetcher:   newFetcherMock(url1, []byte("test"), nil),
			mockDecoder:   newDecoderMock([]byte("test"), nil, nil),
			expectedError: "",
			expectedCerts: []*x509.Certificate{testCertificateWithURL},
			expectedOutput: []string{
				"Fetched issuer certificate file does not contain any certificates: exiting the loop",
			},
		},
		"infinite loop": {
			certs:         []*x509.Certificate{testCertificateWithURL},
			mockLoopLimit: 3,
			mockFetcher:   newFetcherMock(url1, []byte("test"), nil),
			mockDecoder:   newDecoderMock([]byte("test"), testCertificateWithURL, nil),
			expectedError: "",
			expectedCerts: []*x509.Certificate{testCertificateWithURL, testCertificateWithURL, testCertificateWithURL},
			expectedOutput: []string{
				"urlResolver loop limit exceeded; exiting the loop",
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			out := new(bytes.Buffer)

			logger := logrus.New()
			logger.SetLevel(logrus.DebugLevel)
			logger.SetOutput(out)

			r := newURLResolver(logger).(*urlResolver)
			r.loopLimit = tc.mockLoopLimit

			if tc.mockFetcher != nil {
				r.fetcher = tc.mockFetcher(t)
			}

			if tc.mockDecoder != nil {
				r.decoder = tc.mockDecoder(t)
			}

			newCerts, err := r.Resolve(tc.certs)

			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedCerts, newCerts)

			output := out.String()
			if len(tc.expectedOutput) > 0 {
				for _, expectedLine := range tc.expectedOutput {
					assert.Contains(t, output, expectedLine)
				}
			} else {
				assert.Empty(t, output)
			}
		})
	}
}

func TestHTTPFetcher(t *testing.T) {
	assertURLError := func(t *testing.T, err error) {
		var e *url.Error
		if assert.ErrorAs(t, err, &e) {
			assert.Equal(t, "Get", e.Op)
			assert.Contains(t, e.URL, "http://127.0.0.1:")
		}
	}
	assertTimeoutError := func(t *testing.T, err error) {
		assertURLError(t, err)

		var e *url.Error
		if assert.ErrorAs(t, err, &e) {
			assert.True(t, e.Timeout(), "is timeout error")
		}
	}

	tests := map[string]struct {
		mockServer   func() *httptest.Server
		mockFetcher  *httpFetcher
		expectedData []byte
		assertError  func(t *testing.T, err error)
	}{
		"fetch ok": {
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = w.Write([]byte("data"))
				}))
			},
			mockFetcher:  newHTTPFetcher(defaultURLResolverFetchTimeout),
			expectedData: []byte("data"),
			assertError:  nil,
		},
		"fetch timeout": {
			mockServer: func() *httptest.Server {
				return httptest.NewUnstartedServer(nil)
			},
			mockFetcher:  newHTTPFetcher(50 * time.Millisecond),
			expectedData: nil,
			assertError:  assertTimeoutError,
		},
		"fetch no remote": {
			mockServer: func() *httptest.Server {
				srv := httptest.NewUnstartedServer(nil)
				_ = srv.Listener.Close()
				return srv
			},
			mockFetcher:  newHTTPFetcher(50 * time.Millisecond),
			expectedData: nil,
			assertError:  assertURLError,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			resp, err := tc.mockFetcher.Fetch("http://" + tc.mockServer().Listener.Addr().String())
			if tc.assertError != nil {
				tc.assertError(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedData, resp)
		})
	}
}

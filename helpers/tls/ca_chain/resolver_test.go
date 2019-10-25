package ca_chain

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCACertURI  = "/ca-cert"
	invalidCertURI = "/invalid-cert"
)

func TestChainResolver_Resolve(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.RequestURI == testCACertURI {
			_, err := rw.Write([]byte(testCACert))
			require.NoError(t, err)

			return
		}

		if r.RequestURI == invalidCertURI {
			_, err := rw.Write([]byte("-----BEGIN CERTIFICATE-----"))
			require.NoError(t, err)

			return
		}

		rw.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	caCertPool := func() *x509.CertPool {
		block, _ := pem.Decode([]byte(testCACert))
		testCACertificate, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)

		p := x509.NewCertPool()
		p.AddCert(testCACertificate)

		return p
	}

	tests := map[string]struct {
		issuerURL           []string
		certPool            func() *x509.CertPool
		expectedError       string
		expectedChainLength int
		expectedOutput      []string
	}{
		"no issuer certificate URL": {
			expectedError:       "",
			expectedChainLength: 1,
			expectedOutput: []string{
				"Certificate doesn't provide parent URL",
				"Verifying last certificate to find the final root certificate",
				"Last certificate signed by unknown authority; will not update the chain",
			},
		},
		"no issuer certificate URL but CA present in system cert pool": {
			certPool:            caCertPool,
			expectedError:       "",
			expectedChainLength: 2,
			expectedOutput: []string{
				"Certificate doesn't provide parent URL",
				"Verifying last certificate to find the final root certificate",
				"Adding cert from verify chain to the final chain",
			},
		},
		"invalid certificate as parent": {
			issuerURL:           []string{server.URL + invalidCertURI},
			expectedError:       "error while resolving certificates chain: error while fetching issuer certificate: invalid certificate: empty PEM block",
			expectedChainLength: 0,
			expectedOutput: []string{
				"Requesting issuer certificate",
				"Requesting issuer certificate: certificate decoding error",
			},
		},
		"issuer certificate as parent": {
			issuerURL:           []string{server.URL + testCACertURI},
			expectedError:       "",
			expectedChainLength: 2,
			expectedOutput: []string{
				"Requesting issuer certificate",
				"Requesting issuer certificate: appending the certificate to the chain",
				"Fetched issuer certificate is a ROOT certificate so exiting the loop",
			},
		},
		"issuer certificate as parent and within system cert pool": {
			issuerURL:           []string{server.URL + testCACertURI},
			certPool:            caCertPool,
			expectedError:       "",
			expectedChainLength: 2,
			expectedOutput: []string{
				"Requesting issuer certificate",
				"Requesting issuer certificate: appending the certificate to the chain",
				"Fetched issuer certificate is a ROOT certificate so exiting the loop",
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			block, _ := pem.Decode([]byte(testCert))
			testCertificate, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err)

			testCertificate.IssuingCertificateURL = tc.issuerURL

			out := new(bytes.Buffer)

			logger := logrus.New()
			logger.Level = logrus.DebugLevel
			logger.Out = out

			r := newResolver(logger).(*chainResolver)

			if tc.certPool != nil {
				r.verifyOptions = x509.VerifyOptions{
					Roots: tc.certPool(),
				}
			}

			certificates, err := r.Resolve([]*x509.Certificate{testCertificate})

			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, certificates, tc.expectedChainLength)

			output := out.String()
			for _, outputPart := range tc.expectedOutput {
				if outputPart == "" {
					continue
				}

				assert.Contains(t, output, outputPart)
			}
		})
	}
}
